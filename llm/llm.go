package llm

//go:generate mockgen -source llm.go -destination mock/llm_mock.go

import (
	"context"
	"errors"

	"github.com/shredd0r/anki-card-creator/models"
)

var (
	errRequestLimitReached = errors.New("request limit reached")
	errServerIsOverload    = errors.New("server is overload")
)

type GeneratedCardContent struct {
	Paraphrase string   `json:"paraphrase"`
	Examples   []string `json:"examples"`
	Synonyms   []string `json:"synonyms"`
}

type Provider interface {
	GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error)
	RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error)
}
