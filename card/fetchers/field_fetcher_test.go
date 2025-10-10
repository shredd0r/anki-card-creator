package fetchers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"

	"github.com/shredd0r/anki-card-creator/extractors"
	mock_extractors "github.com/shredd0r/anki-card-creator/extractors/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/providers"
	mock_providers "github.com/shredd0r/anki-card-creator/providers/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type cambridgeExtractorExpectedCalls func(subject string, expectedFieldVolumes *expectedFieldVolumes, ce *mock_extractors.MockCambridgeCardExtractor)
type geminiProviderExpectedCalls func(subject string, expectedFieldVolumes *expectedFieldVolumes, gp *mock_providers.MockGeminiProvider)
type methodForInitFieldComponentFetcher func(*slog.Logger, extractors.CambridgeCardExtractor, providers.GeminiProvider) FieldComponentFetcher

type expectedFieldVolumes struct {
	SubjectType   models.SubjectType
	Transcription *string
	Pronunciation *models.File
	Explain       string
	Examples      []string
}

type fieldFetcherPositiveCase struct {
	Name                             string
	Subject                          string
	ExpectedFieldVolumes             expectedFieldVolumes
	InitFieldComponentFetcherForTest methodForInitFieldComponentFetcher
	CambridgeExtractorExpectedCalls  cambridgeExtractorExpectedCalls
	GeminiProviderExpectedCalls      geminiProviderExpectedCalls
}

func TestPostiveCases(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	transcriptionForWord := "word-transcription"
	testcases := []fieldFetcherPositiveCase{
		{
			Name:    "field component fetcher for word",
			Subject: "subject-word",
			ExpectedFieldVolumes: expectedFieldVolumes{
				SubjectType:   models.SubjectTypeWord,
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Explain:       "word-explain",
				Examples:      []string{"word-example-1", "word-example-2"},
			},
			InitFieldComponentFetcherForTest: initFieldComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:  cambridgeCardExtractorExpectedCallsForReturnVolumes,
			GeminiProviderExpectedCalls:      geminiProviderWithoutExpectedCalls,
		},
		{
			Name:    "field component fetcher for word, where cambridge extractor didnt return examples",
			Subject: "subject-word",
			ExpectedFieldVolumes: expectedFieldVolumes{
				SubjectType:   models.SubjectTypeWord,
				Transcription: &transcriptionForWord,
				Pronunciation: &models.File{Content: []byte("word-content")},
				Explain:       "word-explain",
				Examples:      []string{"word-example-1", "word-example-2"},
			},
			InitFieldComponentFetcherForTest: initFieldComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:  cambridgeCardExtractorExpectedCallsReturnVolumesExceptExamples,
			GeminiProviderExpectedCalls:      geminiProviderExpectedCallExamplesWithReturn,
		},
		{
			Name:    "field component fetcher for phrase",
			Subject: "subject-phrase",
			ExpectedFieldVolumes: expectedFieldVolumes{
				SubjectType: models.SubjectTypePhrase,
				Explain:     "phrase-explain",
				Examples:    []string{"phrase-example-1", "phrase-example-2"},
			},
			InitFieldComponentFetcherForTest: initFieldComponentFetcherForPhrase,
			CambridgeExtractorExpectedCalls:  cambridgeCardExtractorWithoutExpectedCalls,
			GeminiProviderExpectedCalls:      geminiProviderExpectedCallsGenerateExamplesAndExplain,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractors.NewMockCambridgeCardExtractor(ctrl)
			gp := mock_providers.NewMockGeminiProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(testcase.Subject, &testcase.ExpectedFieldVolumes, ce)
			testcase.GeminiProviderExpectedCalls(testcase.Subject, &testcase.ExpectedFieldVolumes, gp)

			fieldComponentFetcher := testcase.InitFieldComponentFetcherForTest(logger, ce, gp)

			wg := sync.WaitGroup{}
			wg.Add(4)
			go func() {
				defer wg.Done()
				actualExamples, err := fieldComponentFetcher.GetExamples(t.Context(), testcase.Subject)
				assert.Nil(t, err)
				assert.Equal(t, testcase.ExpectedFieldVolumes.Examples, *actualExamples)
			}()

			go func() {
				defer wg.Done()
				actualExplain, err := fieldComponentFetcher.GetExplain(t.Context(), testcase.Subject)
				assert.Nil(t, err)
				assert.Equal(t, testcase.ExpectedFieldVolumes.Explain, *actualExplain)
			}()

			go func() {
				defer wg.Done()
				actualTranscription, err := fieldComponentFetcher.GetTranscription(t.Context(), testcase.Subject)
				assert.Nil(t, err)
				assert.Equal(t, testcase.ExpectedFieldVolumes.Transcription, actualTranscription)
			}()

			go func() {
				defer wg.Done()
				actualPronunciation, err := fieldComponentFetcher.GetPronunciation(t.Context(), testcase.Subject)
				assert.Nil(t, err)
				assert.Equal(t, testcase.ExpectedFieldVolumes.Pronunciation, actualPronunciation)
			}()

			wg.Wait()
			actualSubjectType := fieldComponentFetcher.GetSubjectType()
			assert.Equal(t, testcase.ExpectedFieldVolumes.SubjectType, actualSubjectType)
		})
	}
}

