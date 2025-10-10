package card

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/shredd0r/anki-card-creator/card/fetchers"
	mock_fetchers "github.com/shredd0r/anki-card-creator/card/fetchers/mock"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	mock_providers "github.com/shredd0r/anki-card-creator/providers/mock"
	"github.com/shredd0r/anki-card-creator/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type geminiProviderExpectedCalls func(subject string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider)
type googleImageProviderExpectedCalls func(subject string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider)
type fieldComponentFetcherFactoryExpectedCalls func(subject string, subjectType models.SubjectType, ctrl *gomock.Controller, fcff *mock_fetchers.MockFieldComponentFetcherFactory)

type positiveCase struct {
	Name                                      string
	ExpectedFlashcard                         *models.Flashcard
	geminiProviderExpectedCalls               geminiProviderExpectedCalls
	googleImageProviderExpectedCalls          googleImageProviderExpectedCalls
	fieldComponentFetcherFactoryExpectedCalls fieldComponentFetcherFactoryExpectedCalls
}

func TestPositiveCases(t *testing.T) {
	transcription := "word-transcription"
	testcases := []positiveCase{
		{
			Name: "create new flashcard for word",
			ExpectedFlashcard: &models.Flashcard{
				Subject:       "word",
				SubjectType:   models.SubjectTypeWord,
				DeckName:      "test-deck",
				Examples:      []string{"word-example"},
				Explain:       "word-explain",
				Transcription: &transcription,
				Pronunciation: &models.File{},
				Picture:       &models.File{},
			},
			geminiProviderExpectedCalls:               geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls:          googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name: "create new flashcard for phrase",
			ExpectedFlashcard: &models.Flashcard{
				Subject:     "subject for phrase",
				SubjectType: models.SubjectTypePhrase,
				DeckName:    "test-deck",
				Examples:    []string{"phrase-example"},
				Explain:     "phrase-explain",
			},
			geminiProviderExpectedCalls:               geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:          googleImageProviderWithoutExpectdCalls,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name: "not found matched image",
			ExpectedFlashcard: &models.Flashcard{
				Subject:       "notmatchedsubject",
				SubjectType:   models.SubjectTypeWord,
				DeckName:      "not-matched-deck",
				Examples:      []string{"word-example"},
				Explain:       "word-explain",
				Transcription: &transcription,
				Pronunciation: &models.File{},
				Picture:       nil,
			},
			geminiProviderExpectedCalls:               geminiProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed,
			googleImageProviderExpectedCalls:          googleImageProviderExpectedCallsWhereUseAllAttempts,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
	}

	logger := slog.Default()
	ctrl := gomock.NewController(t)

	cfg := config.PictureConfig{
		NumberOfAttemptRatingPicture: 3,
		MinimalRating:                7,
	}
	wg := sync.WaitGroup{}
	for _, testcase := range testcases {
		wg.Add(1)
		t.Run(testcase.Name, func(t *testing.T) {
			go func() {
				defer wg.Done()
				gp := mock_providers.NewMockGeminiProvider(ctrl)
				gip := mock_providers.NewMockGoogleImageProvider(ctrl)
				fcff := mock_fetchers.NewMockFieldComponentFetcherFactory(ctrl)

				testcase.geminiProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, cfg, gp)
				testcase.googleImageProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, cfg, ctrl, gip)
				testcase.fieldComponentFetcherFactoryExpectedCalls(testcase.ExpectedFlashcard.Subject, testcase.ExpectedFlashcard.SubjectType, ctrl, fcff)

				c := NewFlashcardCreator(cfg, logger, gp, gip, fcff)

				actualFlashcard, err := c.Create(t.Context(), testcase.ExpectedFlashcard.Subject, testcase.ExpectedFlashcard.DeckName)
				assert.Nil(t, err)

				assert.Equal(t, *testcase.ExpectedFlashcard, *actualFlashcard)
			}()
		})
	}
	wg.Wait()
}

