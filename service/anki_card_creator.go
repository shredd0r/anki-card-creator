package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"

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
	go c.addFlashcardToPackage(ctxWithCancel, cancel, chanFlashcard, chanErr, chanCompleteAddFlashcards)

	for {
		select {
		case <-ctxWithCancel.Done():
		case <-chanCompleteAddFlashcards:
			{
				c.logger.Info("creating flashcards is done")
				return nil
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

	var indexSubject atomic.Int64
	count := c.getCountOfSubject(targets)
	c.logger.Info("detected subjects", slog.Int("count", count))

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

				current := indexSubject.Add(1)
				c.logger.Info("start creating flashcard", slog.String("subject", subject), slog.Int64("current", current), slog.Int("total", count))

				flashcard, err := c.cardCreator.Create(ctxWithCancel, target.DeckName, subject, target.Tags)
				if err != nil {
					c.logger.Error("failed to create flashcard", slog.String("subject", subject), slog.Any("err", err))
					chanErr <- err
				}
				chanFlashcard <- flashcard

				c.logger.Info("creating flashcard is done", slog.String("subject", subject))
			}()
		}
	}
	wg.Wait()
}

func (c *AnkiCardCreator) addFlashcardToPackage(ctxWithCancel context.Context, cancel context.CancelFunc, chanFlashcard chan *models.Flashcard, chanErr chan error, chanComplete chan struct{}) {
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
					if err := c.ankiService.AddFlashcard(ctxWithCancel, flashcard); err != nil {
						c.logger.Error("failed to add flashcard via anki connect", slog.Any("err", err))
						cancel()
						chanErr <- err
						return
					}
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
