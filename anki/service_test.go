package anki

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/atselvan/ankiconnect"
	"github.com/privatesquare/bkst-go-utils/utils/errors"
	mock_ankiconnect "github.com/shredd0r/anki-card-creator/anki/mock/ankiconnect"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Cases:

const (
	typeMIMEForPronunciation = "sound/mpeg"
	typeForPronunciation     = "mpeg"
	typeMIMEForPicture       = "image/jpeg"
	typeForPicture           = "jpeg"
)

type decksManagerExpectedCalls func(*models.Flashcard, *mock_ankiconnect.MockDecksManager)
type mediaManagerExpectedCalls func(*models.Flashcard, *mock_ankiconnect.MockMediaManager)
type notesManagerExpectedCalls func(*models.Flashcard, *mock_ankiconnect.MockNotesManager)
type modelManagerExpectedCalls func(string, *mock_ankiconnect.MockModelsManager)

type positiveCase struct {
	Name                string
	Flashcard           *models.Flashcard
	decksExpectedCalls  decksManagerExpectedCalls
	mediaExpectedCalls  mediaManagerExpectedCalls
	notesExpectedCalls  notesManagerExpectedCalls
	modelsExpectedCalls modelManagerExpectedCalls
}

type negativeCase struct {
	Name                string
	Flashcard           *models.Flashcard
	ExpectedErr         error
	decksExpectedCalls  decksManagerExpectedCalls
	mediaExpectedCalls  mediaManagerExpectedCalls
	notesExpectedCalls  notesManagerExpectedCalls
	modelsExpectedCalls modelManagerExpectedCalls
}

func TestPositiveCases(t *testing.T) {
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	testcases := []positiveCase{
		{
			Name:                "add new flashcard with word",
			Flashcard:           GetFlashcardForWord(),
			decksExpectedCalls:  decksManagerWhereExpectedCreateNew,
			mediaExpectedCalls:  mediaManagerExpectedCallsWhereCouldStoreToFiles,
			notesExpectedCalls:  notesManagerExpectedCallWhereAddedNewNote,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateExist,
		},
		{
			Name:                "add new flashcard with phrase",
			Flashcard:           GetFlashcardForPhrase(),
			decksExpectedCalls:  decksManagerWhereExpectedCreateNew,
			mediaExpectedCalls:  mediaManagerWithoutExpectedCalls,
			notesExpectedCalls:  notesManagerExpectedCallWhereAddedNewNote,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateExist,
		},
		{
			Name:                "add new flashcard where deck already exist",
			Flashcard:           GetFlashcardForWord(),
			decksExpectedCalls:  decksManagerWhereDeckAlreadyExist,
			mediaExpectedCalls:  mediaManagerExpectedCallsWhereCouldStoreToFiles,
			notesExpectedCalls:  notesManagerExpectedCallWhereAddedNewNote,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateExist,
		},
	}

	templateName := "test-templatename"

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			mdm := mock_ankiconnect.NewMockDecksManager(ctrl)
			mmm := mock_ankiconnect.NewMockMediaManager(ctrl)
			mnm := mock_ankiconnect.NewMockNotesManager(ctrl)
			mMm := mock_ankiconnect.NewMockModelsManager(ctrl)

			testcase.decksExpectedCalls(testcase.Flashcard, mdm)
			testcase.mediaExpectedCalls(testcase.Flashcard, mmm)
			testcase.notesExpectedCalls(testcase.Flashcard, mnm)
			testcase.modelsExpectedCalls(templateName, mMm)

			client := &ankiconnect.Client{
				Decks:  mdm,
				Media:  mmm,
				Notes:  mnm,
				Models: mMm,
			}

			ankiService := NewService(logger, client)

			err := ankiService.StoreNewCard(t.Context(), templateName, testcase.Flashcard)
			assert.Nil(t, err)
		})
	}
}