type negativeCase struct {
	Name                                      string
	Subject                                   string
	ExpectedErrMessage                        string
	geminiProviderExpectedCalls               geminiProviderExpectedCalls
	googleImageProviderExpectedCalls          googleImageProviderExpectedCalls
	fieldComponentFetcherFactoryExpectedCalls fieldComponentFetcherFactoryExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	testcases := []negativeCase{
		{
			Name:                             "error from get transcription method",
			Subject:                          "transcription",
			ExpectedErrMessage:               "error from get transcription method",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                             "error from get explain method",
			Subject:                          "explain",
			ExpectedErrMessage:               "error from get explain method",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                             "error from get examples method",
			Subject:                          "examples",
			ExpectedErrMessage:               "error from get examples method",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                             "error from get pronunciation method",
			Subject:                          "pronunciation",
			ExpectedErrMessage:               "error from get pronunciation method",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                             "unsupported subjectType",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "unsupported subject type",
			geminiProviderExpectedCalls:      geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls: googleImageProviderWithoutExpectdCalls,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr,
		},
		{
			Name:                             "gemini provider return err",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "quota for requests is over",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsWhereReturnErr,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
		},
		{
			Name:                             "google image provider return err",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "index out of range",
			geminiProviderExpectedCalls:      geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsWhereReturnErr,
			fieldComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
		},
	}

	logger := slog.Default()
	ctrl := gomock.NewController(t)

	cfg := config.PictureConfig{
		NumberOfAttemptRatingPicture: 3,
		MinimalRating:                7,
	}

	wg := sync.WaitGroup{}
	for _, testcase := range testcases {
		wg.Add(1)
		t.Run(testcase.Name, func(t *testing.T) {
			go func() {
				defer wg.Done()
				gp := mock_providers.NewMockGeminiProvider(ctrl)
				gip := mock_providers.NewMockGoogleImageProvider(ctrl)
				fcff := mock_fetchers.NewMockFieldComponentFetcherFactory(ctrl)

				testcase.geminiProviderExpectedCalls(testcase.Subject, cfg, gp)
				testcase.googleImageProviderExpectedCalls(testcase.Subject, cfg, ctrl, gip)
				testcase.fieldComponentFetcherFactoryExpectedCalls(testcase.Subject, utils.GetSubjectType(testcase.Subject), ctrl, fcff)

				c := NewFlashcardCreator(cfg, logger, gp, gip, fcff)

				flashcard, err := c.Create(t.Context(), testcase.Subject, "test-deckname")
				assert.Nil(t, flashcard)
				assert.EqualError(t, err, testcase.ExpectedErrMessage)
			}()
		})
	}

	wg.Wait()
}

func geminiProviderExpectedCallsForRatingPicture(subject string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	gp.EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(&cfg.MinimalRating, nil)
}

func geminiProviderWithoutExpectdCalls(subject string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
}

func geminiProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed(subject string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	rating := cfg.MinimalRating - 1
	gp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(int(cfg.NumberOfAttemptRatingPicture)).
		Return(&rating, nil)
}

func geminiProviderExpectedCallsWhereReturnErr(subject string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(nil, errors.New("quota for requests is over"))
}

func googleImageProviderExpectedCallsForGetOnePicture(subject string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Return(&models.File{}, nil)

	gip.
		EXPECT().
		NewQuery(gomock.Any(), subject).
		Return(qgip, nil)
}

