package fetchers

//go:generate mockgen -source field_fetcher.go -destination mock/field_fetcher_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/shredd0r/anki-card-creator/extractors"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
)

type FieldComponentFetcher interface {
	GetSubjectType() models.SubjectType
	GetTranscription(ctx context.Context, subject string) (*string, error)
	GetExplain(ctx context.Context, subject string) (*string, error)
	GetExamples(ctx context.Context, subject string) (*[]string, error)
	GetPronunciation(ctx context.Context, subject string) (*models.File, error)
}

// wordFieldComponentFetcher has methods for get volumes for flashcard for word
type wordFieldComponentFetcher struct {
	logger             *slog.Logger
	geminiProvider     providers.GeminiProvider
	cambridgeExtractor extractors.CambridgeCardExtractor
	_cambridgeCard     *models.CambridgeCard
	mu                 sync.Mutex
}

func NewWordFieldComponentFetcher(logger *slog.Logger, geminiProvider providers.GeminiProvider, cambridgeExtractor extractors.CambridgeCardExtractor) FieldComponentFetcher {

	return &wordFieldComponentFetcher{
		logger:             logger.WithGroup("word-field-component-fetcher"),
		geminiProvider:     geminiProvider,
		cambridgeExtractor: cambridgeExtractor,
	}
}

func (wf *wordFieldComponentFetcher) GetSubjectType() models.SubjectType {
	return models.SubjectTypeWord
}
func (wf *wordFieldComponentFetcher) GetTranscription(ctx context.Context, subject string) (*string, error) {
	wf.logger.Debug("start get transcription for word", slog.Any("word", subject))

	select {
	case <-ctx.Done():
		{
			wf.logger.Debug("context is done, returning from GetTranscription")
			return nil, ctx.Err()
		}
	default:
		{
			cambridgeCard, err := wf.fetchCambridgeCard(ctx, subject)
			if err != nil {
				return nil, err
			}
			return cambridgeCard.Transcription, nil
		}
	}
}

func (wf *wordFieldComponentFetcher) GetExplain(ctx context.Context, subject string) (*string, error) {
	wf.logger.Debug("start get explain for word", slog.Any("word", subject))

	cambridgeCard, err := wf.fetchCambridgeCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	return &cambridgeCard.Explains[0], nil
}

// In some cases, Cambridge dictionary doenst have example of using word
// That's why this check is neccessary
func (wf *wordFieldComponentFetcher) GetExamples(ctx context.Context, subject string) (*[]string, error) {
	wf.logger.Debug("start get examples for word", slog.Any("word", subject))

	cambridgeCard, err := wf.fetchCambridgeCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	if len(cambridgeCard.Examples) != 0 {
		return &cambridgeCard.Examples, nil
	}

	geminiExamples, err := wf.geminiProvider.GenerateExamples(ctx, subject)
	if err != nil {
		wf.logger.Error(fmt.Sprintf("failed generate examples for subject: %s", subject), slog.Any("err", err.Error()))
		return nil, err
	}
	return geminiExamples, nil
}
func (wf *wordFieldComponentFetcher) GetPronunciation(ctx context.Context, subject string) (*models.File, error) {
	wf.logger.Debug("start get pronunciation for word", slog.Any("word", subject))

	cambridgeCard, err := wf.fetchCambridgeCard(ctx, subject)
	if err != nil {
		return nil, err
	}

	return cambridgeCard.Pronunciation, nil
}

func (wf *wordFieldComponentFetcher) fetchCambridgeCard(ctx context.Context, subject string) (*models.CambridgeCard, error) {
	select {
	case <-ctx.Done():
		{
			wf.logger.Debug("context is done, returning from fetchCambridgeCard")
			return nil, ctx.Err()
		}
	default:
		{
			wf.mu.Lock()
			wf.logger.Debug("call fetchCambridgeCard")
			defer wf.mu.Unlock()

			if wf._cambridgeCard == nil {
				wf.logger.Debug("get card from cambridge")
				newCambridgeCard, err := wf.cambridgeExtractor.GetCard(ctx, subject)
				if err != nil {
					return nil, err
				}
				wf._cambridgeCard = newCambridgeCard
			}

			return wf._cambridgeCard, nil
		}
	}

}

type phraseFieldComponentFetcher struct {
	logger         *slog.Logger
	geminiProvider providers.GeminiProvider
}

func NewPhraseFieldComponentFetcher(logger *slog.Logger, geminiProvider providers.GeminiProvider) FieldComponentFetcher {
	return &phraseFieldComponentFetcher{
		logger:         logger.WithGroup("phrase-field-component-fetcher"),
		geminiProvider: geminiProvider,
	}
}

func (pf *phraseFieldComponentFetcher) GetSubjectType() models.SubjectType {
	return models.SubjectTypePhrase
}

// For phrase doesn't return transcription
func (pf *phraseFieldComponentFetcher) GetTranscription(ctx context.Context, subject string) (*string, error) {
	return nil, nil
}
func (pf *phraseFieldComponentFetcher) GetExplain(ctx context.Context, subject string) (*string, error) {
	pf.logger.Debug("start get explain for phrase", slog.Any("phrase", subject))
	return pf.geminiProvider.GenerateExplain(ctx, subject)
}
func (pf *phraseFieldComponentFetcher) GetExamples(ctx context.Context, subject string) (*[]string, error) {
	pf.logger.Debug("start get explain for phrase", slog.Any("phrase", subject))
	return pf.geminiProvider.GenerateExamples(ctx, subject)
}

// For phrase doesn't return transcription
func (pf *phraseFieldComponentFetcher) GetPronunciation(ctx context.Context, subject string) (*models.File, error) {
	return nil, nil
}
