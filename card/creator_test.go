package card

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	mock_fetcher "github.com/shredd0r/anki-card-creator/card/fetcher/mock"
	"github.com/shredd0r/anki-card-creator/config"
	mock_google "github.com/shredd0r/anki-card-creator/google/mock"
	mock_llm "github.com/shredd0r/anki-card-creator/llm/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/shredd0r/anki-card-creator/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type funcForCreateCardContentComponentFether func(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetcher.MockCardContent
type llmProviderExpectedCalls func(subject string, usingContext *[]string, cfg config.PictureConfig, gp *mock_llm.MockProvider)
type googleImageProviderExpectedCalls func(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_google.MockImage)
type cardContentFetcherFactoryExpectedCalls func(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory)

type positiveCase struct {
	Name                                   string
	UsingContextForFlashcard               *[]string
	ExpectedFlashcard                      *models.Flashcard
	llmProviderExpectedCalls               llmProviderExpectedCalls
	googleImageProviderExpectedCalls       googleImageProviderExpectedCalls
	cardContentFetcherFactoryExpectedCalls cardContentFetcherFactoryExpectedCalls
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
			llmProviderExpectedCalls:               llmProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls:       googleImageProviderExpectedCallsForGetOnePicture,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
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
			llmProviderExpectedCalls:               llmProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:       googleImageProviderWithoutExpectdCalls,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
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
			llmProviderExpectedCalls:               llmProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:       googleImageProviderWithoutExpectdCalls,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
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
			llmProviderExpectedCalls:               llmProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed,
			googleImageProviderExpectedCalls:       googleImageProviderExpectedCallsWhereUseAllAttempts,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults,
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
			gp := mock_llm.NewMockProvider(ctrl)
			gip := mock_google.NewMockImage(ctrl)
			cccf := mock_fetcher.NewMockCardContentFactory(ctrl)

			testcase.llmProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard, cfg, gp)
			testcase.googleImageProviderExpectedCalls(testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard, cfg, ctrl, gip)
			testcase.cardContentFetcherFactoryExpectedCalls(testcase.ExpectedFlashcard, testcase.UsingContextForFlashcard, ctrl, cccf)

			c := NewFlashcardCreator(cfg, logger, gp, gip, cccf)

			actualFlashcard, err := c.Create(t.Context(), testcase.ExpectedFlashcard.DeckName, testcase.ExpectedFlashcard.Subject, testcase.UsingContextForFlashcard)
			assert.Nil(t, err)

			assert.Equal(t, testcase.ExpectedFlashcard, actualFlashcard)
		})
	}
}

type negativeCase struct {
	Name                                   string
	Subject                                string
	ExpectedErrMessage                     string
	llmProviderExpectedCalls               llmProviderExpectedCalls
	googleImageProviderExpectedCalls       googleImageProviderExpectedCalls
	cardContentFetcherFactoryExpectedCalls cardContentFetcherFactoryExpectedCalls
}

func TestNegativeCases(t *testing.T) {
	usingContexForTest := &[]string{"using-context"}
	testcases := []negativeCase{
		{
			Name:                                   "error from get card content method",
			Subject:                                "err-card-content",
			ExpectedErrMessage:                     "error from card content fetcher",
			llmProviderExpectedCalls:               llmProviderExpectedCallsForRatingPicture,
			googleImageProviderExpectedCalls:       googleImageProviderExpectedCallsForGetOnePicture,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetCardContentMethodReturnErr,
		},
		{
			Name:                                   "unsupported subjectType",
			Subject:                                "test-subject",
			ExpectedErrMessage:                     "unsupported subject type",
			llmProviderExpectedCalls:               llmProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:       googleImageProviderWithoutExpectdCalls,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr,
		},
		{
			Name:                                   "llm provider return err",
			Subject:                                "test-subject",
			ExpectedErrMessage:                     "request limit reached",
			llmProviderExpectedCalls:               llmProviderExpectedCallsWhereReturnErr,
			googleImageProviderExpectedCalls:       googleImageProviderExpectedCallsForGetOnePicture,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
		},
		{
			Name:                                   "google image provider return err",
			Subject:                                "test-subject",
			ExpectedErrMessage:                     "index out of range",
			llmProviderExpectedCalls:               llmProviderWithoutExpectdCalls,
			googleImageProviderExpectedCalls:       googleImageProviderExpectedCallsWhereReturnErr,
			cardContentFetcherFactoryExpectedCalls: fetcherFactoryExpectedCallsWhereGetPictureReturnErr,
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
			gp := mock_llm.NewMockProvider(ctrl)
			gip := mock_google.NewMockImage(ctrl)
			cccf := mock_fetcher.NewMockCardContentFactory(ctrl)

			testcase.llmProviderExpectedCalls(testcase.Subject, usingContexForTest, cfg, gp)
			testcase.googleImageProviderExpectedCalls(testcase.Subject, usingContexForTest, cfg, ctrl, gip)
			testcase.cardContentFetcherFactoryExpectedCalls(&models.Flashcard{
				Subject:     testcase.Subject,
				SubjectType: utils.GetSubjectType(testcase.Subject),
			},
				usingContexForTest,
				ctrl, cccf)

			c := NewFlashcardCreator(cfg, logger, gp, gip, cccf)

			flashcard, err := c.Create(t.Context(), "test-deckname", testcase.Subject, usingContexForTest)
			assert.Nil(t, flashcard)
			assert.EqualError(t, err, testcase.ExpectedErrMessage)
		})
	}
}

