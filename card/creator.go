package card

//go:generate mockgen -source creator.go -destination mock/creator_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/shredd0r/anki-card-creator/card/fetchers"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
	"github.com/shredd0r/anki-card-creator/utils"
)

type FlashcardCreator interface {
	Create(ctx context.Context, subject string, deck string) (*models.Flashcard, error)
}

type implFlashcardCreator struct {
	pictureCfg                   config.PictureConfig
	logger                       *slog.Logger
	geminiProvider               providers.GeminiProvider
	googleImageProvider          providers.GoogleImageProvider
	fieldComponentFetcherFactory fetchers.FieldComponentFetcherFactory
}

func NewFlashcardCreator(pictureCfg config.PictureConfig, logger *slog.Logger,
	geminiProvider providers.GeminiProvider,
	googleImageProvider providers.GoogleImageProvider,
	fieldComponentFetcherFactory fetchers.FieldComponentFetcherFactory) FlashcardCreator {
	return &implFlashcardCreator{
		pictureCfg:                   pictureCfg,
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

type fetchTask struct {
	fieldName     string
	callMethod    func(ctx context.Context, subject string) error
	ignoreTaskErr bool
}

func (c *implFlashcardCreator) create(ctx context.Context, fieldComponentFetcher fetchers.FieldComponentFetcher, subject string, deck string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	ctxForCreate, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	subjectType := fieldComponentFetcher.GetSubjectType()
	var explain *string
	var examples *[]string
	var transcription *string
	var pronunciation *models.File
	var picture *models.File
	chanForErr := make(chan error, 1)

	fetchTasks := []fetchTask{
		{
			fieldName: "explain",
			callMethod: func(ctx context.Context, subject string) error {
				respExplain, err := fieldComponentFetcher.GetExplain(ctx, subject)
				explain = respExplain
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "examples",
			callMethod: func(ctx context.Context, subject string) error {
				respExamples, err := fieldComponentFetcher.GetExamples(ctx, subject)
				examples = respExamples
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "transcription",
			callMethod: func(ctx context.Context, subject string) error {
				respTranscription, err := fieldComponentFetcher.GetTranscription(ctx, subject)
				transcription = respTranscription
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "pronunciation",
			callMethod: func(ctx context.Context, subject string) error {
				respPronunciation, err := fieldComponentFetcher.GetPronunciation(ctx, subject)
				pronunciation = respPronunciation
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "picture",
			callMethod: func(ctx context.Context, subject string) error {
				respPicture, err := c.getPicture(ctx, subject, subjectType)
				picture = respPicture
				return err
			},
			ignoreTaskErr: c.pictureCfg.IgnorePictureError,
		},
	}

	for _, fetchTask := range fetchTasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fetchTask.callMethod(ctxForCreate, subject)
			if err != nil {
				c.logger.Debug(fmt.Sprintf("received error after get %s", fetchTask.fieldName))
				// First check ignoring error
				if fetchTask.ignoreTaskErr {
					c.logger.Debug("received error need ignore, skip it")
				} else {
					// After that, if in channel for error empty, put error to channel, because needed only first error
					if len(chanForErr) == 0 {
						c.logger.Debug("cancel context, put err to chan", slog.Any("err", err.Error()))
						cancel()
						chanForErr <- err
					}
				}
			}
		}()
	}

	c.logger.Debug("start waiting for complete all field component fetcher goroutines")
	wg.Wait()
	cancel()
	if len(chanForErr) != 0 {
		return nil, <-chanForErr
	}

	return &models.Flashcard{
		Subject:       subject,
		SubjectType:   subjectType,
		DeckName:      deck,
		Transcription: transcription,
		Pronunciation: pronunciation,
		Picture:       picture,
		Explain:       *explain,
		Examples:      *examples,
	}, nil
}

// Method for rating pictures gotten from google image
// If all attempts images don't match, creating card continue without picture
func (c *implFlashcardCreator) getPicture(ctx context.Context, subject string, subjectType models.SubjectType) (*models.File, error) {
	if subjectType == models.SubjectTypePhrase {
		c.logger.Debug("picture for phrase is not searching, skip", slog.Any("subject", subject))
		return nil, nil
	}

	queryPageProvider, err := c.googleImageProvider.NewQuery(ctx, subject)
	if err != nil {
		return nil, err
	}

	for attempt := range c.pictureCfg.NumberOfAttemptRatingPicture {
		c.logger.Debug(fmt.Sprintf("attempt: %d for getting picture for subject: %s", attempt, subject))

		picture, err := queryPageProvider.Get(ctx, attempt)
		if err != nil {
			c.logger.Error("failed get picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		// skip picture with mimetype svg, because this type not supported gemini server
		if picture.MIMEType == "image/svg+xml" {
			c.logger.Debug("skip image with type svg")
			continue
		}

		rating, err := c.geminiProvider.RatingPicture(ctx, subject, picture)
		if err != nil {
			c.logger.Error("failed rating picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		if *rating >= c.pictureCfg.MinimalRating {
			c.logger.Debug(fmt.Sprintf("found suitable picture for subject: %s", subject))
			return picture, nil
		}
	}

	c.logger.Warn(fmt.Sprintf("picture for subject: %s not found, continue without picture", subject))
	return nil, nil
}
