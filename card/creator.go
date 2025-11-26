package card

//go:generate mockgen -source creator.go -destination mock/creator_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/shredd0r/anki-card-creator/card/fetcher"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/google"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/utils"
)

type Creator interface {
	Create(ctx context.Context, deck string, subject string, usingContext *[]string) (*models.Flashcard, error)
}

type implCreator struct {
	pictureCfg          config.PictureConfig
	logger              *slog.Logger
	llmProvider         llm.Provider
	googleImageProvider google.Image
	cardContentFactory  fetcher.CardContentFactory
}

func NewFlashcardCreator(pictureCfg config.PictureConfig, logger *slog.Logger,
	llmProvider llm.Provider,
	googleImageProvider google.Image,
	cardContentFactory fetcher.CardContentFactory) Creator {
	return &implCreator{
		pictureCfg:          pictureCfg,
		logger:              logger.WithGroup("flashcard-creator"),
		googleImageProvider: googleImageProvider,
		llmProvider:         llmProvider,
		cardContentFactory:  cardContentFactory,
	}
}

func (c *implCreator) Create(ctx context.Context, deck string, subject string, usingContext *[]string) (*models.Flashcard, error) {
	subjectType := utils.GetSubjectType(subject)
	cardContentFetcher, err := c.cardContentFactory.Get(subjectType)
	if err != nil {
		return nil, err
	}

	return c.create(ctx, cardContentFetcher, deck, subject, usingContext)
}

type fetchTask struct {
	fieldName     string
	callMethod    func(ctx context.Context, subject string, usingContext *[]string) error
	ignoreTaskErr bool
}

func (c *implCreator) create(ctx context.Context, cardContentFetcher fetcher.CardContent, deck string, subject string, usingContext *[]string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	ctxForCreate, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	subjectType := cardContentFetcher.GetSubjectType()
	var cardContent *models.CardContent
	var picture *models.File
	chanForErr := make(chan error, 1)

	fetchTasks := []fetchTask{
		{
			fieldName: "card-content",
			callMethod: func(ctx context.Context, subject string, usingContext *[]string) error {
				respCardContent, err := cardContentFetcher.GetCardContent(ctx, subject, usingContext)
				cardContent = respCardContent
				return err
			},
		},
		{
			fieldName: "picture",
			callMethod: func(ctx context.Context, subject string, usingContext *[]string) error {
				respPicture, err := c.getPicture(ctx, subject, usingContext, subjectType)
				picture = respPicture
				return err
			},
			ignoreTaskErr: c.pictureCfg.IgnoreError,
		},
	}

	for _, fetchTask := range fetchTasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fetchTask.callMethod(ctxForCreate, subject, usingContext)
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

	c.logger.Debug("start waiting for complete all card content fetcher goroutines")
	wg.Wait()
	cancel()
	if len(chanForErr) != 0 {
		return nil, <-chanForErr
	}

	return &models.Flashcard{
		Subject:       subject,
		SubjectType:   subjectType,
		DeckName:      deck,
		Transcription: cardContent.Transcription,
		Pronunciation: cardContent.Pronunciation,
		Picture:       picture,
		Paraphrase:    cardContent.Paraphrase,
		Examples:      cardContent.Examples,
	}, nil
}

// Method for rating pictures gotten from google image
// If all attempts images don't match, creating card continue without picture
func (c *implCreator) getPicture(ctx context.Context, subject string, usingContext *[]string, subjectType models.SubjectType) (*models.File, error) {
	if subjectType == models.SubjectTypePhrase {
		c.logger.Debug("picture for phrase is not searching, skip", slog.Any("subject", subject))
		return nil, nil
	}

	var query string
	if usingContext == nil {
		query = subject
	} else {
		query = fmt.Sprintf("%s %s", subject, strings.Join(*usingContext, ", "))
	}

	queryPageProvider, err := c.googleImageProvider.Request(ctx, query)
	if err != nil {
		return nil, err
	}

	for attempt := range c.pictureCfg.CountSearches {
		c.logger.Debug(fmt.Sprintf("attempt: %d for getting picture for subject: %s", attempt, subject))

		picture, err := queryPageProvider.Get(ctx, attempt)
		if err != nil {
			c.logger.Error("failed get picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		rating, err := c.llmProvider.RatePicture(ctx, subject, picture)
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
