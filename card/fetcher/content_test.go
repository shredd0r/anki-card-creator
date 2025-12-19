package fetcher

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/extractor"
	mock_extractor "github.com/shredd0r/anki-card-creator/extractor/mock"
	"github.com/shredd0r/anki-card-creator/llm"
	mock_llm "github.com/shredd0r/anki-card-creator/llm/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type cambridgeExtractorExpectedCalls func(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge)
type llmProviderExpectedCalls func(subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_llm.MockProvider)
type methodForInitCardContentFetcher func(*slog.Logger, extractor.Cambridge, llm.Provider) CardContent

type fieldFetcherPositiveCase struct {
	Name                            string
	Subject                         string
	ExpectedSubjectType             models.SubjectType
	ExpectedCardContent             models.CardContent
	InitCardContentFetcherForTest   methodForInitCardContentFetcher
	CambridgeExtractorExpectedCalls cambridgeExtractorExpectedCalls
	LLMProviderExpectedCalls        llmProviderExpectedCalls
}

func TestPostiveCases(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	transcriptionForWord := "word-transcription"
	testcases := []fieldFetcherPositiveCase{
		{
			Name:                "field fetcher for word by cambridge",
			Subject:             "subject-word",
			ExpectedSubjectType: models.SubjectTypeWord,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "word-paraphrase",
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentFetcherForTest:   initCardContentFetcherForWordByCambridge,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectedCallsForReturnVolumes,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsGenerateCardContent,
		},
		{
			Name:                "field fetcher for word by AI",
			Subject:             "subject-word",
			ExpectedSubjectType: models.SubjectTypeWord,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "word-paraphrase",
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentFetcherForTest:   initCardContentFetcherForWordByAI,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectedCallOnlyGetPronunciation,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsGenerateCardContent,
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
			InitCardContentFetcherForTest:   initCardContentFetcherForWordByCambridge,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectedCallsReturnVolumesExceptExamples,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsGenerateCardContent,
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
			InitCardContentFetcherForTest:   initCardContentFetcherForPhrase,
			CambridgeExtractorExpectedCalls: cambridgeExtractorWithoutExpectedCalls,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsGenerateCardContent,
		},
	}
	usingContextForTest := &[]string{"some-context"}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractor.NewMockCambridge(ctrl)
			gp := mock_llm.NewMockProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(testcase.Subject, &testcase.ExpectedCardContent, ce)
			testcase.LLMProviderExpectedCalls(testcase.Subject, usingContextForTest, &testcase.ExpectedCardContent, gp)

			cardContentComponentFetcher := testcase.InitCardContentFetcherForTest(logger, ce, gp)

			actualCardContent, err := cardContentComponentFetcher.GetCardContent(t.Context(), testcase.Subject, usingContextForTest)
			assert.Nil(t, err)
			assert.Equal(t, testcase.ExpectedCardContent, *actualCardContent)

			actualSubjectType := cardContentComponentFetcher.GetSubjectType()
			assert.Equal(t, testcase.ExpectedSubjectType, actualSubjectType)
		})
	}
}

