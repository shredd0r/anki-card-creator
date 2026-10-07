package fetcher

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	mock_extractor "github.com/shredd0r/anki-card-creator/extractor/mock"
	mock_imagesearch "github.com/shredd0r/anki-card-creator/imagesearch/mock"
	"github.com/shredd0r/anki-card-creator/llm"
	mock_llm "github.com/shredd0r/anki-card-creator/llm/mock"
	"github.com/shredd0r/anki-card-creator/models"
	mock_tts "github.com/shredd0r/anki-card-creator/tts/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestWithDictionaryCardContent_GetCardContent_FullCambridgeSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	ce := mock_extractor.NewMockCambridge(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	transcription := "word-transcription"
	pronunciation := &models.File{Content: []byte("word-audio")}

	ce.EXPECT().GetCard(gomock.Any(), "word").Return(&extractor.CambridgeCard{
		Subject:       "word",
		Pronunciation: pronunciation,
		Transcription: &transcription,
		Explains:      []string{"cambridge-paraphrase"},
		Examples:      []string{"cambridge-example-1", "cambridge-example-2"},
	}, nil)
	lp.EXPECT().GenerateCardContent(gomock.Any(), "word", (*[]string)(nil)).Return(&llm.GeneratedCardContent{
		Paraphrase:    "ai-paraphrase-should-be-ignored",
		Transcription: nil,
		Examples:      []llm.Example{{Sentence: "ai-example-should-be-ignored"}},
		Synonyms:      []string{"ai-synonym-1", "ai-synonym-2"},
	}, nil)

	c := &withDictionaryCardContent{
		mediaFetcher:       mediaFetcher{logger: slog.Default(), llmProvider: lp},
		cambridgeExtractor: ce,
	}

	got, err := c.GetCardContent(context.Background(), "word", nil)
	require.NoError(t, err)
	assert.Equal(t, "cambridge-paraphrase", got.Paraphrase)
	assert.Equal(t, &transcription, got.Transcription)
	assert.Equal(t, pronunciation, got.Pronunciation)
	assert.Equal(t, []models.Example{{Sentence: "cambridge-example-1"}, {Sentence: "cambridge-example-2"}}, got.Examples)
	// Synonyms always come from AI - Cambridge has no synonym data.
	assert.Equal(t, []string{"ai-synonym-1", "ai-synonym-2"}, got.Synonyms)
}

func TestWithDictionaryCardContent_GetCardContent_PerFieldFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	ce := mock_extractor.NewMockCambridge(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	aiTranscription := "ai-transcription"

	// Cambridge found the word but couldn't scrape explains/examples/pronunciation for it.
	ce.EXPECT().GetCard(gomock.Any(), "word").Return(&extractor.CambridgeCard{
		Subject:       "word",
		Pronunciation: nil,
		Transcription: nil,
		Explains:      []string{},
		Examples:      []string{},
	}, nil)
	lp.EXPECT().GenerateCardContent(gomock.Any(), "word", (*[]string)(nil)).Return(&llm.GeneratedCardContent{
		Paraphrase:    "ai-paraphrase",
		Transcription: &aiTranscription,
		Examples:      []llm.Example{{Sentence: "ai-example"}},
		Synonyms:      []string{"ai-synonym"},
	}, nil)

	c := &withDictionaryCardContent{
		mediaFetcher:       mediaFetcher{logger: slog.Default(), llmProvider: lp},
		cambridgeExtractor: ce,
	}

	got, err := c.GetCardContent(context.Background(), "word", nil)
	require.NoError(t, err)
	assert.Equal(t, "ai-paraphrase", got.Paraphrase)
	assert.Equal(t, &aiTranscription, got.Transcription)
	assert.Nil(t, got.Pronunciation)
	assert.Equal(t, []models.Example{{Sentence: "ai-example"}}, got.Examples)
	assert.Equal(t, []string{"ai-synonym"}, got.Synonyms)
}