func googleImageProviderWithoutExpectdCalls(subject string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {

}

func googleImageProviderExpectedCallsWhereUseAllAttempts(subject string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	for attempt := range cfg.NumberOfAttemptRatingPicture {
		qgip.
			EXPECT().
			Get(gomock.Any(), attempt).
			Times(1).
			Return(&models.File{}, nil)
	}

	gip.
		EXPECT().
		NewQuery(gomock.Any(), subject).
		Return(qgip, nil)
}

func googleImageProviderExpectedCallsWhereReturnErr(subject string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Times(1).
		Return(nil, errors.New("index out of range"))

	gip.
		EXPECT().
		NewQuery(gomock.Any(), subject).
		Return(qgip, nil)
}

func fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults(subject string, subjectType models.SubjectType, ctrl *gomock.Controller, fcff *mock_fetchers.MockFieldComponentFetcherFactory) {
	var mockFieldComponent fetchers.FieldComponentFetcher
	if subjectType == models.SubjectTypeWord {
		mockFieldComponent = mockFieldComponentFetchersForWord(subject, ctrl)
	} else {
		mockFieldComponent = mockFieldComponentFetchersForPhrase(subject, ctrl)
	}

	fcff.
		EXPECT().
		Get(models.SubjectType(subjectType)).
		Times(1).
		Return(mockFieldComponent, nil)

}

func fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr(subject string, subjectType models.SubjectType, ctrl *gomock.Controller, fcff *mock_fetchers.MockFieldComponentFetcherFactory) {
	fcff.
		EXPECT().
		Get(gomock.Any()).
		Times(1).
		Return(nil, errors.New("unsupported subject type"))
}

// This method set in factory fieldComponentMethod which expect any times to call all method of fieldComponentMethod
func fetcherFactoryExpectedCallsWhereGetPictureReturnErr(subject string, subjectType models.SubjectType, ctrl *gomock.Controller, fcff *mock_fetchers.MockFieldComponentFetcherFactory) {
	fieldComponent := mock_fetchers.NewMockFieldComponentFetcher(ctrl)

	fieldComponent.
		EXPECT().
		GetExamples(gomock.Any(), subject).
		AnyTimes()

	fieldComponent.
		EXPECT().
		GetExplain(gomock.Any(), subject).
		AnyTimes()

	fieldComponent.
		EXPECT().
		GetPronunciation(gomock.Any(), subject).
		AnyTimes()

	fieldComponent.
		EXPECT().
		GetSubjectType().
		AnyTimes()

	fieldComponent.
		EXPECT().
		GetTranscription(gomock.Any(), subject).
		AnyTimes()

	fcff.
		EXPECT().
		Get(models.SubjectType(subjectType)).
		Times(1).
		Return(fieldComponent, nil)
}

// For negative cases, was added checking.
// If subject names same as one of flashcard field, fetcher method for getting this volume has to return error
func mockFieldComponentFetchersForWord(subject string, ctrl *gomock.Controller) *mock_fetchers.MockFieldComponentFetcher {
	fieldComponent := mock_fetchers.NewMockFieldComponentFetcher(ctrl)

	fieldComponent.
		EXPECT().
		GetSubjectType().
		Times(1).
		Return(models.SubjectType(models.SubjectTypeWord))

	transcription := "word-transcription"
	fieldComponent.
		EXPECT().
		GetTranscription(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*string, error) {
			if subject == "transcription" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get transcription method")
			}
			return &transcription, nil
		})

	explain := "word-explain"
	fieldComponent.
		EXPECT().
		GetExplain(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*string, error) {
			if subject == "explain" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get explain method")
			}
			return &explain, nil
		})

	fieldComponent.
		EXPECT().
		GetExamples(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*[]string, error) {
			if subject == "examples" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get examples method")
			}
			return &[]string{"word-example"}, nil
		})

	fieldComponent.
		EXPECT().
		GetPronunciation(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*models.File, error) {
			if subject == "pronunciation" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get pronunciation method")
			}
			return &models.File{}, nil
		})

	return fieldComponent
}

// For negative cases, was added checking.
// If subject names same as one of flashcard field, fetcher method for getting this volume has to return error
func mockFieldComponentFetchersForPhrase(subject string, ctrl *gomock.Controller) *mock_fetchers.MockFieldComponentFetcher {
	fieldComponent := mock_fetchers.NewMockFieldComponentFetcher(ctrl)

	fieldComponent.
		EXPECT().
		GetSubjectType().
		Times(1).
		Return(models.SubjectType(models.SubjectTypePhrase))

	fieldComponent.
		EXPECT().
		GetTranscription(gomock.Any(), subject).
		Times(1).
		Return(nil, nil)

	explain := "phrase-explain"
	fieldComponent.
		EXPECT().
		GetExplain(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*string, error) {
			if subject == "explain" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get explain method")
			}
			return &explain, nil
		})

	fieldComponent.
		EXPECT().
		GetExamples(gomock.Any(), subject).
		Times(1).
		DoAndReturn(func(ctx context.Context, subject string) (*[]string, error) {
			if subject == "examples" {
				time.Sleep(time.Second * 2)
				return nil, errors.New("error from get examples method")
			}
			return &[]string{"phrase-example"}, nil
		})

	fieldComponent.
		EXPECT().
		GetPronunciation(gomock.Any(), subject).
		Times(1).
		Return(nil, nil)

	return fieldComponent
}
