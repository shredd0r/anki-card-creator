package services

//go:generate mockgen -source creator.go -destination mock/creator_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

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

	var examples *[]string
	var pictureForCard *models.File

	// RatingPicture and generate examples (if it need) are using gemini api. This calls are very slow.
	// That's why i use goroutine for call this methods
	errChan := make(chan error, 2)
	pictureChan := make(chan *models.File)
	examplesChan := make(chan *[]string)
	wg := sync.WaitGroup{}
	wg.Add(2)
	// Goroutine for rating pictures from google image in order
	go func() {
		defer wg.Done()
		c.tryRatingPicture(ctx, subject, pictureChan, errChan)
	}()
	// Goroutine for check examples. If it nil, generate examples with gemini api
	go func() {
		defer wg.Done()
		c.checkAndGetExamples(ctx, subject, cambridgeCard, examplesChan, errChan)
	}()
	// Goroutine for close channels after complete goroutines which getting data for flashcard
	go func() {
		wg.Wait()
		close(errChan)
		close(pictureChan)
		close(examplesChan)
	}()

	// In infinite loop select checks data channels:
	// - pictureChan used for get picture which match for describe subject
	// - exampleChan used for get examples which show how to use subject in sentences
	// - errChan used for get errors from goroutines
	for {
		select {
		case <-ctx.Done():
			{
				c.logger.Debug("context is done, returning from 'Create'")
				return nil, ctx.Err()
			}
		case err := <-errChan:
			{
				c.logger.Error("failed create flashcard", slog.Any("reason", err.Error()))
				return nil, err
			}
		// Receiver get picture and state 'is channel open'
		// If channel is closed, set this channel to nil.
		// If received picture isn't nil, set this picture to card,
		// but if picture is nil, ignore it
		case pictureFromChan, isOpen := <-pictureChan:
			{
				if !isOpen {
					pictureChan = nil
				} else {
					if pictureFromChan != nil {
						pictureForCard = pictureFromChan
					}
				}
			}
		// Receiver get examples and state 'is channel open'
		// If channel is closed, set this channel to nil.
		// If received examples aren't nil, set this examples to card,
		// but if examples is nil, ignore it
		case examplesFromChan, isOpen := <-examplesChan:
			{
				if !isOpen {
					examplesChan = nil
				} else {
					if examplesFromChan != nil {
						examples = examplesFromChan
					}
				}
			}
		}

		// When channels are nil, it means that all workers are complete
		if pictureChan == nil && examplesChan == nil {
			break
		}
	}

	flashcard := &models.Flashcard{
		Subject:       cambridgeCard.Subject,
		SubjectType:   cambridgeCard.SubjectType,
		DeckName:      deck,
		Pronouns:      cambridgeCard.Pronouns,
		Transcription: cambridgeCard.Transcription,
		Explain:       cambridgeCard.Explains[0],
		Picture:       pictureForCard,
		Examples:      *examples,
	}

	return flashcard, nil
}

// Method for rating pictures gotten from google image
// If all attempts images don't match, creating card continue without picture
func (c *implFlashcardCreator) tryRatingPicture(ctx context.Context, subject string, pictureChan chan *models.File, errChan chan error) {
	for attempt := range c.creatorCfg.NumberOfAttemptRatingPicture {
		c.logger.Debug(fmt.Sprintf("attempt: %d for getting picture for subject: %s", attempt, subject))
		picture, err := c.googleImageProvider.Get(ctx, attempt, subject)
		if err != nil {
			c.logger.Error("failed get picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			errChan <- err
		}

		rating, err := c.geminiProvider.RatingPicture(ctx, subject, picture)
		if err != nil {
			c.logger.Error("failed rating picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			errChan <- err
		}

		if *rating >= c.creatorCfg.MinimalRating {
			c.logger.Debug(fmt.Sprintf("found suitable picture for subject: %s", subject))
			pictureChan <- picture
		}
	}

	c.logger.Warn(fmt.Sprintf("picture for subject: %s not found, continue without picture", subject))
}

// In some cases, Cambridge dictionary doenst have example of using word/phrase.
// That's why this check is neccessary
func (c *implFlashcardCreator) checkAndGetExamples(ctx context.Context, subject string, cambridgeCard *models.CambridgeCard, examplesChan chan *[]string, errChan chan error) {
	var examples *[]string
	if len(cambridgeCard.Examples) != 0 {
		examples = &cambridgeCard.Examples
	} else {
		geminiExamples, err := c.geminiProvider.GenerateExamples(ctx, subject)
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed generate examples for subject: %s", subject), slog.Any("err", err.Error()))
			errChan <- err
		}
		examples = geminiExamples
	}

	examplesChan <- examples
}