func TestWithDictionaryCardContent_GetCardContent_CambridgeHardErrorFallsBackFully(t *testing.T) {
	ctrl := gomock.NewController(t)
	ce := mock_extractor.NewMockCambridge(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	aiTranscription := "ai-transcription"

	ce.EXPECT().GetCard(gomock.Any(), "some phrase").Return(nil, errors.New("unsupported subject"))
	lp.EXPECT().GenerateCardContent(gomock.Any(), "some phrase", (*[]string)(nil)).Return(&llm.GeneratedCardContent{
		Paraphrase:    "ai-paraphrase",
		Transcription: &aiTranscription,
		Examples:      []llm.Example{{Sentence: "ai-example"}},
		Synonyms:      []string{"ai-synonym"},
	}, nil)

	c := &withDictionaryCardContent{
		mediaFetcher:       mediaFetcher{logger: slog.Default(), llmProvider: lp},
		cambridgeExtractor: ce,
	}

	got, err := c.GetCardContent(context.Background(), "some phrase", nil)
	require.NoError(t, err)
	assert.Equal(t, "ai-paraphrase", got.Paraphrase)
	assert.Equal(t, &aiTranscription, got.Transcription)
	assert.Nil(t, got.Pronunciation)
	assert.Equal(t, []models.Example{{Sentence: "ai-example"}}, got.Examples)
	assert.Equal(t, []string{"ai-synonym"}, got.Synonyms)
}

func TestWithDictionaryCardContent_GetCardContent_LLMErrorIsFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	ce := mock_extractor.NewMockCambridge(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	ce.EXPECT().GetCard(gomock.Any(), "word").Return(&extractor.CambridgeCard{
		Subject: "word", Explains: []string{"cambridge-paraphrase"},
	}, nil)
	lp.EXPECT().GenerateCardContent(gomock.Any(), "word", (*[]string)(nil)).Return(nil, errors.New("llm unavailable"))

	c := &withDictionaryCardContent{
		mediaFetcher:       mediaFetcher{logger: slog.Default(), llmProvider: lp},
		cambridgeExtractor: ce,
	}

	got, err := c.GetCardContent(context.Background(), "word", nil)
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestOnlyAICardContent_GetCardContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	lp := mock_llm.NewMockProvider(ctrl)

	transcription := "ai-transcription"
	lp.EXPECT().GenerateCardContent(gomock.Any(), "subject", (*[]string)(nil)).Return(&llm.GeneratedCardContent{
		Paraphrase:    "ai-paraphrase",
		Transcription: &transcription,
		Examples:      []llm.Example{{Sentence: "ai-example"}},
		Synonyms:      []string{"ai-synonym"},
	}, nil)

	c := &onlyAICardContent{mediaFetcher: mediaFetcher{logger: slog.Default(), llmProvider: lp}}

	got, err := c.GetCardContent(context.Background(), "subject", nil)
	require.NoError(t, err)
	assert.Equal(t, "ai-paraphrase", got.Paraphrase)
	assert.Equal(t, &transcription, got.Transcription)
	assert.Nil(t, got.Pronunciation) // comes from GetPronunciation (TTS), not here
	assert.Equal(t, []models.Example{{Sentence: "ai-example"}}, got.Examples)
	assert.Equal(t, []string{"ai-synonym"}, got.Synonyms)
}

func TestOnlyAICardContent_GetCardContent_LLMError(t *testing.T) {
	ctrl := gomock.NewController(t)
	lp := mock_llm.NewMockProvider(ctrl)
	lp.EXPECT().GenerateCardContent(gomock.Any(), "subject", (*[]string)(nil)).Return(nil, errors.New("llm unavailable"))

	c := &onlyAICardContent{mediaFetcher: mediaFetcher{logger: slog.Default(), llmProvider: lp}}

	got, err := c.GetCardContent(context.Background(), "subject", nil)
	require.Error(t, err)
	assert.Nil(t, got)
}

// mediaFetcher is embedded identically by both scenario types - test it once directly.

func TestMediaFetcher_GetPicture_IgnoredWhenConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	gip := mock_imagesearch.NewMockImage(ctrl) // no calls expected

	m := &mediaFetcher{
		cfg:                 config.PictureConfig{Ignore: true},
		logger:              slog.Default(),
		imageSearchProvider: gip,
	}

	picture, err := m.GetPicture(context.Background(), "subject", nil)
	require.NoError(t, err)
	assert.Nil(t, picture)
}

