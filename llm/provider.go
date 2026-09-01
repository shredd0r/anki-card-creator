package llm

//go:generate mockgen -source provider.go -destination mock/provider_mock.go

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/shredd0r/anki-card-creator/models"
)

var (
	ErrRequestLimitReached = errors.New("request limit reached")
	ErrServerIsOverload    = errors.New("server is overload")
)

type promptRequest struct {
	Subject      string    `json:"subject"`
	UsingContext *[]string `json:"using-context,omitempty"`
}

type Provider interface {
	GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error)
	RatePicture(ctx context.Context, subject string, picture *models.File) (*RatedPictureContent, error)
	HealthCheck(ctx context.Context) error
}

type implProvider struct {
	client Client
}

func NewProvider(client Client) Provider {
	return &implProvider{
		client: client,
	}
}

func (p *implProvider) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	return callClientAndUnmarshalResponse[GeneratedCardContent](
		func() (*[]byte, error) {
			return p.client.GenerateCardContent(ctx, getPromptRequestBy(subject, usingContext))
		})
}
func (p *implProvider) RatePicture(ctx context.Context, subject string, picture *models.File) (*RatedPictureContent, error) {
	return callClientAndUnmarshalResponse[RatedPictureContent](
		func() (*[]byte, error) {
			return p.client.RatePicture(ctx, subject, base64.StdEncoding.EncodeToString(picture.Content), picture.MIMEType)
		})
}

func (p *implProvider) HealthCheck(ctx context.Context) error {
	return p.client.HealthCheck(ctx)
}

type methodClientCall func() (*[]byte, error)

func callClientAndUnmarshalResponse[T any](method methodClientCall) (*T, error) {
	resp, err := method()

	if err != nil {
		return nil, err
	}

	var content T
	err = json.Unmarshal(*resp, &content)
	if err != nil {
		return nil, err
	}

	return &content, nil
}

func getPromptRequestBy(subject string, usingContext *[]string) string {
	request := promptRequest{
		Subject:      subject,
		UsingContext: usingContext,
	}
	bytes, _ := json.Marshal(&request)
	return string(bytes)
}
