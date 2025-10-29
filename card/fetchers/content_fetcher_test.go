package fetchers

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/extractors"
	mock_extractors "github.com/shredd0r/anki-card-creator/extractors/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
	mock_providers "github.com/shredd0r/anki-card-creator/providers/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type cambridgeExtractorExpectedCalls func(subject string, expectedCardContent *models.CardContent, ce *mock_extractors.MockCambridgeCardExtractor)
type geminiProviderExpectedCalls func(subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_providers.MockGeminiProvider)
type methodForInitCardContentComponentFetcher func(*slog.Logger, extractors.CambridgeCardExtractor, providers.GeminiProvider) CardContentComponentFetcher

type fieldFetcherPositiveCase struct {
	Name                                   string
	Subject                                string
	ExpectedSubjectType                    models.SubjectType
	ExpectedCardContent                    models.CardContent
	InitCardContentComponentFetcherForTest methodForInitCardContentComponentFetcher
	CambridgeExtractorExpectedCalls        cambridgeExtractorExpectedCalls
	GeminiProviderExpectedCalls            geminiProviderExpectedCalls
}

func TestPostiveCases(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	transcriptionForWord := "word-transcription"
	testcases := []fieldFetcherPositiveCase{
		{
			Name:                "field component fetcher for word",
			Subject:             "subject-word",
			ExpectedSubjectType: models.SubjectTypeWord,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "word-paraphrase",
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentComponentFetcherForTest: initCardContentComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:        cambridgeCardExtractorExpectedCallsForReturnVolumes,
			GeminiProviderExpectedCalls:            geminiProviderExpectedCallsGenerateCardContent,
		},
		{
			Name:                "field component fetcher for word, where cambridge extractor didnt return examples",
			Subject:             "subject-word",
			ExpectedSubjectType: models.SubjectTypeWord,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "word-paraphrase",
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentComponentFetcherForTest: initCardContentComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:        cambridgeCardExtractorExpectedCallsReturnVolumesExceptExamples,
			GeminiProviderExpectedCalls:            geminiProviderExpectedCallsGenerateCardContent,
		},
		{
			Name:                "field component fetcher for phrase",
			Subject:             "subject-phrase",
			ExpectedSubjectType: models.SubjectTypePhrase,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "phrase-paraphrase",
				Transcription: nil,
				Pronunciation: nil,
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentComponentFetcherForTest: initCardContentComponentFetcherForPhrase,
			CambridgeExtractorExpectedCalls:        cambridgeCardExtractorWithoutExpectedCalls,
			GeminiProviderExpectedCalls:            geminiProviderExpectedCallsGenerateCardContent,
		},
	}
	usingContextForTest := &[]string{"some-context"}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractors.NewMockCambridgeCardExtractor(ctrl)
			gp := mock_providers.NewMockGeminiProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(testcase.Subject, &testcase.ExpectedCardContent, ce)
			testcase.GeminiProviderExpectedCalls(testcase.Subject, usingContextForTest, &testcase.ExpectedCardContent, gp)

			cardContentComponentFetcher := testcase.InitCardContentComponentFetcherForTest(logger, ce, gp)

			actualCardContent, err := cardContentComponentFetcher.GetCardContent(t.Context(), testcase.Subject, usingContextForTest)
			assert.Nil(t, err)
			assert.Equal(t, testcase.ExpectedCardContent, *actualCardContent)

			actualSubjectType := cardContentComponentFetcher.GetSubjectType()
			assert.Equal(t, testcase.ExpectedSubjectType, actualSubjectType)
		})
	}
}

