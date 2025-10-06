package card

//go:generate mockgen -source creator.go -destination mock/creator_mock.go

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/card/fetchers"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
	"github.com/shredd0r/anki-card-creator/utils"
	"golang.org/x/sync/errgroup"
)

type FlashcardCreator interface {
	Create(ctx context.Context, subject string, deck string) (*models.Flashcard, error)
}

type implFlashcardCreator struct {
	ratingCfg                    config.RatingConfig
	logger                       *slog.Logger
	geminiProvider               providers.GeminiProvider
	googleImageProvider          providers.GoogleImageProvider
	fieldComponentFetcherFactory fetchers.FieldComponentFetcherFactory
}

func NewFlashcardCreator(ratingCfg config.RatingConfig, logger *slog.Logger,
	geminiProvider providers.GeminiProvider,
	googleImageProvider providers.GoogleImageProvider,
	fieldComponentFetcherFactory fetchers.FieldComponentFetcherFactory) FlashcardCreator {
	return &implFlashcardCreator{
		ratingCfg:                    ratingCfg,
		logger:                       logger.WithGroup("flashcard-creator"),
		googleImageProvider:          googleImageProvider,
		geminiProvider:               geminiProvider,
		fieldComponentFetcherFactory: fieldComponentFetcherFactory,
	}
}

func (c *implFlashcardCreator) Create(ctx context.Context, subject string, deck string) (*models.Flashcard, error) {
	subjectType := utils.GetSubjectType(subject)
	fieldComponentFetcher, err := c.fieldComponentFetcherFactory.Get(subjectType)
	if err != nil {
		return nil, err
	}

	return c.create(ctx, fieldComponentFetcher, subject, deck)
}

func (c *implFlashcardCreator) create(ctx context.Context, fieldComponentFetcher fetchers.FieldComponentFetcher, subject string, deck string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	errg := errgroup.Group{}

	subjectType := fieldComponentFetcher.GetSubjectType()
	chanForExplain := make(chan *string, 1)
	chanForExamples := make(chan *[]string, 1)
	chanForTranscription := make(chan *string, 1)
	chanForPronunciation := make(chan *models.File, 1)
	chanForPicture := make(chan *models.File, 1)

	errg.Go(func() error {
		explain, err := fieldComponentFetcher.GetExplain(ctx, subject)
		chanForExplain <- explain
		c.logger.Debug("done get explain, put it to channel")
		return err
	})

	errg.Go(func() error {
		examples, err := fieldComponentFetcher.GetExamples(ctx, subject)
		chanForExamples <- examples
		c.logger.Debug("done get examples, put it to channel")
		return err
	})

	errg.Go(func() error {
		transcription, err := fieldComponentFetcher.GetTranscription(ctx, subject)
		chanForTranscription <- transcription
		c.logger.Debug("done get transcription, put it to channel")
		return err
	})

	errg.Go(func() error {
		pronunciation, err := fieldComponentFetcher.GetPronunciation(ctx, subject)
		chanForPronunciation <- pronunciation
		c.logger.Debug("done get pronunciation, put it to channel")
		return err
	})

	errg.Go(func() error {
		picture, err := c.getPicture(ctx, subject, subjectType)
		chanForPicture <- picture
		c.logger.Debug("done get picture, put it to channel")
		return err
	})

	c.logger.Debug("start waiting for complete all field component fetcher goroutines")
	err := errg.Wait()
	if err != nil {
		return nil, err
	}

	return &models.Flashcard{
		Subject:       subject,
		SubjectType:   subjectType,
		DeckName:      deck,
		Transcription: <-chanForTranscription,
		Pronunciation: <-chanForPronunciation,
		Picture:       <-chanForPicture,
		Explain:       *<-chanForExplain,
		Examples:      *<-chanForExamples,
	}, nil
}

// Method for rating pictures gotten from google image
// If all attempts images don't match, creating card continue without picture
func (c *implFlashcardCreator) getPicture(ctx context.Context, subject string, subjectType models.SubjectType) (*models.File, error) {
	if subjectType == models.SubjectTypePhrase {
		c.logger.Debug("picture for phrase is not searching, skip", slog.Any("subject", subject))
		return nil, nil
	}

	for attempt := range c.ratingCfg.NumberOfAttemptRatingPicture {
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

		if *rating >= c.ratingCfg.MinimalRating {
			c.logger.Debug(fmt.Sprintf("found suitable picture for subject: %s", subject))
			return picture, nil
		}
	}

	c.logger.Warn(fmt.Sprintf("picture for subject: %s not found, continue without picture", subject))
	return nil, nil
}
