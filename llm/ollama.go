package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/prathyushnallamothu/ollamago"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	responseStructureForCardContent = `
		{
			"type": "object",
			"properties": {
				"paraphrase": { "type": "string" },
				"examples": { 
					"type": "array",
					"items": {
						"type": "string"
					}
				},
				"synonyms": { 
					"type": "array",
					"items": {
						"type": "string"
					}
				}
			}
		}`
	responseStructureForRatePicture = `
		{
			"type": "object",
			"properties": {
				"rating": { "type": "integer" },
				"analysis": { "type": "string" }
			}
		}`
)

var (
	errGenerationIsNotDone = errors.New("generation isn`t done")
	errModelNotFound       = errors.New("model not found")
)

type ollama struct {
	model  string
	logger *slog.Logger
	client *ollamago.Client
}

func NewOllama(logger *slog.Logger, model string, client *ollamago.Client) Provider {
	return &ollama{
		model:  model,
		logger: logger.WithGroup("ollama-provider"),
		client: client,
	}
}

func (p *ollama) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	p.logger.Debug("start generate card content", slog.Any("subject", subject))

	resp, err := p.client.Generate(ctx, ollamago.GenerateRequest{
		Model:  p.model,
		Prompt: getPromptRequestBy(subject, usingContext),
		Stream: false,
		System: systemInstructionForCardContent,
		Format: []byte(responseStructureForCardContent),
	})

	if p.handleOllamaError(resp, err) != nil {
		return nil, err
	}

	var generateCardContent GeneratedCardContent
	err = json.Unmarshal([]byte(resp.Response), &generateCardContent)
	if err != nil {
		p.logger.Error("failed unmarshal card content", slog.Any("err", err.Error()))
		return nil, err
	}

	return &generateCardContent, nil
}

func (p *ollama) RatePicture(ctx context.Context, subject string, picture *models.File) (*uint8, error) {
	p.logger.Debug("start rate picture", slog.Any("subject", subject))
	resp, err := p.client.Generate(ctx, ollamago.GenerateRequest{
		Model:  p.model,
		Prompt: getPromptRequestBy(subject, nil),
		Stream: false,
		System: systemInstructionForRatePicture,
		Images: []string{
			base64.StdEncoding.EncodeToString(picture.Content),
		},
		Format: []byte(responseStructureForRatePicture),
	})

	if p.handleOllamaError(resp, err) != nil {
		return nil, err
	}

	var ratingResponse ratingPictureResponse
	err = json.Unmarshal([]byte(resp.Response), &ratingResponse)
	if err != nil {
		p.logger.Error("failed unmarshal rating picture", slog.Any("err", err.Error()))
		return nil, err
	}

	p.logger.Debug("thinking during rate picture", slog.Any("analysis", ratingResponse.Analysis))

	return &ratingResponse.Rating, nil
}

func (p *ollama) HealthCheck(ctx context.Context) error {
	resp, err := p.client.Generate(ctx, ollamago.GenerateRequest{
		Model:  p.model,
		Prompt: "My name is Creator, whats yours?",
	})

	if p.handleOllamaError(resp, err) != nil {
		return err
	}
	return nil
}

func (p *ollama) handleOllamaError(resp *ollamago.GenerateResponse, err error) error {
	if err != nil {
		p.logger.Error("received error from server", slog.Any("err", err.Error()))

		var respErr ollamago.ResponseError
		err := json.Unmarshal([]byte(resp.Response), &respErr)
		if err != nil {
			p.logger.Error("failed unmarshal response error", slog.Any("err", err.Error()))
			return err
		}

		// https://docs.ollama.com/api/errors
		switch respErr.StatusCode {
		case http.StatusTooManyRequests:
			{
				p.logger.Error("too many generation requests to ollama client")
				return errRequestLimitReached
			}
		case http.StatusNotFound:
			{
				p.logger.Error("model not found")
				return errModelNotFound
			}
		}

		return err
	}

	if !resp.Done {
		p.logger.Error("generation isn't done yet")
		return errGenerationIsNotDone
	}

	return nil
}
