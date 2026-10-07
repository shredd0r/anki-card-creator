package llm

//go:generate mockgen -source client.go -destination mock/client_mock.go

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tmc/langchaingo/llms"
)

var (
	ErrGenerationIsNotDone   = errors.New("generation isn`t done")
	ErrModelNotFound         = errors.New("model not found")
	ErrNoOneMessageGenerated = errors.New("no one messsage generated")
)

type GeneratedCardContent struct {
	Paraphrase    string   `json:"paraphrase"`
	Transcription *string  `json:"transcription"`
	Examples      []string `json:"examples"`
	Synonyms      []string `json:"synonyms"`
}

type RatedPictureContent struct {
	Rating uint8 `json:"rating"`
}

type Client interface {
	GenerateCardContent(ctx context.Context, prompt string) (*[]byte, error)
	RatePicture(ctx context.Context, subject string, base64PictureContent string, mimeType string) (*[]byte, error)
	HealthCheck(ctx context.Context) error
}

type implClient struct {
	seed   int
	logger *slog.Logger

	cardContentLLM llms.Model
	ratePictureLLM llms.Model
	healthCheckLLM llms.Model
}

// NewClient wraps three already-constructed langchaingo llms.Model instances
// (one per response shape) into a Client. There are three, not one, because
// langchaingo only accepts a structured-output ResponseFormat at
// client-construction time, not per call - cardContentLLM and
// ratePictureLLM are expected to be constructed with their respective
// ResponseFormat (see schema.go), healthCheckLLM with none.
//
// See NewOpenAIClient for the constructor that actually builds these models
// against an OpenAI-compatible endpoint.
func NewClient(logger *slog.Logger, seed int, cardContentLLM, ratePictureLLM, healthCheckLLM llms.Model) Client {
	return &implClient{
		logger:         logger,
		seed:           seed,
		cardContentLLM: cardContentLLM,
		ratePictureLLM: ratePictureLLM,
		healthCheckLLM: healthCheckLLM,
	}
}

func (p *implClient) GenerateCardContent(ctx context.Context, prompt string) (*[]byte, error) {
	return p.chatCompletionRequest(ctx, p.cardContentLLM, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemInstructionForCardContent),
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
}

func (p *implClient) RatePicture(ctx context.Context, subject string, base64PictureContent string, mimeType string) (*[]byte, error) {
	return p.chatCompletionRequest(ctx, p.ratePictureLLM, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemInstructionForRatePicture),
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextContent{Text: subject},
				llms.ImageURLPart(fmt.Sprintf("data:%s;base64,%s", mimeType, base64PictureContent)),
			},
		},
	})
}

func (p *implClient) HealthCheck(ctx context.Context) error {
	_, err := p.chatCompletionRequest(ctx, p.healthCheckLLM, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Healthcheck"),
	})
	return err
}

func (p *implClient) chatCompletionRequest(ctx context.Context, model llms.Model, messages []llms.MessageContent) (*[]byte, error) {
	resp, err := model.GenerateContent(ctx, messages, llms.WithSeed(p.seed))
	if err != nil {
		p.logger.Error("received error response from llm server", "err", err)
		return nil, err
	}

	if len(resp.Choices) < 1 {
		p.logger.Error("server doesn't return any mesages")
		return nil, ErrNoOneMessageGenerated
	}

	choice := resp.Choices[0]
	p.logger.Debug("received content", slog.String("content", choice.Content), slog.String("reasoning-content", choice.ReasoningContent))

	// Some locally-served "thinking"/reasoning models (via LM Studio and
	// similar OpenAI-compatible servers) put the entire response - including
	// the JSON we asked for - into message.reasoning_content and leave
	// message.content empty, instead of splitting chain-of-thought from the
	// final answer. Fall back to it rather than treating an empty Content as
	// "no response".
	content := choice.Content
	if content == "" {
		content = choice.ReasoningContent
	}

	outputText := []byte(content)
	return &outputText, nil
}
