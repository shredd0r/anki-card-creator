package fetcher

//go:generate mockgen -source factory.go -destination mock/factory_mock.go

import (
	"errors"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/models"
)

var errUnsupportedSubjectType = errors.New("unsupported subject type")

type CardContentFactory interface {
	Get(subjectType models.SubjectType) (CardContent, error)
}

type implCardContentFactory struct {
	logger             *slog.Logger
	cambridgeExtractor extractor.Cambridge
	llmProvider        llm.Provider
	wordCardContent    CardContent
	phraseCardContent  CardContent
}

func NewCardContentFactory(cfg config.Config, logger *slog.Logger, cambridgeExtractor extractor.Cambridge,
	llmProvider llm.Provider) CardContentFactory {
	var wordCardContent CardContent
	if cfg.OnlyAI {
		wordCardContent = NewWordCardContentByAI(logger, llmProvider, cambridgeExtractor)
	} else {
		wordCardContent = NewWordCardContentByCambridge(logger, llmProvider, cambridgeExtractor)
	}
	return &implCardContentFactory{
		logger:             logger.WithGroup("field-component-fetcher-factory"),
		cambridgeExtractor: cambridgeExtractor,
		llmProvider:        llmProvider,
		wordCardContent:    wordCardContent,
		phraseCardContent:  NewPhraseCardContent(logger, llmProvider),
	}
}

func (f *implCardContentFactory) Get(subjectType models.SubjectType) (CardContent, error) {
	f.logger.Debug("start get card method creator", slog.Any("subjectType", subjectType))

	switch subjectType {
	// For word, will be created new instanse for each word.
	// If don't do it, then has to call cambridge extractor in each CardMethodCreator methods
	case models.SubjectTypeWord:
		{
			f.logger.Debug("returning creator for word", slog.Any("subjectType", subjectType))
			return f.wordCardContent, nil
		}

	case models.SubjectTypePhrase:
		{
			f.logger.Debug("returning creator for phrase", slog.Any("subjectType", subjectType))
			return f.phraseCardContent, nil
		}
	default:
		{
			return nil, errUnsupportedSubjectType
		}
	}
}
