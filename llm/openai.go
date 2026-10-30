package llm

import (
	"fmt"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/tmc/langchaingo/llms/openai"
)

// NewOpenAIClient builds a Client backed by langchaingo's OpenAI-compatible
// chat completions backend, so it works against any OpenAI-compatible
// endpoint (e.g. a local LM Studio server), not just api.openai.com.
//
// It constructs the three llms.Model instances NewClient needs - one for
// card content, one for picture rating, one for the plain health check -
// from cfg.Token/cfg.BaseUrl/cfg.Model, then hands them to NewClient (see
// NewClient for why three separate models are needed).
func NewOpenAIClient(logger *slog.Logger, cfg *config.LLMConfig) (Client, error) {
	cardContentLLM, err := openai.New(
		openai.WithToken(cfg.Token),
		openai.WithBaseURL(cfg.BaseUrl),
		openai.WithModel(cfg.Model),
		openai.WithResponseFormat(cardContentResponseFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("create card content llm client: %w", err)
	}

	ratePictureLLM, err := openai.New(
		openai.WithToken(cfg.Token),
		openai.WithBaseURL(cfg.BaseUrl),
		openai.WithModel(cfg.Model),
		openai.WithResponseFormat(ratePictureResponseFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("create rate picture llm client: %w", err)
	}

	healthCheckLLM, err := openai.New(
		openai.WithToken(cfg.Token),
		openai.WithBaseURL(cfg.BaseUrl),
		openai.WithModel(cfg.Model),
	)
	if err != nil {
		return nil, fmt.Errorf("create health check llm client: %w", err)
	}

	logger.Debug("created langchaingo OpenAI-compatible client", slog.String("model", cfg.Model), slog.String("base-url", cfg.BaseUrl))

	return NewClient(logger.WithGroup("openAI"), int(cfg.Seed), cfg.Timeout, cardContentLLM, ratePictureLLM, healthCheckLLM), nil
}