func TestMediaFetcher_GetPicture_RetriesUntilRatingThresholdMet(t *testing.T) {
	ctrl := gomock.NewController(t)
	gip := mock_imagesearch.NewMockImage(ctrl)
	res := mock_imagesearch.NewMockResult(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	cfg := config.PictureConfig{CountSearches: 3, MinimalRating: 7}
	gip.EXPECT().Request(gomock.Any(), "subject").Return(res, nil)

	lowRated := &models.File{Filename: "low.jpg"}
	goodRated := &models.File{Filename: "good.jpg"}
	res.EXPECT().Get(gomock.Any(), "subject", uint(0)).Return(lowRated, nil)
	lp.EXPECT().RatePicture(gomock.Any(), "subject", lowRated).Return(&llm.RatedPictureContent{Rating: 3}, nil)
	res.EXPECT().Get(gomock.Any(), "subject", uint(1)).Return(goodRated, nil)
	lp.EXPECT().RatePicture(gomock.Any(), "subject", goodRated).Return(&llm.RatedPictureContent{Rating: 8}, nil)

	m := &mediaFetcher{cfg: cfg, logger: slog.Default(), llmProvider: lp, imageSearchProvider: gip}

	picture, err := m.GetPicture(context.Background(), "subject", nil)
	require.NoError(t, err)
	assert.Equal(t, goodRated, picture)
}

func TestMediaFetcher_GetPicture_ExhaustsAttemptsWithoutSuitablePicture(t *testing.T) {
	ctrl := gomock.NewController(t)
	gip := mock_imagesearch.NewMockImage(ctrl)
	res := mock_imagesearch.NewMockResult(ctrl)
	lp := mock_llm.NewMockProvider(ctrl)

	cfg := config.PictureConfig{CountSearches: 2, MinimalRating: 7}
	gip.EXPECT().Request(gomock.Any(), "subject").Return(res, nil)
	for i := range uint(2) {
		res.EXPECT().Get(gomock.Any(), "subject", i).Return(&models.File{}, nil)
		lp.EXPECT().RatePicture(gomock.Any(), "subject", gomock.Any()).Return(&llm.RatedPictureContent{Rating: 3}, nil)
	}

	m := &mediaFetcher{cfg: cfg, logger: slog.Default(), llmProvider: lp, imageSearchProvider: gip}

	picture, err := m.GetPicture(context.Background(), "subject", nil)
	require.NoError(t, err)
	assert.Nil(t, picture)
}

func TestMediaFetcher_GetPronunciation(t *testing.T) {
	ctrl := gomock.NewController(t)
	speech := mock_tts.NewMockSpeech(ctrl)
	speech.EXPECT().Create(gomock.Any(), "subject").Return(bytes.NewReader([]byte("audio-bytes")), nil)

	m := &mediaFetcher{logger: slog.Default(), speech: speech}

	file, err := m.GetPronunciation(context.Background(), "subject")
	require.NoError(t, err)
	assert.Equal(t, []byte("audio-bytes"), file.Content)
	assert.Equal(t, "audio/mpeg", file.MIMEType)
}

func TestMediaFetcher_GetPronunciation_SpeechError(t *testing.T) {
	ctrl := gomock.NewController(t)
	speech := mock_tts.NewMockSpeech(ctrl)
	speech.EXPECT().Create(gomock.Any(), "subject").Return(nil, errors.New("tts unavailable"))

	m := &mediaFetcher{logger: slog.Default(), speech: speech}

	file, err := m.GetPronunciation(context.Background(), "subject")
	require.Error(t, err)
	assert.Nil(t, file)
}
