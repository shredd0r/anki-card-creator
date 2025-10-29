package card

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	mock_fetchers "github.com/shredd0r/anki-card-creator/card/fetchers/mock"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	mock_providers "github.com/shredd0r/anki-card-creator/providers/mock"
	"github.com/shredd0r/anki-card-creator/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type funcForCreateCardContentComponentFether func(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetchers.MockCardContentComponentFetcher
type geminiProviderExpectedCalls func(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider)
type googleImageProviderExpectedCalls func(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider)
type cardContentComponentFetcherFactoryExpectedCalls func(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory)

type positiveCase struct {
	Name                                            string
	UsingContextForFlashcard                        *[]string
	ExpectedFlashcard                               *models.Flashcard
	geminiProviderExpectedCalls                     geminiProviderExpectedCalls
	googleImageProviderExpectedCalls                googleImageProviderExpectedCalls
	cardContentComponentFetcherFactoryExpectedCalls cardContentComponentFetcherFactoryExpectedCalls
}

func TestPositiveCases(t *testing.T) {
	transcription := "word-transcription"
	testcases := []positiveCase{
		{
			Name:                     "create new flashcard for word",
			UsingContextForFlashcard: &[]string{"context-word"},
			ExpectedFlashcard: &models.Flashcard{
				Subject:       "word",
				SubjectType:   models.SubjectTypeWord,
				DeckName:      "test-deck",
				Examples:      []string{"word-example"},
				Paraphrase:    "word-explain",
				Transcription: &transcription,
				Pronunciation: &models.File{},
				Picture:       &models.File{},
			},
			geminiProviderExpectedCalls:                     geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls:                googleImageProviderExpectedCallsForGetOnePicture,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                     "create new flashcard for phrase",
			UsingContextForFlashcard: &[]string{"context-phrase"},
			ExpectedFlashcard: &models.Flashcard{
				Subject:     "subject for phrase",
				SubjectType: models.SubjectTypePhrase,
				DeckName:    "test-deck",
				Examples:    []string{"phrase-example"},
				Paraphrase:  "phrase-explain",
			},
			geminiProviderExpectedCalls:                     geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:                googleImageProviderWithoutExpectdCalls,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                     "create new flashcard without using context",
			UsingContextForFlashcard: nil,
			ExpectedFlashcard: &models.Flashcard{
				Subject:     "subject for phrase",
				SubjectType: models.SubjectTypePhrase,
				DeckName:    "test-deck",
				Examples:    []string{"phrase-example"},
				Paraphrase:  "phrase-explain",
			},
			geminiProviderExpectedCalls:                     geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:                googleImageProviderWithoutExpectdCalls,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
		{
			Name:                     "not found matched image",
			UsingContextForFlashcard: &[]string{"context-without-image"},
			ExpectedFlashcard: &models.Flashcard{
				Subject:       "notmatchedsubject",
				SubjectType:   models.SubjectTypeWord,
				DeckName:      "not-matched-deck",
				Examples:      []string{"word-example"},
				Paraphrase:    "word-explain",
				Transcription: &transcription,
				Pronunciation: &models.File{},
				Picture:       nil,
			},
			geminiProviderExpectedCalls:                     geminiProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed,
			googleImageProviderExpectedCalls:                googleImageProviderExpectedCallsWhereUseAllAttempts,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
		},
	}

	logger := slog.Default()
	ctrl := gomock.NewController(t)

	cfg := config.PictureConfig{
		NumberOfAttemptRatingPicture: 3,
		MinimalRating:                7,
	}
	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			gp := mock_providers.NewMockGeminiProvider(ctrl)
			gip := mock_providers.NewMockGoogleImageProvider(ctrl)
			cccf := mock_fetchers.NewMockCardContentComponentFetcherFactory(ctrl)

			testcase.geminiProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard, cfg, gp)
			testcase.googleImageProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard, cfg, ctrl, gip)
			testcase.cardContentComponentFetcherFactoryExpectedCalls(testcase.ExpectedFlashcard, testcase.UsingContextForFlashcard, ctrl, cccf)

			c := NewFlashcardCreator(cfg, logger, gp, gip, cccf)

			actualFlashcard, err := c.Create(t.Context(), testcase.ExpectedFlashcard.DeckName, testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard)
			assert.Nil(t, err)

			assert.Equal(t, testcase.ExpectedFlashcard, actualFlashcard)
		})
	}
}

type negativeCase struct {
	Name                                            string
	Subject                                         string
	ExpectedErrMessage                              string
	geminiProviderExpectedCalls                     geminiProviderExpectedCalls
	googleImageProviderExpectedCalls                googleImageProviderExpectedCalls
	cardContentComponentFetcherFactoryExpectedCalls cardContentComponentFetcherFactoryExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	usingContexForTest := &[]string{"using-context"}
	testcases := []negativeCase{
		{
			Name:                             "error from get card content method",
			Subject:                          "err-card-content",
			ExpectedErrMessage:               "error from card component component fetcher",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetCardContentMethodReturnErr,
		},
		{
			Name:                             "unsupported subjectType",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "unsupported subject type",
			geminiProviderExpectedCalls:      geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls: googleImageProviderWithoutExpectdCalls,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr,
		},
		{
			Name:                             "gemini provider return err",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "quota for requests is over",
			geminiProviderExpectedCalls:      geminiProviderExpectedCallsWhereReturnErr,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsForGetOnePicture,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
		},
		{
			Name:                             "google image provider return err",
			Subject:                          "test-subject",
			ExpectedErrMessage:               "index out of range",
			geminiProviderExpectedCalls:      geminiProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls: googleImageProviderExpectedCallsWhereReturnErr,
			cardContentComponentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
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
				cccf := mock_fetchers.NewMockCardContentComponentFetcherFactory(ctrl)

				testcase.geminiProviderExpectedCalls(testcase.Subject, usingContexForTest, cfg, gp)
				testcase.googleImageProviderExpectedCalls(testcase.Subject, usingContexForTest, cfg, ctrl, gip)
				testcase.cardContentComponentFetcherFactoryExpectedCalls(&models.Flashcard{
					Subject:     testcase.Subject,
					SubjectType: utils.GetSubjectType(testcase.Subject),
				},
					usingContexForTest,
					ctrl, cccf)

				c := NewFlashcardCreator(cfg, logger, gp, gip, cccf)

				flashcard, err := c.Create(t.Context(), "test-deckname", testcase.Subject, usingContexForTest)
				assert.Nil(t, flashcard)
				assert.EqualError(t, err, testcase.ExpectedErrMessage)
			}()
		})
	}

	wg.Wait()
}

