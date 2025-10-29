package fetchers

//go:generate mockgen -source fetcher_factory.go -destination mock/fetcher_factory_mock.go

import (
	"errors"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/extractors"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
)

var errUnsupportedSubjectType = errors.New("unsupported subject type")

type CardContentComponentFetcherFactory interface {
	Get(subjectType models.SubjectType) (CardContentComponentFetcher, error)
}

type implFieldComponentFetcherFactory struct {
	logger                    *slog.Logger
	cambridgeExtractor        extractors.CambridgeCardExtractor
	geminiProvider            providers.GeminiProvider
	wordFieldComponentFetcher CardContentComponentFetcher
	phraseCardMethodCreator   CardContentComponentFetcher
}

func NewCardContentComponentFetcherFactory(logger *slog.Logger, cambridgeExtractor extractors.CambridgeCardExtractor,
	geminiProvider providers.GeminiProvider) CardContentComponentFetcherFactory {
	return &implFieldComponentFetcherFactory{
		logger:                    logger.WithGroup("field-component-fetcher-factory"),
		cambridgeExtractor:        cambridgeExtractor,
		geminiProvider:            geminiProvider,
		wordFieldComponentFetcher: NewWordCardContentComponentFetcher(logger, geminiProvider, cambridgeExtractor),
		phraseCardMethodCreator:   NewPhraseCardContentComponentFetcher(logger, geminiProvider),
	}
}

func (f *implFieldComponentFetcherFactory) Get(subjectType models.SubjectType) (CardContentComponentFetcher, error) {
	f.logger.Debug("start get card method creator", slog.Any("subjectType", subjectType))

	switch subjectType {
	// For word, will be created new instanse for each word.
	// If don't do it, then has to call cambridge extractor in each CardMethodCreator methods
	case models.SubjectTypeWord:
		{
			f.logger.Debug("returning creator for word", slog.Any("subjectType", subjectType))
			return f.wordFieldComponentFetcher, nil
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