func llmProviderExpectedCallsForRatingPicture(subject string, usingContext *[]string, cfg config.PictureConfig, llmp *mock_llm.MockProvider) {
	llmp.EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(&cfg.MinimalRating, nil)
}

func llmProviderWithoutExpectdCalls(subject string, usingContext *[]string, cfg config.PictureConfig, llmp *mock_llm.MockProvider) {
}

func llmProviderExpectedCallsWhereAllPictureHaveRatingLessThanNeed(subject string, usingContext *[]string, cfg config.PictureConfig, llmp *mock_llm.MockProvider) {
	rating := cfg.MinimalRating - 1
	llmp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(int(cfg.NumberOfAttemptRatingPicture)).
		Return(&rating, nil)
}

func llmProviderExpectedCallsWhereReturnErr(subject string, usingContext *[]string, cfg config.PictureConfig, llmp *mock_llm.MockProvider) {
	llmp.
		EXPECT().
		RatingPicture(gomock.Any(), subject, gomock.Any()).
		Times(1).
		Return(nil, errors.New("request limit reached"))
}

func googleImageProviderExpectedCallsForGetOnePicture(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_google.MockImage) {
	qgip := mock_google.NewMockResult(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Return(&models.File{}, nil)

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)
}

func googleImageProviderWithoutExpectdCalls(subject string, usingContex *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_google.MockImage) {
}

func googleImageProviderExpectedCallNewQuery(subject string, usingContext *[]string, gip *mock_google.MockImage, qgip *mock_google.MockResult) {
	query := subject
	if usingContext != nil {
		query += " " + strings.Join(*usingContext, ", ")
	}

	gip.
		EXPECT().
		Request(gomock.Any(), query).
		Return(qgip, nil)
}

func googleImageProviderExpectedCallsWhereUseAllAttempts(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_google.MockImage) {
	qgip := mock_google.NewMockResult(ctrl)

	for attempt := range cfg.NumberOfAttemptRatingPicture {
		qgip.
			EXPECT().
			Get(gomock.Any(), attempt).
			Times(1).
			Return(&models.File{}, nil)
	}

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)
}

func googleImageProviderExpectedCallsWhereReturnErr(subject string, usingContext *[]string, cfg config.PictureConfig, ctrl *gomock.Controller, gip *mock_google.MockImage) {
	qgip := mock_google.NewMockResult(ctrl)

	qgip.
		EXPECT().
		Get(gomock.Any(), uint(0)).
		Times(1).
		Return(nil, errors.New("index out of range"))

	googleImageProviderExpectedCallNewQuery(subject, usingContext, gip, qgip)

}

func fetcherFactoryExpectedCallsWhereFetchersReturnCorrectResults(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory) {
	fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard, usingContext, ctrl, cccf, mockCardContentComponentFetchersForFlashcard)
}

func fetcherFactoryExpectedCallsWhereGetCardContentMethodReturnErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory) {
	fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard, usingContext, ctrl, cccf, mockCardContentComponentFetchersWithErr)
}

func fetcherFactoryExpectedCallsWhereFetchersReturnBy(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory, funcForCreateFetherForFlashcard funcForCreateCardContentComponentFether) {
	mockCardContentComponent := funcForCreateFetherForFlashcard(flashcard, usingContext, ctrl)

	cccf.
		EXPECT().
		Get(models.SubjectType(flashcard.SubjectType)).
		Times(1).
		Return(mockCardContentComponent, nil)
}

func fetcherFactoryExpectedCallsWhereReturnUnsupportedSubjectTypeErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory) {
	cccf.
		EXPECT().
		Get(gomock.Any()).
		Times(1).
		Return(nil, errors.New("unsupported subject type"))
}

// This method set in factory cardContentMethod which expect any times to call all method of cardContentMethod
func fetcherFactoryExpectedCallsWhereGetPictureReturnErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller, cccf *mock_fetcher.MockCardContentFactory) {
	cardContentComponent := mock_fetcher.NewMockCardContent(ctrl)

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

func mockCardContentComponentFetchersForFlashcard(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetcher.MockCardContent {
	cardContentComponent := mock_fetcher.NewMockCardContent(ctrl)

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

func mockCardContentComponentFetchersWithErr(flashcard *models.Flashcard, usingContext *[]string, ctrl *gomock.Controller) *mock_fetcher.MockCardContent {
	cardContentComponent := mock_fetcher.NewMockCardContent(ctrl)

	cardContentComponent.
		EXPECT().
		GetSubjectType().
		Times(1).
		Return(models.SubjectType(flashcard.SubjectType))

	cardContentComponent.
		EXPECT().
		GetCardContent(gomock.Any(), flashcard.Subject, usingContext).
		Times(1).
		Return(nil, errors.New("error from card content fetcher"))

	return cardContentComponent
}
