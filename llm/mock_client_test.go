package llm_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This demonstrates using NewMockClient directly as a Provider backend in a
// test when a real LLM server isn't available. For asserting specific call
// arguments/responses, prefer the mockgen mocks in llm/mock instead.
func TestMockClient_GenerateCardContent(t *testing.T) {
	provider := llm.NewProvider(llm.NewMockClient(slog.Default()))

	content, err := provider.GenerateCardContent(context.Background(), "lime", nil)
	require.NoError(t, err)

	assert.Contains(t, content.Paraphrase, "lime")
	require.NotNil(t, content.Transcription)
	assert.Len(t, content.Examples, 3)
	assert.Len(t, content.Synonyms, 3)
}

func TestMockClient_RatePicture(t *testing.T) {
	provider := llm.NewProvider(llm.NewMockClient(slog.Default()))

	rating, err := provider.RatePicture(context.Background(), "lime", &models.File{
		Content:  []byte("mock-image-bytes"),
		MIMEType: "image/jpeg",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, rating.Rating, uint8(5))
}

func TestMockClient_HealthCheck(t *testing.T) {
	provider := llm.NewProvider(llm.NewMockClient(slog.Default()))
	require.NoError(t, provider.HealthCheck(context.Background()))
}
