package anki

import (
	"context"
	"log/slog"
	"testing"

	mock_anki "github.com/shredd0r/anki-card-creator/anki/mock"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*implService, *mock_anki.MockClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	client := mock_anki.NewMockClient(ctrl)

	return &implService{
		logger:    slog.Default(),
		client:    client,
		modelName: model_name,
		cfg:       config.AnkiConnectConfig{},
		decks:     map[string]struct{}{},
	}, client
}

func TestEnsureNoteType_CreatesModelWhenMissing(t *testing.T) {
	s, client := newTestService(t)
	ctx := context.Background()

	client.EXPECT().Invoke(ctx, "modelNames", nil, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]string)) = []string{"Basic"}
			return nil
		})
	client.EXPECT().Invoke(ctx, "createModel", gomock.Any(), nil).Return(nil)

	err := s.ensureNoteType(ctx)
	require.NoError(t, err)
}

func TestEnsureNoteType_ReconcilesTemplatesWhenModelMatches(t *testing.T) {
	s, client := newTestService(t)
	ctx := context.Background()

	client.EXPECT().Invoke(ctx, "modelNames", nil, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]string)) = []string{model_name}
			return nil
		})
	client.EXPECT().Invoke(ctx, "modelFieldNames", map[string]string{"modelName": model_name}, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]string)) = noteTypeFieldNames()
			return nil
		})
	client.EXPECT().Invoke(ctx, "updateModelTemplates", gomock.Any(), nil).Return(nil)
	client.EXPECT().Invoke(ctx, "updateModelStyling", gomock.Any(), nil).Return(nil)

	err := s.ensureNoteType(ctx)
	require.NoError(t, err)
}

func TestEnsureNoteType_ErrorsOnFieldMismatch(t *testing.T) {
	s, client := newTestService(t)
	ctx := context.Background()

	client.EXPECT().Invoke(ctx, "modelNames", nil, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]string)) = []string{model_name}
			return nil
		})
	client.EXPECT().Invoke(ctx, "modelFieldNames", map[string]string{"modelName": model_name}, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]string)) = []string{"Front", "Back"}
			return nil
		})

	err := s.ensureNoteType(ctx)
	require.Error(t, err)
}

func TestAddFlashcard_AddsNewNoteWhenNoneFound(t *testing.T) {
	s, client := newTestService(t)
	ctx := context.Background()

	flashcard := &models.Flashcard{
		Subject:  "lime",
		DeckName: "English",
		Examples: []models.Example{{Sentence: "The lime is sour."}},
		Synonyms: []string{"citrus"},
		Tags:     []string{"fruit"},
	}

	client.EXPECT().Invoke(gomock.Any(), "createDeck", map[string]string{"deck": "English"}, nil).Return(nil)
	client.EXPECT().Invoke(gomock.Any(), "findNotes", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]int64)) = []int64{}
			return nil
		})

	var addedFields map[string]string
	client.EXPECT().Invoke(gomock.Any(), "addNote", gomock.Any(), nil).DoAndReturn(
		func(_ context.Context, _ string, params any, _ any) error {
			addedFields = params.(addNoteParams).Note.Fields
			assert.Equal(t, "English", params.(addNoteParams).Note.DeckName)
			assert.Contains(t, params.(addNoteParams).Note.Tags, default_tag)
			return nil
		})

	err := s.AddFlashcard(ctx, flashcard)
	require.NoError(t, err)
	assert.Equal(t, "lime", addedFields["Subject"])
	assert.Equal(t, "citrus", addedFields["Synonyms"])
	assert.Equal(t, "<ul><li>The <b>lime</b> is sour.</li></ul>", addedFields["Example"])
}

func TestAddFlashcard_UpdatesExistingNote(t *testing.T) {
	s, client := newTestService(t)
	s.decks["English"] = struct{}{} // deck already ensured, no createDeck call expected
	ctx := context.Background()

	flashcard := &models.Flashcard{Subject: "lime", DeckName: "English"}

	client.EXPECT().Invoke(gomock.Any(), "findNotes", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]int64)) = []int64{42}
			return nil
		})
	client.EXPECT().Invoke(gomock.Any(), "updateNoteFields", gomock.Any(), nil).DoAndReturn(
		func(_ context.Context, _ string, params any, _ any) error {
			assert.Equal(t, int64(42), params.(updateNoteFieldsParams).Note.ID)
			return nil
		})
	client.EXPECT().Invoke(gomock.Any(), "addTags", addTagsParams{Notes: []int64{42}, Tags: default_tag}, nil).Return(nil)

	err := s.AddFlashcard(ctx, flashcard)
	require.NoError(t, err)
}

func TestAddFlashcard_StoresMediaAndFormatsFields(t *testing.T) {
	s, client := newTestService(t)
	s.decks["English"] = struct{}{}
	ctx := context.Background()

	flashcard := &models.Flashcard{
		Subject:       "lime",
		DeckName:      "English",
		Picture:       &models.File{Filename: "lime.jpg", Content: []byte("data")},
		Pronunciation: &models.File{Filename: "lime.mp3", Content: []byte("audio")},
	}

	client.EXPECT().Invoke(gomock.Any(), "storeMediaFile", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, params any, result any) error {
			filename := params.(map[string]string)["filename"]
			*(result.(*string)) = filename
			return nil
		}).Times(2)
	client.EXPECT().Invoke(gomock.Any(), "findNotes", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, _ any, result any) error {
			*(result.(*[]int64)) = []int64{}
			return nil
		})

	var fields map[string]string
	client.EXPECT().Invoke(gomock.Any(), "addNote", gomock.Any(), nil).DoAndReturn(
		func(_ context.Context, _ string, params any, _ any) error {
			fields = params.(addNoteParams).Note.Fields
			return nil
		})

	err := s.AddFlashcard(ctx, flashcard)
	require.NoError(t, err)
	assert.Equal(t, "<img src='lime.jpg'>", fields["Picture"])
	assert.Equal(t, "lime.mp3", fields["Pronunciation"])
}