func geminiProviderExpectedCallsForRatingPicture(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	gp.EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(&cfg.MinimalRating, nil)
}

func geminiProviderWithoutExpectdCalls(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
}

func geminiProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	rating := cfg.MinimalRating - 1
	gp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(int(cfg.NumberOfAttemptRatingPicture)).
		Return(&rating, nil)
}

func geminiProviderExpectedCallsWhereReturnErr(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_providers.MockGeminiProvider) {
	gp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(nil, errors.New("quota for requests is over"))
}

func googleImageProviderExpectedCallsForGetOnePicture(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Return(&models.File{}, nil)

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)
}

func googleImageProviderWithoutExpectdCalls(subject string, usingContex *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
}

func googleImageProviderExpectedCallNewQuery(subject string, usingContext *[]string, gip *mock_providers.MockGoogleImageProvider, qgip *mock_providers.MockGoogleImageQueryProvider) {
	query := subject
	if usingContext != nil {
		query += " " + strings.Join(*usingContext, ", ")
	}

	gip.
		EXPECT().
		NewQuery(gomock.Any(), query).
		Return(qgip, nil)
}

func googleImageProviderExpectedCallsWhereUseAllAttempts(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	for attempt := range cfg.NumberOfAttemptRatingPicture {
		qgip.
			EXPECT().
			Get(gomock.Any(), attempt).
			Times(1).
			Return(&models.File{}, nil)
	}

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)
}

func googleImageProviderExpectedCallsWhereReturnErr(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_providers.MockGoogleImageProvider) {
	qgip := mock_providers.NewMockGoogleImageQueryProvider(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Times(1).
		Return(nil, errors.New("index out of range"))

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)

}

func fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory) {
	fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard, usingContext, ctrl, cccf, mockCardContentComponentFetchersForFlashcard)
}

func fetcherFactoryExpectedCallsWhereGetCardContentMethodReturnErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory) {
	fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard, usingContext, ctrl, cccf, mockCardContentComponentFetchersWithErr)
}

func fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory, funcForCreateFetherForFlashcard funcForCreateCardContentComponentFether) {
	mockCardContentComponent := funcForCreateFetherForFlashcard(flashcard, usingContext, ctrl)

	cccf.
		EXPECT().
		Get(models.SubjectType(flashcard.SubjectType)).
		Times(1).
		Return(mockCardContentComponent, nil)
}

func fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory) {
	cccf.
		EXPECT().
		Get(gomock.Any()).
		Times(1).
		Return(nil, errors.New("unsupported subject type"))
}

// This method set in factory cardContentComponentMethod which expect any times to call all method of cardContentComponentMethod
func fetcherFactoryExpectedCallsWhereGetPictureReturnErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetchers.MockCardContentComponentFetcherFactory) {
	cardContentComponent := mock_fetchers.NewMockCardContentComponentFetcher(ctrl)

	cardContentComponent.
		EXPECT().
		GetSubjectType().
		AnyTimes()

	cardContentComponent.
		EXPECT().
		GetCardContent(gomock.Any(), flashcard.Subject, usingContext).
		AnyTimes()

	cccf.
		EXPECT().
		Get(models.SubjectType(flashcard.SubjectType)).
		Times(1).
		Return(cardContentComponent, nil)
}

func mockCardContentComponentFetchersForFlashcard(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetchers.MockCardContentComponentFetcher {
	cardContentComponent := mock_fetchers.NewMockCardContentComponentFetcher(ctrl)

	cardContentComponent.
		EXPECT().
		GetSubjectType().
		Times(1).
		Return(models.SubjectType(flashcard.SubjectType))

	cardContentComponent.
		EXPECT().
		GetCardContent(gomock.Any(), flashcard.Subject, usingContext).
		Times(1).
		Return(&models.CardContent{
			Paraphrase:    flashcard.Paraphrase,
			Transcription: flashcard.Transcription,
			Pronunciation: flashcard.Pronunciation,
			Examples:      flashcard.Examples,
			Synonyms:      flashcard.Synonyms,
		}, nil)

	return cardContentComponent
}

func mockCardContentComponentFetchersWithErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetchers.MockCardContentComponentFetcher {
	cardContentComponent := mock_fetchers.NewMockCardContentComponentFetcher(ctrl)

	cardContentComponent.
		EXPECT().
		GetSubjectType().
		Times(1).
		Return(models.SubjectType(flashcard.SubjectType))

	cardContentComponent.
		EXPECT().
		GetCardContent(gomock.Any(), flashcard.Subject, usingContext).
		Times(1).
		Return(nil, errors.New("error from card component component fetcher"))

	return cardContentComponent
}