type fieldFetcherNegativeCase struct {
	Name                                   string
	SubjectType                            models.SubjectType
	ExpectedMessageErr                     string
	InitCardContentComponentFetcherForTest methodForInitCardContentComponentFetcher
	CambridgeExtractorExpectedCalls        cambridgeExtractorExpectedCalls
	GeminiProviderExpectedCalls            geminiProviderExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	testcases := []fieldFetcherNegativeCase{
		{
			Name:                                   "field component fetcher for word, cambridge extractor return error",
			SubjectType:                            models.SubjectTypeWord,
			ExpectedMessageErr:                     "error from cambridge card extractor",
			InitCardContentComponentFetcherForTest: initCardContentComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:        cambridgeCardExtractorExpectedCallsEveryTimeReturnErr,
			GeminiProviderExpectedCalls:            geminiProviderWithoutExpectedCalls,
		},
		{
			Name:                                   "field component fetcher for phrase, gemini provider return error",
			SubjectType:                            models.SubjectTypePhrase,
			ExpectedMessageErr:                     "error from gemini provider",
			InitCardContentComponentFetcherForTest: initCardContentComponentFetcherForPhrase,
			CambridgeExtractorExpectedCalls:        cambridgeCardExtractorWithoutExpectedCalls,
			GeminiProviderExpectedCalls:            geminiProviderExpectedCallsReturnErr,
		},
	}

	subjectForTests := "test-subject"
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractors.NewMockCambridgeCardExtractor(ctrl)
			gp := mock_providers.NewMockGeminiProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(subjectForTests, nil, ce)
			testcase.GeminiProviderExpectedCalls(subjectForTests, nil, nil, gp)

			cardContentComponentFetcher := testcase.InitCardContentComponentFetcherForTest(logger, ce, gp)

			actualCardContent, err := cardContentComponentFetcher.GetCardContent(t.Context(), subjectForTests, nil)
			assert.Nil(t, actualCardContent)
			assert.EqualError(t, err, testcase.ExpectedMessageErr)
		})
	}
}

func initCardContentComponentFetcherForWord(logger *slog.Logger, ce extractors.CambridgeCardExtractor, gp providers.GeminiProvider) CardContentComponentFetcher {
	return NewWordCardContentComponentFetcher(logger, gp, ce)
}

func initCardContentComponentFetcherForPhrase(logger *slog.Logger, ce extractors.CambridgeCardExtractor, gp providers.GeminiProvider) CardContentComponentFetcher {
	return NewPhraseCardContentComponentFetcher(logger, gp)
}

func cambridgeCardExtractorExpectedCallsForReturnVolumes(subject string, expectedCardContent *models.CardContent, ce *mock_extractors.MockCambridgeCardExtractor) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&models.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedCardContent.Pronunciation,
			Transcription: expectedCardContent.Transcription,
			Examples:      expectedCardContent.Examples,
			Explains:      []string{expectedCardContent.Paraphrase},
		}, nil)
}

func cambridgeCardExtractorExpectedCallsReturnVolumesExceptExamples(subject string, expectedCardContent *models.CardContent, ce *mock_extractors.MockCambridgeCardExtractor) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&models.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedCardContent.Pronunciation,
			Transcription: expectedCardContent.Transcription,
			Explains:      []string{expectedCardContent.Paraphrase},
		}, nil)
}

func cambridgeCardExtractorExpectedCallsEveryTimeReturnErr(subject string, expectedCardContent *models.CardContent, ce *mock_extractors.MockCambridgeCardExtractor) {
	ce.EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*models.CambridgeCard, error) {
			return nil, errors.New("error from cambridge card extractor")
		})
}

func cambridgeCardExtractorWithoutExpectedCalls(subject string, expectedCardContent *models.CardContent, ce *mock_extractors.MockCambridgeCardExtractor) {
}

func geminiProviderExpectedCallsGenerateCardContent(subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		Times(1).
		Return(&models.GeminiCard{
			Paraphrase: expectedCardContent.Paraphrase,
			Examples:   expectedCardContent.Examples,
			Synonyms:   expectedCardContent.Synonyms,
		}, nil)
}

func geminiProviderExpectedCallsReturnErr(subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		Times(1).
		Return(nil, errors.New("error from gemini provider"))
}

func geminiProviderWithoutExpectedCalls(subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_providers.MockGeminiProvider) {

}
