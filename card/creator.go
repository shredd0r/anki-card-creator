package card

//go:generate mockgen -source creator.go -destination mock/creator_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/hegedustibor/htgo-tts/voices"
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
	speech              *htgotts.Speech
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
		speech:              &htgotts.Speech{Language: voices.EnglishUK},
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
	fieldName  string
	callMethod func(ctx context.Context, subject string, usingContext *[]string) error
}

func (c *implCreator) create(ctx context.Context, cardContentFetcher fetcher.CardContent, deck string, subject string, usingContext *[]string) (*models.Flashcard, error) {
	c.logger.Debug("start create flashcard", slog.Any("subject", subject))

	ctxForCreate, cancel := context.WithCancel(ctx)
	wg := &sync.WaitGroup{}

	subjectType := cardContentFetcher.GetSubjectType()
	var cardContent *models.CardContent
	var picture *models.File
	var pronunciation *models.File
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
				respPicture, err := cardContentFetcher.GetPicture(ctx, subject, usingContext)
				picture = respPicture
				return err
			},
		},
		{
			fieldName: "pronunciation",
			callMethod: func(ctx context.Context, subject string, usingContext *[]string) error {
				respPronunciation, err := cardContentFetcher.GetPronunciation(subject)
				pronunciation = respPronunciation
				return err
			},
		},
	}

	for _, fetchTask := range fetchTasks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := fetchTask.callMethod(ctxForCreate, subject, usingContext)
			if err != nil {
				c.logger.Debug(fmt.Sprintf("received error after get %s", fetchTask.fieldName))
				if len(chanForErr) == 0 {
					c.logger.Debug("cancel context, put err to chan", slog.Any("err", err.Error()))
					cancel()
					chanForErr <- err
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

	tags := []string{}
	if usingContext != nil {
		tags = append(tags, *usingContext...)
	}

	if cardContent.Pronunciation != nil {
		pronunciation = cardContent.Pronunciation
	}

	return &models.Flashcard{
		Subject:       subject,
		SubjectType:   subjectType,
		DeckName:      deck,
		Transcription: cardContent.Transcription,
		Pronunciation: pronunciation,
		Synonyms:      cardContent.Synonyms,
		Picture:       picture,
		Paraphrase:    cardContent.Paraphrase,
		Examples:      cardContent.Examples,
		Tags:          tags,
	}, nil
}
