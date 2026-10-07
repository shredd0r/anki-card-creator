package fetcher

//go:generate mockgen -source content.go -destination mock/content_mock.go

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/imagesearch"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/tts"
)

type CardContent interface {
	GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error)
	// Method for rating pictures gotten from the image search provider
	GetPicture(ctx context.Context, subject string, usingContext *[]string) (*models.File, error)
	GetPronunciation(ctx context.Context, subject string) (*models.File, error)
}

// mediaFetcher implements the picture-search/rating and TTS-pronunciation
// logic shared identically by both scenarios (withDictionaryCardContent and
// onlyAICardContent embed it) so it isn't duplicated.
type mediaFetcher struct {
	cfg                 config.PictureConfig
	logger              *slog.Logger
	speech              tts.Speech
	llmProvider         llm.Provider
	imageSearchProvider imagesearch.Image
}

func (m *mediaFetcher) GetPicture(ctx context.Context, subject string, usingContext *[]string) (*models.File, error) {
	if m.cfg.Ignore {
		m.logger.Debug("picture search disabled via config, skip")
		return nil, nil
	}

	var query string
	if usingContext == nil {
		query = subject
	} else {
		query = fmt.Sprintf("%s %s", subject, strings.Join(*usingContext, ", "))
	}

	queryPageProvider, err := m.imageSearchProvider.Request(ctx, query)
	if err != nil {
		return nil, err
	}

	defer queryPageProvider.Close()

	for attempt := range m.cfg.CountSearches {
		m.logger.Debug(fmt.Sprintf("attempt: %d for getting picture for subject: %s", attempt, subject))

		picture, err := queryPageProvider.Get(ctx, subject, uint(attempt))
		if err != nil {
			m.logger.Error("failed get picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		rating, err := m.llmProvider.RatePicture(ctx, subject, picture)
		if err != nil {
			m.logger.Error("failed rating picture for subject", slog.Any("subject", subject), slog.Any("attempt", attempt))
			return nil, err
		}

		if rating.Rating >= m.cfg.MinimalRating {
			m.logger.Debug(fmt.Sprintf("found suitable picture for subject: %s", subject))
			return picture, nil
		}
	}

	m.logger.Warn(fmt.Sprintf("picture for subject: %s not found, continue without picture", subject))
	return nil, nil
}

func (m *mediaFetcher) GetPronunciation(ctx context.Context, subject string) (*models.File, error) {
	filename := fmt.Sprintf("%s-prononunciation", subject)

	pronunciation, err := m.speech.Create(ctx, subject)
	if err != nil {
		m.logger.Error("failed generate pronunciation", "err", err.Error())
		return nil, err
	}

	contentBytes, err := io.ReadAll(pronunciation)
	if err != nil {
		m.logger.Error("failed read pronunciation reader", "err", err.Error())
		return nil, err
	}

	return &models.File{
		Filename: filename,
		Content:  contentBytes,
		MIMEType: "audio/mpeg",
	}, nil
}

// withDictionaryCardContent tries Cambridge Dictionary first for paraphrase,
// transcription and examples, falling back to AI generation per-field for
// whatever Cambridge doesn't have (including entirely, e.g. for a
// phrase-shaped subject Cambridge doesn't support). Synonyms always come
// from AI - Cambridge has no synonym data.
type withDictionaryCardContent struct {
	mediaFetcher
	cambridgeExtractor extractor.Cambridge
}

func NewWithDictionaryCardContent(cfg config.PictureConfig, logger *slog.Logger, imageSearchProvider imagesearch.Image, llmProvider llm.Provider, cambridgeExtractor extractor.Cambridge, speech tts.Speech) CardContent {
	return &withDictionaryCardContent{
		mediaFetcher: mediaFetcher{
			cfg:                 cfg,
			logger:              logger.WithGroup("with-dictionary-card-content"),
			speech:              speech,
			llmProvider:         llmProvider,
			imageSearchProvider: imageSearchProvider,
		},
		cambridgeExtractor: cambridgeExtractor,
	}
}

func (wf *withDictionaryCardContent) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	wf.logger.Debug("get card content with dictionary")

	var cambridgeCard *extractor.CambridgeCard
	var cambridgeErr error
	var generatedCardContent *llm.GeneratedCardContent
	var llmErr error

	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		cambridgeCard, cambridgeErr = wf.cambridgeExtractor.GetCard(ctx, subject)
	}()
	go func() {
		defer wg.Done()
		generatedCardContent, llmErr = wf.llmProvider.GenerateCardContent(ctx, subject, usingContext)
	}()
	wg.Wait()

	// The LLM call is not optional (synonyms only ever come from it, and it
	// backfills anything Cambridge misses), so its failure is fatal. A
	// Cambridge failure is not - it just means every text field falls back
	// to the AI-generated ones below.
	if llmErr != nil {
		return nil, llmErr
	}
	if cambridgeErr != nil {
		wf.logger.Debug("cambridge dictionary lookup failed, falling back to AI for all text fields", slog.Any("err", cambridgeErr.Error()))
	}

	paraphrase := generatedCardContent.Paraphrase
	if cambridgeCard != nil && len(cambridgeCard.Explains) > 0 {
		paraphrase = cambridgeCard.Explains[0]
	}

	transcription := generatedCardContent.Transcription
	if cambridgeCard != nil && cambridgeCard.Transcription != nil {
		transcription = cambridgeCard.Transcription
	}

	examples := generatedCardContent.Examples
	if cambridgeCard != nil && len(cambridgeCard.Examples) > 0 {
		examples = cambridgeCard.Examples
	}

	var pronunciation *models.File
	if cambridgeCard != nil {
		pronunciation = cambridgeCard.Pronunciation
	}

	return &models.CardContent{
		Paraphrase:    paraphrase,
		Transcription: transcription,
		Pronunciation: pronunciation,
		Examples:      examples,
		Synonyms:      generatedCardContent.Synonyms,
	}, nil
}

// onlyAICardContent generates every text field via AI and pronunciation via
// TTS, never touching Cambridge. Picture search/rating is shared with
// withDictionaryCardContent via mediaFetcher.
type onlyAICardContent struct {
	mediaFetcher
}

func NewOnlyAICardContent(cfg config.PictureConfig, logger *slog.Logger, imageSearchProvider imagesearch.Image, llmProvider llm.Provider, speech tts.Speech) CardContent {
	return &onlyAICardContent{
		mediaFetcher: mediaFetcher{
			cfg:                 cfg,
			logger:              logger.WithGroup("only-ai-card-content"),
			speech:              speech,
			llmProvider:         llmProvider,
			imageSearchProvider: imageSearchProvider,
		},
	}
}

func (of *onlyAICardContent) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	of.logger.Debug("get card content with only ai")

	generatedCardContent, err := of.llmProvider.GenerateCardContent(ctx, subject, usingContext)
	if err != nil {
		return nil, err
	}

	of.logger.Debug("card content by ai received")
	return &models.CardContent{
		Paraphrase:    generatedCardContent.Paraphrase,
		Transcription: generatedCardContent.Transcription,
		Pronunciation: nil, // comes from GetPronunciation (TTS) instead
		Examples:      generatedCardContent.Examples,
		Synonyms:      generatedCardContent.Synonyms,
	}, nil
}
