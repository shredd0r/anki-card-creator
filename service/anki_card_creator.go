package service

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"sync"
	"time"

	"github.com/shredd0r/anki-card-creator/anki"
	"github.com/shredd0r/anki-card-creator/card"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	default_queue_size = uint(10)
)

type AnkiCardCreator struct {
	queueSize   uint
	outputPath  string
	logger      *slog.Logger
	ankiService anki.Service
	cardCreator card.Creator
}

func NewAnkiCardCreator(cfg config.Config, logger *slog.Logger, ankiService anki.Service, cardCreator card.Creator) *AnkiCardCreator {
	queueSize := default_queue_size

	if cfg.QueueSize != 0 {
		queueSize = cfg.QueueSize
	}

	return &AnkiCardCreator{
		queueSize:   queueSize,
		outputPath:  cfg.Output,
		logger:      logger.WithGroup("anki-card-creator"),
		ankiService: ankiService,
		cardCreator: cardCreator,
	}
}

func (c *AnkiCardCreator) Create(ctx context.Context, targets *[]extractor.TargetInfo) error {
	c.logger.Info("start creating flashcard and store it in anki")

	chanErr := make(chan error)
	chanCompleteAddFlashcards := make(chan struct{})
	chanFlashcard := make(chan *models.Flashcard, c.queueSize)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.createAllFlashcards(ctxWithCancel, cancel, targets, chanFlashcard, chanErr)
	go c.addFlashcardToPackage(ctxWithCancel, chanFlashcard, chanCompleteAddFlashcards)

	for {
		select {
		case <-ctxWithCancel.Done():
		case <-chanCompleteAddFlashcards:
			{
				c.logger.Info("creating flashcards is done, start formating package")
				return c.ankiService.SavePackage(ctx, path.Join(c.outputPath, c.generateAPKGFileName()))
			}
		case err := <-chanErr:
			{
				c.logger.Error("received error during create flashcard", slog.Any("err", err))
				return err
			}
		}
	}
}

func (c *AnkiCardCreator) createAllFlashcards(ctxWithCancel context.Context, cancel context.CancelFunc, targets *[]extractor.TargetInfo, chanFlashcard chan *models.Flashcard, chanErr chan error) {
	chanQueue := make(chan struct{}, c.queueSize)
	wg := sync.WaitGroup{}
	defer cancel()

	indexSubject := 0
	count := c.getCountOfSubject(targets)
	c.logger.Info(fmt.Sprintf("Detected %d subjects", count))

	for _, target := range *targets {
		for _, subject := range target.Subjects {
			wg.Add(1)
			go func() {
				// Realese the queue and cancel child ctx
				defer func() {
					wg.Done()
					<-chanQueue
				}()
				chanQueue <- struct{}{}

				indexSubject++
				c.logger.Info(fmt.Sprintf("start creating flashcard %s, current subject : %d, count of subjects: %d", subject, indexSubject, count))

				flashcard, err := c.cardCreator.Create(ctxWithCancel, target.DeckName, subject, target.Tags)
				if err != nil {
					chanErr <- err
				}
				chanFlashcard <- flashcard

				c.logger.Info("creating flashcard is done", slog.Any("subject", subject))
			}()
		}
	}
	wg.Wait()
}

func (c *AnkiCardCreator) addFlashcardToPackage(ctxWithCancel context.Context, chanFlashcard chan *models.Flashcard, chanComplete chan struct{}) {
	for {
		select {
		case <-ctxWithCancel.Done():
			{
				c.logger.Debug("context is done, stop goroutine 'addFlashcardToPackage'")
				close(chanFlashcard)
				chanComplete <- struct{}{}
				return
			}
		case flashcard, ok := <-chanFlashcard:
			{
				if flashcard != nil {
					c.ankiService.AddFlashcard(ctxWithCancel, flashcard)
				}
				if !ok {
					c.logger.Debug("channel for flashcards already closed, stop goroutine 'addFlashcardToPackage'")
					return
				}
			}
		}
	}
}

func (c *AnkiCardCreator) getCountOfSubject(targets *[]extractor.TargetInfo) int {
	count := 0
	for _, target := range *targets {
		count += len(target.Subjects)
	}
	return count
}

func (c *AnkiCardCreator) generateAPKGFileName() string {
	return fmt.Sprintf("generated-flashcards-%d.apkg", time.Now().UnixMilli())
}
