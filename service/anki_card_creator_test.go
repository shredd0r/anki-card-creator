package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	mock_anki "github.com/shredd0r/anki-card-creator/anki/mock"
	mock_card "github.com/shredd0r/anki-card-creator/card/mock"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreate_ReturnsErrorWhenAddFlashcardFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	ankiService := mock_anki.NewMockService(ctrl)
	cardCreator := mock_card.NewMockCreator(ctrl)

	flashcard := &models.Flashcard{Subject: "lime", DeckName: "English"}
	cardCreator.EXPECT().Create(gomock.Any(), "English", "lime", gomock.Any()).Return(flashcard, nil)

	wantErr := errors.New("anki connect unreachable")
	ankiService.EXPECT().AddFlashcard(gomock.Any(), flashcard).Return(wantErr)

	c := NewAnkiCardCreator(config.Config{QueueSize: 1}, slog.Default(), ankiService, cardCreator)

	targets := &[]extractor.TargetInfo{
		{DeckName: "English", Subjects: []string{"lime"}},
	}

	err := c.Create(context.Background(), targets)
	require.ErrorIs(t, err, wantErr)
}

func TestCreate_SucceedsWhenAllFlashcardsAdded(t *testing.T) {
	ctrl := gomock.NewController(t)
	ankiService := mock_anki.NewMockService(ctrl)
	cardCreator := mock_card.NewMockCreator(ctrl)

	flashcard := &models.Flashcard{Subject: "lime", DeckName: "English"}
	cardCreator.EXPECT().Create(gomock.Any(), "English", "lime", gomock.Any()).Return(flashcard, nil)
	ankiService.EXPECT().AddFlashcard(gomock.Any(), flashcard).Return(nil)

	c := NewAnkiCardCreator(config.Config{QueueSize: 1}, slog.Default(), ankiService, cardCreator)

	targets := &[]extractor.TargetInfo{
		{DeckName: "English", Subjects: []string{"lime"}},
	}

	err := c.Create(context.Background(), targets)
	require.NoError(t, err)
}
