package llm

//go:generate mockgen -source client.go -destination mock/client_mock.go

import (
	"context"
	"errors"
)

var (
	ErrGenerationIsNotDone = errors.New("generation isn`t done")
	ErrModelNotFound       = errors.New("model not found")
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
