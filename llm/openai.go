package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"github.com/shredd0r/anki-card-creator/config"
)

const seed = 459273690274

var ErrNoOneMessageGenerated = errors.New("no one messsage generated")

type OpenAIClient struct {
	seed  int64
	model string

	logger *slog.Logger
	client *openai.Client
}

func NewOpenAIClient(logger *slog.Logger, cfg *config.LLMConfig) Client {
	c := openai.NewClient(
		option.WithAPIKey("key"),
		option.WithBaseURL("http://localhost:1234/v1/"),
	)

	return &OpenAIClient{
		logger: logger.WithGroup("openAI"),
		seed:   cfg.Seed,
		model:  cfg.Model,
		client: &c,
	}
}

func (p *OpenAIClient) GenerateCardContent(ctx context.Context, prompt string) (*[]byte, error) {
	return p.chatCompletionRequest(ctx,
		openai.ChatCompletionNewParams{
			Model: p.model,
			ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &responses.ResponseFormatJSONSchemaParam{
					JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
						Name:   "card_content",
						Schema: cardContentSchema,
						Strict: openai.Bool(true),
					},
				},
			},
			Seed: openai.Int(p.seed),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.ChatCompletionMessageParamUnion{
					OfSystem: &openai.ChatCompletionSystemMessageParam{
						Role: "system",
						Content: openai.ChatCompletionSystemMessageParamContentUnion{
							OfString: openai.String(systemInstructionForCardContent),
						},
					},
				},
				openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Role: "user",
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String(prompt),
						},
					},
				},
			},
		})
}

func (p *OpenAIClient) RatePicture(ctx context.Context, subject string, base64PictureContent string, mimeType string) (*[]byte, error) {
	return p.chatCompletionRequest(ctx,
		openai.ChatCompletionNewParams{
			Model: p.model,
			ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &responses.ResponseFormatJSONSchemaParam{
					JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
						Name:   "rate_picture",
						Schema: ratePictureSchema,
						Strict: openai.Bool(true),
					},
				},
			},

			Seed: openai.Int(p.seed),
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.ChatCompletionMessageParamUnion{
					OfSystem: &openai.ChatCompletionSystemMessageParam{
						Role: "system",
						Content: openai.ChatCompletionSystemMessageParamContentUnion{
							OfString: openai.String(systemInstructionForRatePicture),
						},
					},
				},
				openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Role: "user",
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String(subject),
						},
					},
				},
				openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Role: "user",
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfArrayOfContentParts: []openai.ChatCompletionContentPartUnionParam{
								{
									OfImageURL: &openai.ChatCompletionContentPartImageParam{
										ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
											URL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64PictureContent),
										},
									},
								},
							},
						},
					},
				},
			},
		})
}

func (p *OpenAIClient) HealthCheck(ctx context.Context) error {
	_, err := p.chatCompletionRequest(ctx,
		openai.ChatCompletionNewParams{
			Seed:  openai.Int(p.seed),
			Model: p.model,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Role: "user",
						Content: openai.ChatCompletionUserMessageParamContentUnion{
							OfString: openai.String("Healthcheck"),
						},
					},
				},
			},
		})

	return err
}

func (p *OpenAIClient) chatCompletionRequest(ctx context.Context, params openai.ChatCompletionNewParams) (*[]byte, error) {
	resp, err := p.client.Chat.Completions.New(ctx, params)

	if err != nil {
		p.logger.Error("received error response from OpenAI server", "err", err)
		return nil, err
	}

	if len(resp.Choices) < 1 {
		p.logger.Error("server doesn't return any mesages")
		return nil, ErrNoOneMessageGenerated
	}

	outputText := []byte(resp.Choices[0].Message.Content)
	return &outputText, nil
}