type fieldFetcherNegativeCase struct {
	Name                             string
	SubjectType                      models.SubjectType
	ExpectedExamplesErr              string
	ExpectedExplainErr               string
	ExpectedPronunciationErr         string
	ExpectedTranscriptionErr         string
	InitFieldComponentFetcherForTest methodForInitFieldComponentFetcher
	CambridgeExtractorExpectedCalls  cambridgeExtractorExpectedCalls
	GeminiProviderExpectedCalls      geminiProviderExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	testcases := []fieldFetcherNegativeCase{
		{
			Name:                             "field component fetcher for word, cambridge extractor return error",
			SubjectType:                      models.SubjectTypeWord,
			ExpectedExamplesErr:              "1st error from cambridge card extractor",
			ExpectedExplainErr:               "2st error from cambridge card extractor",
			ExpectedTranscriptionErr:         "3st error from cambridge card extractor",
			ExpectedPronunciationErr:         "4st error from cambridge card extractor",
			InitFieldComponentFetcherForTest: initFieldComponentFetcherForWord,
			CambridgeExtractorExpectedCalls:  cambridgeCardExtractorExpectedCallsEveryTimeReturnErr,
			GeminiProviderExpectedCalls:      geminiProviderWithoutExpectedCalls,
		},
		{
			Name:                             "field component fetcher for phrase, gemini provider return error",
			SubjectType:                      models.SubjectTypePhrase,
			ExpectedExamplesErr:              "error in method GetExamples",
			ExpectedExplainErr:               "error in method GetExplain",
			InitFieldComponentFetcherForTest: initFieldComponentFetcherForPhrase,
			CambridgeExtractorExpectedCalls:  cambridgeCardExtractorWithoutExpectedCalls,
			GeminiProviderExpectedCalls:      geminiProviderExpectedCallsReturnErr,
		},
	}

	subjectForTests := "test-subject"
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			ce := mock_extractors.NewMockCambridgeCardExtractor(ctrl)
			gp := mock_providers.NewMockGeminiProvider(ctrl)

			testcase.CambridgeExtractorExpectedCalls(subjectForTests, nil, ce)
			testcase.GeminiProviderExpectedCalls(subjectForTests, nil, gp)

			fieldComponentFetcher := testcase.InitFieldComponentFetcherForTest(logger, ce, gp)

			actualExamples, err := fieldComponentFetcher.GetExamples(t.Context(), subjectForTests)
			assert.Nil(t, actualExamples)
			assert.EqualError(t, err, testcase.ExpectedExamplesErr)

			actualExplain, err := fieldComponentFetcher.GetExplain(t.Context(), subjectForTests)
			assert.Nil(t, actualExplain)
			assert.EqualError(t, err, testcase.ExpectedExplainErr)

			// For phrase, this methods always return nil volume and nil error
			if testcase.SubjectType == models.SubjectTypeWord {
				actualTranscription, err := fieldComponentFetcher.GetTranscription(t.Context(), subjectForTests)
				assert.Nil(t, actualTranscription)
				assert.EqualError(t, err, testcase.ExpectedTranscriptionErr)

				actualPronunciation, err := fieldComponentFetcher.GetPronunciation(t.Context(), subjectForTests)
				assert.Nil(t, actualPronunciation)
				assert.EqualError(t, err, testcase.ExpectedPronunciationErr)
			}
		})
	}
}

