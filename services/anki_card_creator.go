package services

import (
	"context"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/card"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractors"
	"golang.org/x/sync/errgroup"
)

const default_batch_size = 5
const template_name = "Maple Template X"

type AnkiCardCreator struct {
	queueSize       int
	logger          *slog.Logger
	ankiService     AnkiService
	cardCreator     card.FlashcardCreator
	targetExtractor extractors.TargetExtractor
}

func NewAnkiCardCreator(cfg config.Config, logger *slog.Logger, ankiService AnkiService, cardCreator card.FlashcardCreator, targetExtractor extractors.TargetExtractor) *AnkiCardCreator {
	queueSize := default_batch_size

	if cfg.QueueSize != 0 {
		queueSize = cfg.QueueSize
	}

	return &AnkiCardCreator{
		queueSize:       queueSize,
		logger:          logger.WithGroup("anki-card-creator"),
		ankiService:     ankiService,
		cardCreator:     cardCreator,
		targetExtractor: targetExtractor,
	}
}

// TODO maybe need split errors to two category:
// - critical - when this error returns, workflow will stop
// - noncritical - when this error returns, workflow continue, add not stored target for some pull and return aka "not generated"
func (c *AnkiCardCreator) Create(ctx context.Context, pathToTargets string) error {
	c.logger.Info("start creating flashcard and store it in anki")
	targets, err := c.targetExtractor.GetTargetsFromFile(ctx, pathToTargets)
	if err != nil {
		return err
	}

	errg := errgroup.Group{}
	chanQueue := make(chan bool, c.queueSize)

	for _, target := range *targets {
		for _, subject := range target.Subjects {
			errg.Go(func() error {
				// Realese the queue
				defer func() { <-chanQueue }()
				chanQueue <- true
				c.logger.Info("start creating flashcard", slog.Any("subject", subject))
				isExist, err := c.ankiService.IsCardAlreadyExist(ctx, subject, target.DeckName)
				if err != nil {
					return err
				}
				if isExist {
					c.logger.Debug("flashcard already exist, skip it", slog.Any("subject", subject), slog.Any("deckname", target.DeckName))
					return nil
				}

				flashcard, err := c.cardCreator.Create(ctx, subject, target.DeckName)
				if err != nil {
					return err
				}

				err = c.ankiService.StoreNewCard(ctx, template_name, flashcard)
				if err != nil {
					return err
				}
				c.logger.Info("creating flashcard is done", slog.Any("subject", subject))
				return nil
			})
		}
	}

	err = errg.Wait()
	if err != nil {
		c.logger.Error("failed create one of card, stop creatings cards")
		return err
	}
	c.logger.Info("creating flashcards is done")

	return nil
}
