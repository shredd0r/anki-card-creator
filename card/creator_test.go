package card

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	mock_fetcher "github.com/shredd0r/anki-card-creator/card/fetcher/mock"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreate_AssemblesFlashcardFromCardContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	transcription := "word-transcription"
	picture := &models.File{Filename: "word.jpeg", MIMEType: "image/jpeg"}
	ttsPronunciation := &models.File{Filename: "word.mp3", MIMEType: "audio/mpeg"}
	usingContext := &[]string{"context-word"}

	cc.EXPECT().GetCardContent(gomock.Any(), "word", usingContext).Return(&models.CardContent{
		Paraphrase:    "word-explain",
		Transcription: &transcription,
		Examples:      []models.Example{{Sentence: "word-example"}},
		Synonyms:      []string{"synonym"},
	}, nil)
	cc.EXPECT().GetPicture(gomock.Any(), "word", usingContext).Return(picture, nil)
	cc.EXPECT().GetPronunciation(gomock.Any(), "word").Return(ttsPronunciation, nil)

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "test-deck", "word", usingContext)
	require.NoError(t, err)

	assert.Equal(t, &models.Flashcard{
		Subject:       "word",
		SubjectType:   models.SubjectTypeWord,
		DeckName:      "test-deck",
		Examples:      []models.Example{{Sentence: "word-example"}},
		Paraphrase:    "word-explain",
		Transcription: &transcription,
		Pronunciation: ttsPronunciation,
		Picture:       picture,
		Synonyms:      []string{"synonym"},
		Tags:          []string{"context-word"},
	}, got)
}

func TestCreate_CardContentPronunciationOverridesTTS(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	dictionaryPronunciation := &models.File{Filename: "cambridge.mp3", MIMEType: "audio/mpeg"}
	ttsPronunciation := &models.File{Filename: "tts.mp3", MIMEType: "audio/mpeg"}

	cc.EXPECT().GetCardContent(gomock.Any(), "word", (*[]string)(nil)).Return(&models.CardContent{
		Paraphrase:    "word-explain",
		Pronunciation: dictionaryPronunciation,
	}, nil)
	cc.EXPECT().GetPicture(gomock.Any(), "word", (*[]string)(nil)).Return(nil, nil)
	// GetPronunciation (TTS) still runs concurrently even when the dictionary
	// already has audio - its result must be discarded in favor of it.
	cc.EXPECT().GetPronunciation(gomock.Any(), "word").Return(ttsPronunciation, nil)

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "test-deck", "word", nil)
	require.NoError(t, err)
	assert.Equal(t, dictionaryPronunciation, got.Pronunciation)
}

func TestCreate_NoUsingContextProducesEmptyTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	cc.EXPECT().GetCardContent(gomock.Any(), "subject", (*[]string)(nil)).Return(&models.CardContent{Paraphrase: "p"}, nil)
	cc.EXPECT().GetPicture(gomock.Any(), "subject", (*[]string)(nil)).Return(nil, nil)
	cc.EXPECT().GetPronunciation(gomock.Any(), "subject").Return(nil, nil)

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "test-deck", "subject", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{}, got.Tags)
}

func TestCreate_SubjectTypeComputedFromSubjectShape(t *testing.T) {
	testcases := []struct {
		subject  string
		expected models.SubjectType
	}{
		{subject: "word", expected: models.SubjectTypeWord},
		{subject: "a phrase", expected: models.SubjectTypePhrase},
	}

	for _, tc := range testcases {
		t.Run(tc.subject, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cc := mock_fetcher.NewMockCardContent(ctrl)
			cc.EXPECT().GetCardContent(gomock.Any(), tc.subject, (*[]string)(nil)).Return(&models.CardContent{}, nil)
			cc.EXPECT().GetPicture(gomock.Any(), tc.subject, (*[]string)(nil)).Return(nil, nil)
			cc.EXPECT().GetPronunciation(gomock.Any(), tc.subject).Return(nil, nil)

			creator := NewFlashcardCreator(slog.Default(), cc)
			got, err := creator.Create(context.Background(), "deck", tc.subject, nil)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got.SubjectType)
		})
	}
}

func TestCreate_GetCardContentErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	cc.EXPECT().GetCardContent(gomock.Any(), "subject", (*[]string)(nil)).Return(nil, errors.New("card content failed"))
	cc.EXPECT().GetPicture(gomock.Any(), "subject", (*[]string)(nil)).AnyTimes().Return(nil, nil)
	cc.EXPECT().GetPronunciation(gomock.Any(), "subject").AnyTimes().Return(nil, nil)

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "deck", "subject", nil)
	require.Nil(t, got)
	assert.EqualError(t, err, "card content failed")
}

func TestCreate_GetPictureErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	cc.EXPECT().GetCardContent(gomock.Any(), "subject", (*[]string)(nil)).AnyTimes().Return(&models.CardContent{}, nil)
	cc.EXPECT().GetPicture(gomock.Any(), "subject", (*[]string)(nil)).Return(nil, errors.New("picture search failed"))
	cc.EXPECT().GetPronunciation(gomock.Any(), "subject").AnyTimes().Return(nil, nil)

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "deck", "subject", nil)
	require.Nil(t, got)
	assert.EqualError(t, err, "picture search failed")
}

func TestCreate_GetPronunciationErrorPropagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	cc := mock_fetcher.NewMockCardContent(ctrl)

	cc.EXPECT().GetCardContent(gomock.Any(), "subject", (*[]string)(nil)).AnyTimes().Return(&models.CardContent{}, nil)
	cc.EXPECT().GetPicture(gomock.Any(), "subject", (*[]string)(nil)).AnyTimes().Return(nil, nil)
	cc.EXPECT().GetPronunciation(gomock.Any(), "subject").Return(nil, errors.New("tts failed"))

	creator := NewFlashcardCreator(slog.Default(), cc)
	got, err := creator.Create(context.Background(), "deck", "subject", nil)
	require.Nil(t, got)
	assert.EqualError(t, err, "tts failed")
}
