package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractors"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
)

type FlashcardCreator interface {
	Create(ctx context.Context, subject string, deck string) (*models.Flashcard, error)
}

type implFlashcardCreator struct {
	creatorCfg          config.CreatorConfig
	logger              *slog.Logger
	cambridgeExtractor  extractors.CambridgeCardExtractor
	googleImageProvider providers.GoogleImageProvider
	geminiProvider      providers.GeminiProvider
}

func NewFlashcardCreator(creatorCfg config.CreatorConfig, logger *slog.Logger,
	cambridgeExtractor extractors.CambridgeCardExtractor,
	googleImageProvider providers.GoogleImageProvider,
	geminiProvider providers.GeminiProvider) FlashcardCreator {
	return &implFlashcardCreator{
		creatorCfg:          creatorCfg,
		logger:              logger.WithGroup("flashcard-creator"),
		cambridgeExtractor:  cambridgeExtractor,
		googleImageProvider: googleImageProvider,
		geminiProvider:      geminiProvider,
	}
}

func (c *implFlashcardCreator) Create(ctx context.Context, subject string, deck string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	cambridgeCard, err := c.cambridgeExtractor.GetCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	var pictureForCard *models.File
	for attempt := range c.creatorCfg.NumberOfAttemptRatingPicture {
		c.logger.Debug(fmt.Sprintf("attempt: %d for getting picture for subject: %s", attempt, subject))
		picture, err := c.googleImageProvider.Get(ctx, attempt, subject)
		if err != nil {
			c.logger.Error("failed get picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		rating, err := c.geminiProvider.RatingPicture(ctx, subject, picture)
		if err != nil {
			c.logger.Error("failed rating picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		if *rating >= c.creatorCfg.MinimalRating {
			c.logger.Debug(fmt.Sprintf("found suitable picture for subject: %s", subject))
			pictureForCard = picture
			break
		}
	}

	c.logger.Warn("not found picture for subject, flashcard will be without picture", slog.Any("subject", subject))

	flashcard := &models.Flashcard{
		Subject:       cambridgeCard.Subject,
		SubjectType:   cambridgeCard.SubjectType,
		DeckName:      deck,
		Pronouns:      cambridgeCard.Pronouns,
		Transcription: cambridgeCard.Transcription,
		Explain:       cambridgeCard.Explains[0],
		Picture:       pictureForCard,
		Examples:      cambridgeCard.Examples,
	}

	return flashcard, nil
}