func TestNegativeCases(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	flashcardForCaseIssueInMimeType := GetFlashcardForWord()
	flashcardForCaseIssueInMimeType.Pronunciation.MIMEType = "incorrect"
	flashcardForCaseIssueInMimeType.Picture.MIMEType = "incorrect"

	testcases := []negativeCase{
		{
			Name:                "flashcard already exist",
			ExpectedErr:         errCardAlreadyExist,
			Flashcard:           GetFlashcardForWord(),
			decksExpectedCalls:  decksManagerWithoutExpectedCalls,
			mediaExpectedCalls:  mediaManagerWithoutExpectedCalls,
			notesExpectedCalls:  notesManagerWhereAlreadyExist,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateExist,
		},
		{
			Name:                "template not exist",
			ExpectedErr:         errTemplateNameNotExist,
			Flashcard:           GetFlashcardForWord(),
			decksExpectedCalls:  decksManagerWithoutExpectedCalls,
			mediaExpectedCalls:  mediaManagerWithoutExpectedCalls,
			notesExpectedCalls:  notesManagerWithoutExpectedCalls,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateNotExist,
		},
		{
			// Not sure, that this case can be reproduced
			Name:                "file has issue in mimetype",
			ExpectedErr:         errMoreThanOneType,
			Flashcard:           flashcardForCaseIssueInMimeType,
			decksExpectedCalls:  decksManagerWhereDeckAlreadyExist,
			mediaExpectedCalls:  mediaManagerWithoutExpectedCalls,
			notesExpectedCalls:  notesManagerExpectedCallsWhereCheckCardIsExist,
			modelsExpectedCalls: modelsManagerExpectedCallsWhereTemplateExist,
		},
	}

	templateName := "test-templatename"

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			mdm := mock_ankiconnect.NewMockDecksManager(ctrl)
			mmm := mock_ankiconnect.NewMockMediaManager(ctrl)
			mnm := mock_ankiconnect.NewMockNotesManager(ctrl)
			mMm := mock_ankiconnect.NewMockModelsManager(ctrl)

			testcase.decksExpectedCalls(testcase.Flashcard, mdm)
			testcase.mediaExpectedCalls(testcase.Flashcard, mmm)
			testcase.notesExpectedCalls(testcase.Flashcard, mnm)
			testcase.modelsExpectedCalls(templateName, mMm)

			client := &ankiconnect.Client{
				Decks:  mdm,
				Media:  mmm,
				Notes:  mnm,
				Models: mMm,
			}

			ankiService := NewService(logger, client)

			err := ankiService.StoreNewCard(t.Context(), templateName, testcase.Flashcard)
			assert.NotNil(t, err)
			assert.ErrorIs(t, err, testcase.ExpectedErr)
		})
	}
}
func GetFlashcardForWord() *models.Flashcard {
	transcription := "test-transcription"
	return &models.Flashcard{
		Subject:     "word",
		SubjectType: models.SubjectTypeWord,
		DeckName:    "test-deck-name",
		Pronunciation: &models.File{
			Content:  []byte("pronunciation-content"),
			MIMEType: typeMIMEForPronunciation,
		},
		Transcription: &transcription,
		Paraphrase:    "test-paraphrase",
		Picture: &models.File{
			Content:  []byte("picture-content"),
			MIMEType: typeMIMEForPicture,
		},
		Examples: []string{"text-example"},
	}
}

func GetFlashcardForPhrase() *models.Flashcard {
	return &models.Flashcard{
		Subject:     "Test phrase",
		SubjectType: models.SubjectTypePhrase,
		DeckName:    "test-deck-name",
		Paraphrase:  "test-paraphrase",
		Examples:    []string{"test-example"},
	}
}

func decksManagerWhereExpectedCreateNew(flashcard *models.Flashcard, mdm *mock_ankiconnect.MockDecksManager) {
	mdm.
		EXPECT().
		GetAll().
		Times(1).
		Return(&[]string{}, nil)
	mdm.
		EXPECT().
		Create(flashcard.DeckName).
		Times(1).
		Return(nil)
}

func decksManagerWhereDeckAlreadyExist(flashcard *models.Flashcard, mdm *mock_ankiconnect.MockDecksManager) {
	mdm.
		EXPECT().
		GetAll().
		Times(1).
		Return(&[]string{flashcard.DeckName}, nil)
}

func decksManagerWithoutExpectedCalls(f *models.Flashcard, mdm *mock_ankiconnect.MockDecksManager) {}

