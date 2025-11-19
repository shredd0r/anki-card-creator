package llm

import (
	"context"

	"github.com/shredd0r/anki-card-creator/models"
)

type localLLM struct {
}

func NewLocalLLM() Provider {
	return &localLLM{}
}

func (p *localLLM) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	return nil, nil
}

func (p *localLLM) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	return nil, nil
}
