package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

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

// Regression test for a real bug: card-content creation (fast, e.g. with a
// mock LLM) used to finish - and cancel the shared context as a side effect -
// well before the single-threaded AddFlashcard consumer had drained its
// backlog of real (slow) AnkiConnect calls, so later AddFlashcard calls
// failed with "context canceled" even though nothing actually went wrong.
func TestCreate_DoesNotCancelAddFlashcardWhenCreationFinishesFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	ankiService := mock_anki.NewMockService(ctrl)
	cardCreator := mock_card.NewMockCreator(ctrl)

	const subjectCount = 5
	subjects := make([]string, subjectCount)
	for i := range subjects {
		subjects[i] = string(rune('a' + i))
	}

	cardCreator.EXPECT().Create(gomock.Any(), "English", gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, deck, subject string, _ *[]string) (*models.Flashcard, error) {
			// Card creation is instant, so all subjects finish (and the
			// producer's wg.Wait() returns) well before the consumer below
			// has processed even one of them.
			return &models.Flashcard{Subject: subject, DeckName: deck}, nil
		}).Times(subjectCount)

	ankiService.EXPECT().AddFlashcard(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ *models.Flashcard) error {
			// Simulate a slow real network round-trip to AnkiConnect, and
			// assert the context handed to us hasn't been cancelled out from
			// under us just because creation elsewhere already finished.
			time.Sleep(5 * time.Millisecond)
			require.NoError(t, ctx.Err())
			return nil
		}).Times(subjectCount)

	c := NewAnkiCardCreator(config.Config{QueueSize: uint(subjectCount)}, slog.Default(), ankiService, cardCreator)

	targets := &[]extractor.TargetInfo{
		{DeckName: "English", Subjects: subjects},
	}

	err := c.Create(context.Background(), targets)
	require.NoError(t, err)
}