func mediaManagerExpectedCallsWhereCouldStoreToFiles(flashcard *models.Flashcard, mmm *mock_ankiconnect.MockMediaManager) {
	filenameForPronunciation := getFilename(flashcard, typeForPronunciation)
	filenameForPicture := getFilename(flashcard, typeForPicture)
	encodedPronunciationContent := base64.StdEncoding.EncodeToString(flashcard.Pronunciation.Content)
	encodedPictureContent := base64.StdEncoding.EncodeToString(flashcard.Picture.Content)

	mmm.
		EXPECT().
		StoreMediaFile(filenameForPronunciation, encodedPronunciationContent).
		Times(1).
		Return(&encodedPronunciationContent, nil)

	mmm.
		EXPECT().
		StoreMediaFile(filenameForPicture, encodedPictureContent).
		Times(1).
		Return(&encodedPictureContent, nil)
}

func notesManagerExpectedCallsWhereCheckCardIsExist(flashcard *models.Flashcard, mnm *mock_ankiconnect.MockNotesManager) {
	expectedQuery := fmt.Sprintf(`"subject:%s" "deck:%s"`, flashcard.Subject, flashcard.DeckName)

	mnm.
		EXPECT().
		Get(expectedQuery).
		Times(1).
		Return(&[]ankiconnect.ResultNotesInfo{}, nil)
}

func notesManagerExpectedCallWhereAddedNewNote(flashcard *models.Flashcard, mnm *mock_ankiconnect.MockNotesManager) {
	fields := map[string]string{
		"Subject":      flashcard.Subject,
		"Paraphrase":   flashcard.Paraphrase,
		"Example":      fmt.Sprintf("<ul><li>%s</li></ul>", flashcard.Examples[0]),
		"Has_Spelling": "1",
	}
	if flashcard.Pronunciation != nil {
		fields["Pronunciation"] = getFilename(flashcard, typeForPronunciation)
	}

	if flashcard.Picture != nil {
		fields["Picture"] = putFileNameToImgTag(getFilename(flashcard, typeForPicture))
	}
	if flashcard.Transcription != nil {
		fields["Transcription"] = *flashcard.Transcription
	}

	expectedNote := ankiconnect.Note{
		DeckName:  flashcard.DeckName,
		ModelName: "test-templatename",
		Fields:    fields,
	}
	notesManagerExpectedCallsWhereCheckCardIsExist(flashcard, mnm)

	mnm.
		EXPECT().
		Add(expectedNote).
		Times(1).
		Return(nil)
}

func notesManagerWhereAlreadyExist(flashcard *models.Flashcard, mnm *mock_ankiconnect.MockNotesManager) {
	mnm.
		EXPECT().
		Get(gomock.Any()).
		Times(1).
		Return(&[]ankiconnect.ResultNotesInfo{
			{
				Fields: map[string]ankiconnect.FieldData{
					"Subject": ankiconnect.FieldData{
						Value: flashcard.Subject,
					},
				},
			},
		}, nil)
}

func notesManagerWithoutExpectedCalls(flashcard *models.Flashcard, mnm *mock_ankiconnect.MockNotesManager) {
}

func mediaManagerWithoutExpectedCalls(flashcard *models.Flashcard, mmm *mock_ankiconnect.MockMediaManager) {
}

func modelsManagerExpectedCallsWhereTemplateExist(templateName string, mmm *mock_ankiconnect.MockModelsManager) {
	mmm.
		EXPECT().
		GetFields(templateName).
		Times(1).
		Return(&[]string{}, nil)
}

func modelsManagerExpectedCallsWhereTemplateNotExist(templateName string, mmm *mock_ankiconnect.MockModelsManager) {
	mmm.
		EXPECT().
		GetFields(templateName).
		Times(1).
		Return(nil, &errors.RestErr{Message: fmt.Sprintf("model was not found: %s", templateName)})
}

func getFilename(flashcard *models.Flashcard, typeOfFile string) string {
	return fmt.Sprintf("_%s.%s", strings.ToLower(strings.ReplaceAll(flashcard.Subject, " ", "-")), typeOfFile)
}

func putFileNameToImgTag(filename string) string {
	return fmt.Sprintf("<img src='%s'>", filename)
}
