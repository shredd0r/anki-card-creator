package llm

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
)

// fakeModel is a minimal llms.Model used to control how long
// GenerateContent takes to respond, so timeout behavior can be exercised
// without a real LLM server.
type fakeModel struct {
	delay    time.Duration
	response *llms.ContentResponse
}

func (f *fakeModel) GenerateContent(ctx context.Context, _ []llms.MessageContent, _ ...llms.CallOption) (*llms.ContentResponse, error) {
	select {
	case <-time.After(f.delay):
		return f.response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeModel) Call(_ context.Context, _ string, _ ...llms.CallOption) (string, error) {
	return "", nil
}

func newTestClient(t *testing.T, timeout time.Duration, model llms.Model) Client {
	t.Helper()
	return NewClient(slog.Default(), 1, timeout, model, model, model)
}

func TestChatCompletionRequest_TimesOutWhenServerIsSlow(t *testing.T) {
	model := &fakeModel{delay: 50 * time.Millisecond}
	client := newTestClient(t, 5*time.Millisecond, model)

	_, err := client.GenerateCardContent(context.Background(), "prompt")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestChatCompletionRequest_SucceedsWithinTimeout(t *testing.T) {
	model := &fakeModel{
		response: &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: `{"ok":true}`}}},
	}
	client := newTestClient(t, time.Second, model)

	resp, err := client.GenerateCardContent(context.Background(), "prompt")

	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(*resp))
}

func TestChatCompletionRequest_NoTimeoutWhenZero(t *testing.T) {
	model := &fakeModel{
		delay:    20 * time.Millisecond,
		response: &llms.ContentResponse{Choices: []*llms.ContentChoice{{Content: `{"ok":true}`}}},
	}
	client := newTestClient(t, 0, model)

	resp, err := client.GenerateCardContent(context.Background(), "prompt")

	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(*resp))
}

func TestChatCompletionRequest_RespectsParentCancellation(t *testing.T) {
	model := &fakeModel{delay: time.Second}
	client := newTestClient(t, time.Minute, model)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GenerateCardContent(ctx, "prompt")

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
