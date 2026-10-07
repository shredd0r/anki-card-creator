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
	c.logger.Info("creating flashcard and store it in anki")

	// chanErr is sized so that every goroutine that might send to it (one per
	// subject, plus the single AddFlashcard consumer) can always do so
	// without blocking, even after Create has already returned via the first
	// error - otherwise a second erroring goroutine would leak forever
	// waiting on an unbuffered send nobody reads anymore.
	chanErr := make(chan error, c.getCountOfSubject(targets)+1)
	// chanCompleteAddFlashcards is buffered for the same reason: its single
	// sender must never block, even in the rare case it fires at the same
	// moment Create is already returning via chanErr.
	chanCompleteAddFlashcards := make(chan struct{}, 1)
	chanFlashcard := make(chan *models.Flashcard, c.queueSize)
	ctxWithCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	go c.createAllFlashcards(ctxWithCancel, cancel, targets, chanFlashcard, chanErr)
	go c.addFlashcardToPackage(ctxWithCancel, cancel, chanFlashcard, chanErr, chanCompleteAddFlashcards)

	select {
	case <-chanCompleteAddFlashcards:
		c.logger.Info("creating flashcards is done")
		return nil
	case err := <-chanErr:
		c.logger.Error("received error during create flashcard", slog.Any("err", err))
		return err
	}
}

func (c *AnkiCardCreator) createAllFlashcards(ctxWithCancel context.Context, cancel context.CancelFunc, targets *[]extractor.TargetInfo, chanFlashcard chan *models.Flashcard, chanErr chan error) {
	chanQueue := make(chan struct{}, c.queueSize)
	wg := sync.WaitGroup{}

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
				c.logger.Info("creating flashcard", slog.String("subject", subject), slog.Int64("current", current), slog.Int("total", count))

				flashcard, err := c.cardCreator.Create(ctxWithCancel, target.DeckName, subject, target.Tags)
				if err != nil {
					c.logger.Error("failed to create flashcard", slog.String("subject", subject), slog.Any("err", err))
					cancel()
					chanErr <- err
					return
				}
				chanFlashcard <- flashcard

				c.logger.Info("creating flashcard is done", slog.String("subject", subject))
			}()
		}
	}
	wg.Wait()
	// All producer goroutines are done (whether they succeeded or errored),
	// so no more sends to chanFlashcard can happen. Closing it here (rather
	// than relying on context cancellation) is what lets the consumer detect
	// "all flashcards produced" as a distinct event from "something failed".
	close(chanFlashcard)
}

func (c *AnkiCardCreator) addFlashcardToPackage(ctxWithCancel context.Context, cancel context.CancelFunc, chanFlashcard chan *models.Flashcard, chanErr chan error, chanComplete chan struct{}) {
	for {
		select {
		case <-ctxWithCancel.Done():
			{
				c.logger.Debug("context is done, stop goroutine 'addFlashcardToPackage'")
				return
			}
		case flashcard, ok := <-chanFlashcard:
			{
				if !ok {
					c.logger.Debug("channel for flashcards closed, all flashcards processed")
					chanComplete <- struct{}{}
					return
				}
				if flashcard == nil {
					continue
				}
				if err := c.ankiService.AddFlashcard(ctxWithCancel, flashcard); err != nil {
					c.logger.Error("failed to add flashcard via anki connect", slog.Any("err", err))
					cancel()
					chanErr <- err
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