func initFieldComponentFetcherForWord(logger *slog.Logger, ce extractors.CambridgeCardExtractor, gp providers.GeminiProvider) FieldComponentFetcher {
	return NewWordFieldComponentFetcher(logger, gp, ce)
}

func initFieldComponentFetcherForPhrase(logger *slog.Logger, ce extractors.CambridgeCardExtractor, gp providers.GeminiProvider) FieldComponentFetcher {
	return NewPhraseFieldComponentFetcher(logger, gp)
}

func cambridgeCardExtractorExpectedCallsForReturnVolumes(subject string, expectedFieldVolumes *expectedFieldVolumes, ce *mock_extractors.MockCambridgeCardExtractor) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&models.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedFieldVolumes.Pronunciation,
			Transcription: expectedFieldVolumes.Transcription,
			Examples:      expectedFieldVolumes.Examples,
			Explains:      []string{expectedFieldVolumes.Explain},
		}, nil)
}

func cambridgeCardExtractorExpectedCallsReturnVolumesExceptExamples(subject string, expectedFieldVolumes *expectedFieldVolumes, ce *mock_extractors.MockCambridgeCardExtractor) {
	ce.
		EXPECT().
		GetCard(gomock.Any(), subject).
		Times(1).
		Return(&models.CambridgeCard{
			Subject:       subject,
			Pronunciation: expectedFieldVolumes.Pronunciation,
			Transcription: expectedFieldVolumes.Transcription,
			Explains:      []string{expectedFieldVolumes.Explain},
		}, nil)
}

func cambridgeCardExtractorExpectedCallsEveryTimeReturnErr(subject string, expectedFieldVolumes *expectedFieldVolumes, ce *mock_extractors.MockCambridgeCardExtractor) {
	// Its methods:
	// - GetTranscription
	// - GetExplain
	// - GetExamples
	// - GetPronunciation
	countOfMethodForGetVolumesFromCambridgeCard := 4
	numOfcurrentErr := 0

	ce.EXPECT().
		GetCard(gomock.Any(), subject).
		Times(countOfMethodForGetVolumesFromCambridgeCard).
		DoAndReturn(func(ctx context.Context, subject string) (*models.CambridgeCard, error) {
			numOfcurrentErr++
			return nil, fmt.Errorf("%dst error from cambridge card extractor", numOfcurrentErr)
		})
}

func cambridgeCardExtractorWithoutExpectedCalls(subject string, expectedFieldVolumes *expectedFieldVolumes, ce *mock_extractors.MockCambridgeCardExtractor) {
}

func geminiProviderExpectedCallsGenerateExamplesAndExplain(subject string, expectedFieldVolumes *expectedFieldVolumes, gp *mock_providers.MockGeminiProvider) {
	geminiProviderExpectedCallExamplesWithReturn(subject, expectedFieldVolumes, gp)

	gp.
		EXPECT().
		GenerateExplain(gomock.Any(), subject).
		Times(1).
		Return(&expectedFieldVolumes.Explain, nil)
}

func geminiProviderExpectedCallExamplesWithReturn(subject string, expectedFieldVolumes *expectedFieldVolumes, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		GenerateExamples(gomock.Any(), subject).
		Times(1).
		Return(&expectedFieldVolumes.Examples, nil)
}

func geminiProviderExpectedCallsReturnErr(subject string, expectedFieldVolumes *expectedFieldVolumes, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		GenerateExamples(gomock.Any(), subject).
		Times(1).
		Return(nil, errors.New("error in method GetExamples"))

	gp.
		EXPECT().
		GenerateExplain(gomock.Any(), subject).
		Times(1).
		Return(nil, errors.New("error in method GetExplain"))
}

func geminiProviderWithoutExpectedCalls(subject string, expectedFieldVolumes *expectedFieldVolumes, gp *mock_providers.MockGeminiProvider) {

}
