package fetchers

//go:generate mockgen -source content_fetcher.go -destination mock/content_fetcher_mock.go

import (
	"context"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/extractors"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
)

type CardContentComponentFetcher interface {
	GetSubjectType() models.SubjectType
	GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error)
}

// wordCardContentComponentFetcher has methods for get volumes for flashcard for word
type wordCardContentComponentFetcher struct {
	logger             *slog.Logger
	geminiProvider     providers.GeminiProvider
	cambridgeExtractor extractors.CambridgeCardExtractor
}

func NewWordCardContentComponentFetcher(logger *slog.Logger, geminiProvider providers.GeminiProvider, cambridgeExtractor extractors.CambridgeCardExtractor) CardContentComponentFetcher {
	return &wordCardContentComponentFetcher{
		logger:             logger.WithGroup("word-field-component-fetcher"),
		geminiProvider:     geminiProvider,
		cambridgeExtractor: cambridgeExtractor,
	}
}

func (wf *wordCardContentComponentFetcher) GetSubjectType() models.SubjectType {
	return models.SubjectTypeWord
}

func (wf *wordCardContentComponentFetcher) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	wf.logger.Debug("start get card content for word")
	cambridgeCard, err := wf.cambridgeExtractor.GetCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	geminiCardContent, err := wf.geminiProvider.GenerateCardContent(ctx, subject, usingContext)
	if err != nil {
		return nil, err
	}

	examples := cambridgeCard.Examples
	if len(examples) == 0 {
		examples = geminiCardContent.Examples
	}

	cardContent := models.CardContent{
		Paraphrase:    cambridgeCard.Explains[0],
		Transcription: cambridgeCard.Transcription,
		Pronunciation: cambridgeCard.Pronunciation,
		Examples:      examples,
		Synonyms:      geminiCardContent.Synonyms,
	}

	return &cardContent, nil
}

type phraseCardContentComponentFetcher struct {
	logger         *slog.Logger
	geminiProvider providers.GeminiProvider
}

func NewPhraseCardContentComponentFetcher(logger *slog.Logger, geminiProvider providers.GeminiProvider) CardContentComponentFetcher {
	return &phraseCardContentComponentFetcher{
		logger:         logger.WithGroup("phrase-field-component-fetcher"),
		geminiProvider: geminiProvider,
	}
}

func (pf *phraseCardContentComponentFetcher) GetSubjectType() models.SubjectType {
	return models.SubjectTypePhrase
}

func (pf *phraseCardContentComponentFetcher) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	pf.logger.Debug("start get card content for word")

	geminiCard, err := pf.geminiProvider.GenerateCardContent(ctx, subject, usingContext)
	if err != nil {
		return nil, err
	}

	// For phrase doesn't return transcription and pronunciation
	return &models.CardContent{
		Paraphrase:    geminiCard.Paraphrase,
		Transcription: nil,
		Pronunciation: nil,
		Examples:      geminiCard.Examples,
		Synonyms:      geminiCard.Synonyms,
	}, nil
}
