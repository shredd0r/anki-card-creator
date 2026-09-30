package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// mockRating is always well above any reasonable picture.min-rating, so the
// mock client never blocks on the picture-search retry loop.
const mockRating = 10

// NewMockClient returns a Client that never talks to a real LLM server. It
// returns sensible, subject-aware canned content for every call, so the rest
// of the pipeline (card assembly, Anki writes) can be exercised end-to-end
// without LM Studio/OpenAI running. Useful both for manual local runs and as
// a lightweight stand-in in tests that don't need per-call assertions (use
// the mockgen mocks in llm/mock for that instead).
func NewMockClient(logger *slog.Logger) Client {
	return &implMockClient{logger: logger.WithGroup("mock-llm")}
}

type implMockClient struct {
	logger *slog.Logger
}

func (c *implMockClient) GenerateCardContent(ctx context.Context, prompt string) (*[]byte, error) {
	subject := extractSubjectFromPrompt(prompt)
	c.logger.Debug("mock generating card content", slog.String("subject", subject))

	transcription := fmt.Sprintf("/mɒk-%s/", subject)
	content := GeneratedCardContent{
		Paraphrase:    fmt.Sprintf("A mock, generated explanation of what %q means.", subject),
		Transcription: &transcription,
		Examples: []string{
			fmt.Sprintf("This is a mock example sentence using %q.", subject),
			fmt.Sprintf("Here is another mock sentence with %q in it.", subject),
			fmt.Sprintf("A third mock example that mentions %q.", subject),
		},
		Synonyms: []string{
			fmt.Sprintf("%s-synonym-1", subject),
			fmt.Sprintf("%s-synonym-2", subject),
			fmt.Sprintf("%s-synonym-3", subject),
		},
	}

	body, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	return &body, nil
}

func (c *implMockClient) RatePicture(ctx context.Context, subject string, base64PictureContent string, mimeType string) (*[]byte, error) {
	c.logger.Debug("mock rating picture", slog.String("subject", subject))

	body, err := json.Marshal(RatedPictureContent{Rating: mockRating})
	if err != nil {
		return nil, err
	}
	return &body, nil
}

func (c *implMockClient) HealthCheck(ctx context.Context) error {
	return nil
}

func extractSubjectFromPrompt(prompt string) string {
	var pr struct {
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal([]byte(prompt), &pr); err != nil || pr.Subject == "" {
		return "subject"
	}
	return pr.Subject
}
