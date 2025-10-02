package card

//go:generate mockgen -source fetcher_factory.go -destination mock/fetcher_factory_mock.go

import (
	"errors"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/extractors"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
)

var errUnsupportedSubjectType = errors.New("unsupported subject type")

type FieldComponentFetcherFactory interface {
	Get(subjectType models.SubjectType) (FieldComponentFetcher, error)
}

type implFieldComponentFetcherFactory struct {
	logger                  *slog.Logger
	cambridgeExtractor      extractors.CambridgeCardExtractor
	geminiProvider          providers.GeminiProvider
	phraseCardMethodCreator FieldComponentFetcher
}

func NewFieldComponentFetcherFactory(logger *slog.Logger, cambridgeExtractor extractors.CambridgeCardExtractor,
	geminiProvider providers.GeminiProvider) FieldComponentFetcherFactory {
	return &implFieldComponentFetcherFactory{
		logger:                  logger.WithGroup("field-component-fetcher-factory"),
		cambridgeExtractor:      cambridgeExtractor,
		geminiProvider:          geminiProvider,
		phraseCardMethodCreator: NewPhraseFieldComponentFetcher(logger, geminiProvider),
	}
}

func (f *implFieldComponentFetcherFactory) Get(subjectType models.SubjectType) (FieldComponentFetcher, error) {
	f.logger.Debug("start get card method creator", slog.Any("subjectType", subjectType))

	switch subjectType {
	// For word, will be created new instanse for each word.
	// If don't do it, then has to call cambridge extractor in each CardMethodCreator methods
	case models.SubjectTypeWord:
		{
			f.logger.Debug("returning creator for word", slog.Any("subjectType", subjectType))

			newWordCardFieldComponentFetcher := NewWordFieldComponentFetcher(f.logger, f.geminiProvider, f.cambridgeExtractor)
			return newWordCardFieldComponentFetcher, nil
		}

	case models.SubjectTypePhrase:
		{
			f.logger.Debug("returning creator for phrase", slog.Any("subjectType", subjectType))
			return f.phraseCardMethodCreator, nil
		}
	default:
		{
			return nil, errUnsupportedSubjectType
		}
	}
}
