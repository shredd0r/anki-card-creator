package fetcher

//go:generate mockgen -source content.go -destination mock/content_mock.go

import (
	"context"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/models"
)

type CardContent interface {
	GetSubjectType() models.SubjectType
	GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error)
}

// wordCardContent has methods for get volumes for flashcard for word
type wordCardContent struct {
	logger             *slog.Logger
	llmProvider        llm.Provider
	cambridgeExtractor extractor.Cambridge
}

func NewWordCardContent(logger *slog.Logger, llmProvider llm.Provider, cambridgeExtractor extractor.Cambridge) CardContent {
	return &wordCardContent{
		logger:             logger.WithGroup("word-field-component-fetcher"),
		llmProvider:        llmProvider,
		cambridgeExtractor: cambridgeExtractor,
	}
}

func (wf *wordCardContent) GetSubjectType() models.SubjectType {
	return models.SubjectTypeWord
}

func (wf *wordCardContent) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	wf.logger.Debug("start get card content for word")
	cambridgeCard, err := wf.cambridgeExtractor.GetCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	geminiCardContent, err := wf.llmProvider.GenerateCardContent(ctx, subject, usingContext)
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

type phraseCardContent struct {
	logger      *slog.Logger
	llmProvider llm.Provider
}

func NewPhraseCardContent(logger *slog.Logger, llmProvider llm.Provider) CardContent {
	return &phraseCardContent{
		logger:      logger.WithGroup("phrase-field-component-fetcher"),
		llmProvider: llmProvider,
	}
}

func (pf *phraseCardContent) GetSubjectType() models.SubjectType {
	return models.SubjectTypePhrase
}

func (pf *phraseCardContent) GetCardContent(ctx context.Context, subject string, usingContext *[]string) (*models.CardContent, error) {
	pf.logger.Debug("start get card content for phrase")

	geminiCard, err := pf.llmProvider.GenerateCardContent(ctx, subject, usingContext)
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
