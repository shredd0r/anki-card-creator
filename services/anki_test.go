package services

import (
	"log/slog"
	"testing"

	"github.com/atselvan/ankiconnect"
	"github.com/shredd0r/anki-card-creator/models"
	mock_ankiconnect "github.com/shredd0r/anki-card-creator/services/mock/ankiconnect"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Cases:
// TestAddWordFlashcard
// TestAddPhraseFlashcard
// TestDeckAlreadyExist
// TestCardAlreadyExist (negative)
// TestTemplateNotExisst (negative)
// TestFileHasIssueInMIMEType (negative)

func Test_test(t *testing.T) {
	logger := slog.Default()
	ctrl := gomock.NewController(t)

	decks := mock_ankiconnect.NewMockDecksManager(ctrl)
	media := mock_ankiconnect.NewMockMediaManager(ctrl)
	notes := mock_ankiconnect.NewMockNotesManager(ctrl)

	client := &ankiconnect.Client{
		Decks: decks,
		Media: media,
		Notes: notes,
	}

	flashcard := GetFlashcard()

	decks.EXPECT().
		GetAll().
		Times(1).
		Return(&[]string{}, nil)

	decks.EXPECT().
		Create(flashcard.DeckName).
		Times(1).
		Return(nil)

	newFile := "new-file"
	media.EXPECT().
		StoreMediaFile(gomock.Any(), gomock.Any()).
		Times(2).
		Return(&newFile, nil)

	notes.EXPECT().
		Add(gomock.Any()).
		Times(1).
		Return(nil)

	ankiService := NewAnkiService(logger, client)

	err := ankiService.StoreNewCard(t.Context(), "test-templatename", flashcard)
	assert.Nil(t, err)
}

func GetFlashcard() *models.Flashcard {
	transcription := "test-transcription"
	return &models.Flashcard{
		Subject:       "subject",
		SubjectType:   models.SubjectTypeWord,
		DeckName:      "test-deckname",
		Pronunciation: &models.File{MIMEType: "mpeg/mp3"},
		Transcription: &transcription,
		Explain:       "test-explain",
		Picture:       &models.File{MIMEType: "image/jpeg"},
		Examples:      []string{"text-example"},
	}
}
