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
	fieldName                 string
	gettingVolumeAndPutToChan func(ctx context.Context, subject string) error
	ignoreTaskErr             bool
}

func (c *implFlashcardCreator) create(ctx context.Context, fieldComponentFetcher fetchers.FieldComponentFetcher, subject string, deck string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	ctxForCreate, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	subjectType := fieldComponentFetcher.GetSubjectType()
	chanForExplain := make(chan *string, 1)
	chanForExamples := make(chan *[]string, 1)
	chanForTranscription := make(chan *string, 1)
	chanForPronunciation := make(chan *models.File, 1)
	chanForPicture := make(chan *models.File, 1)
	chanForErr := make(chan error, 1)

	fetchTasks := []fetchTask{
		{
			fieldName: "explain",
			gettingVolumeAndPutToChan: func(ctx context.Context, subject string) error {
				explain, err := fieldComponentFetcher.GetExplain(ctx, subject)
				chanForExplain <- explain
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "examples",
			gettingVolumeAndPutToChan: func(ctx context.Context, subject string) error {
				examples, err := fieldComponentFetcher.GetExamples(ctx, subject)
				chanForExamples <- examples
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "transcription",
			gettingVolumeAndPutToChan: func(ctx context.Context, subject string) error {
				transcription, err := fieldComponentFetcher.GetTranscription(ctx, subject)
				chanForTranscription <- transcription
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "pronunciation",
			gettingVolumeAndPutToChan: func(ctx context.Context, subject string) error {
				pronunciation, err := fieldComponentFetcher.GetPronunciation(ctx, subject)
				chanForPronunciation <- pronunciation
				return err
			},
			ignoreTaskErr: false,
		},
		{
			fieldName: "picture",
			gettingVolumeAndPutToChan: func(ctx context.Context, subject string) error {
				picture, err := c.getPicture(ctx, subject, subjectType)
				chanForPicture <- picture
				return err
			},
			ignoreTaskErr: c.pictureCfg.IgnorePictureError,
		},
	}

	for _, fetchTask := range fetchTasks {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				c.logger.Debug("call defer method")
			}()
			err := fetchTask.gettingVolumeAndPutToChan(ctxForCreate, subject)
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