type fieldFetcherNegativeCase struct {
	Name                            string
	SubjectType                     models.SubjectType
	ExpectedMessageErr              string
	InitCardContentFetcherForTest   methodForInitCardContentFetcher
	CambridgeExtractorExpectedCalls cambridgeExtractorExpectedCalls
	LLMProviderExpectedCalls        llmProviderExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	testcases := []fieldFetcherNegativeCase{
		{
			Name:                            "field component fetcher for word, cambridge extractor return error",
			SubjectType:                     models.SubjectTypeWord,
			ExpectedMessageErr:              "error from cambridge card extractor",
			InitCardContentFetcherForTest:   initCardContentFetcherForWordByCambridge,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectedCallsEveryTimeReturnErr,
			LLMProviderExpectedCalls:        llmProviderExpectedOneOrNoneCall,
		},
		{
			Name:                            "field component fetcher for word, llm provider return error",
			SubjectType:                     models.SubjectTypeWord,
			ExpectedMessageErr:              "error from llm provider",
			InitCardContentFetcherForTest:   initCardContentFetcherForWordByAI,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectOneOrNoneGetPronunciationCall,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsReturnErr,
		},
		{
			Name:                            "field component fetcher for phrase, llm provider return error",
			SubjectType:                     models.SubjectTypePhrase,
			ExpectedMessageErr:              "error from llm provider",
			InitCardContentFetcherForTest:   initCardContentFetcherForPhrase,
			CambridgeExtractorExpectedCalls: cambridgeExtractorWithoutExpectedCalls,
			LLMProviderExpectedCalls:        llmProviderExpectedCallsReturnErr,
		},
	}

	subjectForTests := "test-subject"
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractor.NewMockCambridge(ctrl)
			gp := mock_llm.NewMockProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(subjectForTests, nil, ce)
			testcase.LLMProviderExpectedCalls(subjectForTests, nil, nil, gp)

			cardContentComponentFetcher := testcase.InitCardContentFetcherForTest(logger, ce, gp)

			actualCardContent, err := cardContentComponentFetcher.GetCardContent(t.Context(), subjectForTests, nil)
			assert.Nil(t, actualCardContent)
			assert.EqualError(t, err, testcase.ExpectedMessageErr)
		})
	}
}

func initCardContentFetcherForWordByCambridge(logger *slog.Logger, ce extractor.Cambridge, llmp llm.Provider) CardContent {
	return NewWordCardContentByCambridge(logger, llmp, ce)
}

func initCardContentFetcherForWordByAI(logger *slog.Logger, ce extractor.Cambridge, llmp llm.Provider) CardContent {
	return NewWordCardContentByAI(logger, llmp, ce)
}

func initCardContentFetcherForPhrase(logger *slog.Logger, ce extractor.Cambridge, llmp llm.Provider) CardContent {
	return NewPhraseCardContent(logger, llmp)
}

func cambridgeExtractorExpectedCallOnlyGetPronunciation(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
	ce.
		EXPECT().
		GetPronunciation(gomock.Any(), subject).
		Times(1).
		Return(expectedCardContent.Pronunciation, nil)
}

func cambridgeExtractorExpectedCallsForReturnVolumes(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&extractor.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedCardContent.Pronunciation,
			Transcription: expectedCardContent.Transcription,
			Examples:      expectedCardContent.Examples,
			Explains:      []string{expectedCardContent.Paraphrase},
		}, nil)
}

func cambridgeExtractorExpectedCallsReturnVolumesExceptExamples(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&extractor.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedCardContent.Pronunciation,
			Transcription: expectedCardContent.Transcription,
			Explains:      []string{expectedCardContent.Paraphrase},
		}, nil)
}

func cambridgeExtractorExpectedCallsEveryTimeReturnErr(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
	ce.EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*extractor.CambridgeCard, error) {
			return nil, errors.New("error from cambridge card extractor")
		})
}

func cambridgeExtractorWithoutExpectedCalls(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
}

func cambridgeExtractorExpectOneOrNoneGetPronunciationCall(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge) {
	ce.
		EXPECT().
		GetPronunciation(gomock.Any(), subject).
		AnyTimes()
}

func llmProviderExpectedCallsGenerateCardContent(subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		Times(1).
		Return(&llm.GeneratedCardContent{
			Paraphrase:    expectedCardContent.Paraphrase,
			Transcription: expectedCardContent.Transcription,
			Examples:      expectedCardContent.Examples,
			Synonyms:      expectedCardContent.Synonyms,
		}, nil)
}

func llmProviderExpectedCallsReturnErr(subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		Times(1).
		Return(nil, errors.New("error from llm provider"))
}

func llmProviderExpectedOneOrNoneCall(subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		AnyTimes()
}
