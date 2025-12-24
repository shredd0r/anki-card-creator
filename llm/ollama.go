package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"syscall"

	"github.com/prathyushnallamothu/ollamago"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	"golang.org/x/sync/errgroup"
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
	ErrGenerationIsNotDone = errors.New("generation isn`t done")
	ErrModelNotFound       = errors.New("model not found")
)

type ollama struct {
	thinkingModel string
	visionModel   string
	logger        *slog.Logger
	client        *ollamago.Client
}

func NewOllama(logger *slog.Logger, cfg config.OllamaConfig, client *ollamago.Client) Provider {
	return &ollama{
		thinkingModel: cfg.ThinkingModel,
		visionModel:   cfg.VisionModel,
		logger:        logger.WithGroup("ollama-provider"),
		client:        client,
	}
}

func (p *ollama) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	p.logger.Debug("start generate card content", slog.Any("subject", subject))

	resp, err := p.client.Generate(ctx, ollamago.GenerateRequest{
		Model:  p.thinkingModel,
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
		Model:  p.visionModel,
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
	p.logger.Debug("start healthcheck on ollama server")
	errwg, childCtx := errgroup.WithContext(ctx)

	if p.isModelsEqual() {
		return p.healthCheck(ctx, p.thinkingModel, p.thinkingModel)
	}

	errwg.Go(
		func() error {
			return p.healthCheck(childCtx, p.thinkingModel, "thinking")
		})
	errwg.Go(
		func() error {
			return p.healthCheck(childCtx, p.visionModel, "vision")

		})
	return errwg.Wait()
}

func (p *ollama) healthCheck(ctx context.Context, modelName string, modelType string) error {
	p.logger.Debug("make request to model", slog.Any("model type", modelType))

	resp, err := p.client.Generate(ctx, ollamago.GenerateRequest{
		Model:  modelName,
		Prompt: "My name is Creator, whats yours?",
	})

	if p.handleOllamaError(resp, err) != nil {
		return err
	}

	p.logger.Debug("model return success response", slog.Any("model name", modelName))
	return nil
}

func (p *ollama) isModelsEqual() bool {
	return p.thinkingModel == p.visionModel
}

func (p *ollama) handleOllamaError(resp *ollamago.GenerateResponse, err error) error {
	if err != nil {
		p.logger.Error("received error from ollama server")
		p.logger.Debug("received erro from ollama server", slog.Any("err", err.Error()))

		if errors.Is(err, syscall.ECONNREFUSED) {
			return err
		}
		if resp == nil {
			return err
		}

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
				return ErrRequestLimitReached
			}
		case http.StatusNotFound:
			{
				p.logger.Error("model not found")
				return ErrModelNotFound
			}
		}

		return err
	}

	if !resp.Done {
		p.logger.Error("generation isn't done yet")
		return ErrGenerationIsNotDone
	}

	return nil
}
