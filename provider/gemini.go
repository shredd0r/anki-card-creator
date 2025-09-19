package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/models"
	"google.golang.org/genai"
)

const (
	model_for_generate_text = "gemini-2.5-flash"
)

// TODO add checking out of tokens, 503 error code (server is overload)
type GeminiProvider interface {
	GenerateExamples(ctx context.Context, subject string) (*[]string, error)
	GenerateExplain(ctx context.Context, subject string) (*string, error)
	RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error)
}

func NewGeminiProvider(logger *slog.Logger, client *genai.Client) GeminiProvider {
	return &implGeminiProvider{
		logger: logger,
		client: client,
		cfgForGenerateExamples: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
				},
			},
			SystemInstruction: genai.NewContentFromText(
				`I will send you new word or phrase or idiom, you should create 5 examples using it, concise, for low level of English proficiency`,
				genai.RoleUser),
		},
		cfgForGenerateExplain: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeString,
			},
			SystemInstruction: genai.NewContentFromText(
				`I will send you word or phrase or idiom, you should write explain this word. Answer could be only 1 sentence, concise, for low level of English proficiency`,
				genai.RoleUser),
		},
		cfgForRatingPicture: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"rating":   {Type: genai.TypeInteger},
					"analysis": {Type: genai.TypeString},
				},
			},
			SystemInstruction: genai.NewContentFromText(
				`I will send you picture and word, you have to analyze this picture. 
				Picture will be use for flashcard, so the picture have to describe word, idiom, phrase. 
				Word, idion or phrase can't be written on picture. 
				You should give me rating between 1 - 10, where 10 its best match `,
				genai.RoleUser),
		},
	}
}

type implGeminiProvider struct {
	logger                 *slog.Logger
	client                 *genai.Client
	cfgForGenerateExamples *genai.GenerateContentConfig
	cfgForGenerateExplain  *genai.GenerateContentConfig
	cfgForRatingPicture    *genai.GenerateContentConfig
}

// GenerateExamples - method for send request to gemini backend for generate examples of subject.
// Subject can be: word, phrase, idiom
func (p *implGeminiProvider) GenerateExamples(ctx context.Context, subject string) (*[]string, error) {
	result, err := p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		genai.Text(subject),
		p.cfgForGenerateExamples,
	)

	if err != nil {
		p.logger.Error(fmt.Sprintf("failed generate content for subject: %s", subject), slog.Any("err", err.Error()))
		return nil, err
	}

	examplesStr := result.Text()
	var examples []string
	err = json.Unmarshal([]byte(examplesStr), &examples)
	if err != nil {
		p.logger.Error("failed unmarshal generated text to array")
	}
	return &examples, nil
}

// GenerateExplain - method for send request to gemini backend for generate explain of subject.
// Subject can be: word, phrase, idiom
func (p *implGeminiProvider) GenerateExplain(ctx context.Context, subject string) (*string, error) {
	result, err := p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		genai.Text(subject),
		p.cfgForGenerateExplain,
	)

	if err != nil {
		p.logger.Error(fmt.Sprintf("failed generate content for subject: %s", subject), slog.Any("err", err.Error()))
		return nil, err
	}

	examplesStr := result.Text()

	return &examplesStr, nil
}

// RatingPicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// I am using rating picture approach, because free gemini api access allows send image. Generation is not allowed.
func (p *implGeminiProvider) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	p.logger.Debug("start method 'RatingPicture'")

	var pictureBytes []byte
	_, err := picture.Content.Read(pictureBytes)

	if err != nil {
		p.logger.Error("failed read picture reader", slog.Any("err", err.Error()))
		return nil, err
	}

	parts := []*genai.Part{
		genai.NewPartFromText(subject),
		genai.NewPartFromBytes(pictureBytes, picture.Type),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	_, err = p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		contents,
		p.cfgForRatingPicture,
	)

	return nil, nil
}
