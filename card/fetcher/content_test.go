package fetcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	mock_extractor "github.com/shredd0r/anki-card-creator/extractor/mock"
	"github.com/shredd0r/anki-card-creator/google"
	mock_google "github.com/shredd0r/anki-card-creator/google/mock"
	"github.com/shredd0r/anki-card-creator/llm"
	mock_llm "github.com/shredd0r/anki-card-creator/llm/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type googleImageProviderExpectedCalls func(cfg config.PictureConfig, subject string, usingContext *[]string, gipmp *mock_google.MockImage)
type cambridgeExtractorExpectedCalls func(subject string, expectedCardContent *models.CardContent, ce *mock_extractor.MockCambridge)
type llmProviderExpectedCalls func(cfg config.PictureConfig, subject string, usingContext *[]string, expectedCardContent *models.CardContent, gp *mock_llm.MockProvider)
type methodForInitCardContentFetcher func(config.PictureConfig, *slog.Logger, google.Image, extractor.Cambridge, llm.Provider) CardContent

type fieldFetcherPositiveCase struct {
	Name                             string
	Subject                          string
	ExpectedSubjectType              models.SubjectType
	ExpectedCardContent              models.CardContent
	InitCardContentFetcherForTest    methodForInitCardContentFetcher
	GoogleImageProviderExpectedCalls googleImageProviderExpectedCalls
	CambridgeExtractorExpectedCalls  cambridgeExtractorExpectedCalls
	LLMProviderExpectedCalls         llmProviderExpectedCalls
}

// Add tests for checking rate picture
func TestPostiveCases(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	transcriptionForWord := "word-transcription"
	testcases := []fieldFetcherPositiveCase{
		{
			Name:                "field fetcher for word",
			Subject:             "subject-word",
			ExpectedSubjectType: models.SubjectTypeWord,
			ExpectedCardContent: models.CardContent{
				Paraphrase:    "word-paraphrase",
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Examples:      []string{"word-example-1", "word-example-2"},
				Synonyms:      []string{"word-synonym-1", "word-synonym-2"},
			},
			InitCardContentFetcherForTest:    initCardContentFetcherForWord,
			GoogleImageProviderExpectedCalls: googleImageProviderExpectedAllCalls,
			CambridgeExtractorExpectedCalls:  cambridgeExtractorExpectedCallsForReturnVolumes,
			LLMProviderExpectedCalls:         llmProviderExpectedCallsGenerateCardContent,
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
			InitCardContentFetcherForTest:   initCardContentFetcherForWord,
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
			cfg := config.PictureConfig{
				CountSearches: 5,
				MinimalRating: 5,
			}
			ce := mock_extractor.NewMockCambridge(ctrl)
			lp := mock_llm.NewMockProvider(ctrl)
			gip := mock_google.NewMockImage(ctrl)

			testcase.CambridgeExtractorExpectedCalls(testcase.Subject, &testcase.ExpectedCardContent, ce)
			testcase.LLMProviderExpectedCalls(cfg, testcase.Subject, usingContextForTest, &testcase.ExpectedCardContent, lp)

			cardContentComponentFetcher := testcase.InitCardContentFetcherForTest(cfg, logger, gip, ce, lp)

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
			InitCardContentFetcherForTest:   initCardContentFetcherForWord,
			CambridgeExtractorExpectedCalls: cambridgeExtractorExpectedCallsEveryTimeReturnErr,
			LLMProviderExpectedCalls:        llmProviderExpectedOneOrNoneCall,
		},
		{
			Name:                            "field component fetcher for word, llm provider return error",
			SubjectType:                     models.SubjectTypeWord,
			ExpectedMessageErr:              "error from llm provider",
			InitCardContentFetcherForTest:   initCardContentFetcherForWord,
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
			cfg := config.PictureConfig{
				CountSearches: 5,
				MinimalRating: 5,
			}
			ce := mock_extractor.NewMockCambridge(ctrl)
			lp := mock_llm.NewMockProvider(ctrl)
			gip := mock_google.NewMockImage(ctrl)

			testcase.CambridgeExtractorExpectedCalls(subjectForTests, nil, ce)
			testcase.LLMProviderExpectedCalls(cfg, subjectForTests, nil, nil, lp)

			cardContentComponentFetcher := testcase.InitCardContentFetcherForTest(cfg, logger, gip, ce, lp)

			actualCardContent, err := cardContentComponentFetcher.GetCardContent(t.Context(), subjectForTests, nil)
			assert.Nil(t, actualCardContent)
			assert.EqualError(t, err, testcase.ExpectedMessageErr)
		})
	}
}

func initCardContentFetcherForWord(cfg config.PictureConfig, logger *slog.Logger, gip google.Image, ce extractor.Cambridge, llmp llm.Provider) CardContent {
	return NewWordCardContent(cfg, logger, gip, llmp, ce)
}

func initCardContentFetcherForPhrase(cfg config.PictureConfig, logger *slog.Logger, gip google.Image, ce extractor.Cambridge, llmp llm.Provider) CardContent {
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

func llmProviderExpectedCallsGenerateCardContent(cfg config.PictureConfig, subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
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

	llmp.
		EXPECT().
		RatePicture(gomock.Any(), subject, gomock.Any()).
		MaxTimes(int(cfg.CountSearches))
}

func llmProviderExpectedCallsReturnErr(cfg config.PictureConfig, subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		Times(1).
		Return(nil, errors.New("error from llm provider"))
}

func llmProviderExpectedOneOrNoneCall(cfg config.PictureConfig, subject string, usingContext *[]string, expectedCardContent *models.CardContent, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		GenerateCardContent(gomock.Any(), subject, usingContext).
		AnyTimes()

	llmp.
		EXPECT().
		RatePicture(gomock.Any(), subject, gomock.Any()).
		AnyTimes()
}

func googleImageProviderExpectedAllCalls(cfg config.PictureConfig, subject string, usingContext *[]string, gipmp *mock_google.MockImage) {
	searchRequest := fmt.Sprintf("%s %s", subject, *usingContext)
	gipmp.
		EXPECT().
		Request(gomock.Any(), searchRequest).
		MaxTimes(int(cfg.CountSearches))
}
