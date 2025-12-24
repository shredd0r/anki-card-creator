package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"syscall"
	"time"

	"github.com/shredd0r/anki-card-creator/models"
	"google.golang.org/genai"
)

var errNotSupportedMIMEType = errors.New("unsupported MIME type")

const (
	model_for_generate_text = "gemini-2.5-flash" //free model
	rpm                     = 5                  //request per minute
)

func NewGemini(logger *slog.Logger, client *genai.Client) Provider {
	return &implGemini{
		logger: logger,
		client: client,
		cfgForGenerateContentCard: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"paraphrase": {
						Type: genai.TypeString,
					},
					"examples": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
					},
					"synonyms": {
						Type: genai.TypeArray,
						Items: &genai.Schema{
							Type: genai.TypeString,
						},
					},
				},
			},
			SystemInstruction: genai.NewContentFromText(systemInstructionForCardContent, genai.RoleUser),
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
			SystemInstruction: genai.NewContentFromText(systemInstructionForRatePicture, genai.RoleUser),
		},
	}
}

type implGemini struct {
	logger                    *slog.Logger
	client                    *genai.Client
	cfgForGenerateContentCard *genai.GenerateContentConfig
	cfgForRatingPicture       *genai.GenerateContentConfig
}

// GenerateCardContent generate content for subject. 'usingContext' means that context where is used this subject.
// For example word "starter" is used in context ["food", "cafe", "dish"]
// 'usingContext' can be nil
func (p *implGemini) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	result, err := p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		genai.Text(getPromptRequestBy(subject, usingContext)),
		p.cfgForGenerateContentCard,
	)

	if err != nil {
		p.logger.Error(fmt.Sprintf("failed generate content for subject: %s", subject))
		return nil, p.wrapError(err)
	}

	var geminiCard GeneratedCardContent
	geminiCardStr := result.Text()
	err = json.Unmarshal([]byte(geminiCardStr), &geminiCard)
	if err != nil {
		p.logger.Error("failed unmarshal generate card content", slog.Any("err", err.Error()))
		return nil, err
	}

	return &geminiCard, nil
}

// RatePicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// I am using rating picture approach, because free gemini api access allows send image. Generation is not allowed.
func (p *implGemini) RatePicture(ctx context.Context, subject string, picture *models.File) (*uint8, error) {
	p.logger.Debug("start method 'RatingPicture'")

	// skip picture with mimetype svg, because this type not supported gemini server
	if picture.MIMEType == "image/svg+xml" {
		return nil, errNotSupportedMIMEType
	}

	// Put to contents subject for compare and bytes of picture
	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(subject),
			genai.NewPartFromBytes(picture.Content, picture.MIMEType),
		}, genai.RoleUser),
	}

	resp, err := p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		contents,
		p.cfgForRatingPicture,
	)

	if err != nil {
		p.logger.Error("failed rating picture", slog.Any("err", err.Error()))
		return nil, p.wrapError(err)
	}

	// TODO not sure that this way is right for get json object from gemini response
	response := &ratingPictureResponse{}
	err = json.Unmarshal([]byte(resp.Text()), response)
	if err != nil {
		p.logger.Error("failed unmarshal response to struct", slog.Any("err", err.Error()))
		return nil, err
	}

	p.logger.Debug(fmt.Sprintf("history analization picture: %s", response.Analysis))

	return &response.Rating, nil
}

// HealthCheck - request to server with llm for generate greetings message
func (p *implGemini) HealthCheck(ctx context.Context) error {
	_, err := p.client.Models.GenerateContent(
		ctx,
		model_for_generate_text,
		genai.Text("My name is Creator, whats yours?"),
		nil,
	)
	if err != nil {
		p.logger.Error("failed health check", slog.Any("err", err.Error()))
		return p.wrapError(err)
	}
	return nil
}

func (p *implGemini) wrapError(err error) error {
	if errors.Is(err, syscall.ECONNREFUSED) {
		p.logger.Error("failed connection to gemini server")
		return err
	}

	if !errors.As(err, &genai.APIError{}) {
		return err
	}

	apiErr := err.(genai.APIError)

	switch apiErr.Code {
	case http.StatusServiceUnavailable:
		{
			return ErrServerIsOverload
		}
	case http.StatusTooManyRequests:
		{
			return ErrRequestLimitReached
		}
	default:
		return err
	}
}

type callMethodTask struct {
	nameOfMethod string
	method       func() error
}

// rateLimitGemini decorator for Gemini, which calculate using quota for all requests and wait for new tokens if it is out.
// Add calls to queue if quota is out and wait for new one
type rateLimitGemini struct {
	mt                        sync.Mutex
	isStartedMinutePeriodChan chan struct{}
	activeTaskChan            chan struct{}

	logger         *slog.Logger
	geminiProvider Provider
}

func NewRateLimitGemini(logger *slog.Logger, client *genai.Client) Provider {
	provider := &rateLimitGemini{
		logger:                    logger,
		activeTaskChan:            make(chan struct{}, rpm),
		isStartedMinutePeriodChan: make(chan struct{}, 1),
		geminiProvider:            NewGemini(logger, client),
	}

	return provider
}

// GenerateCardContent generate content for subject. 'usingContext' means that context where is used this subject.
// For example word "starter" is used in context ["food", "cafe", "dish"]
// 'usingContext' can be nil
// Calling this method start process for calculate requesting per minutes for all calls to gemini backend
func (p *rateLimitGemini) GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error) {
	var geminiCard *GeneratedCardContent

	generateContentCardTask := callMethodTask{
		nameOfMethod: "GenerateCardContent",
		method: func() error {
			geminiCardResp, err := p.geminiProvider.GenerateCardContent(ctx, subject, usingContext)
			geminiCard = geminiCardResp
			return err
		},
	}

	err := p.callProviderMethod(ctx, generateContentCardTask)
	if err != nil {
		return nil, err
	}

	return geminiCard, nil
}

// RatePicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// Calling this method start process for calculate requesting per minutes for all calls to gemini backend
func (p *rateLimitGemini) RatePicture(ctx context.Context, subject string, picture *models.File) (*uint8, error) {
	var rating *uint8

	generateExplainTask := callMethodTask{
		nameOfMethod: "RatePicture",
		method: func() error {
			ratingResp, err := p.geminiProvider.RatePicture(ctx, subject, picture)
			rating = ratingResp
			return err
		},
	}

	err := p.callProviderMethod(ctx, generateExplainTask)
	if err != nil {
		return nil, err
	}

	return rating, nil
}

func (p *rateLimitGemini) HealthCheck(ctx context.Context) error {
	generateExplainTask := callMethodTask{
		nameOfMethod: "HealthCheck",
		method: func() error {
			return p.geminiProvider.HealthCheck(ctx)
		},
	}

	return p.callProviderMethod(ctx, generateExplainTask)
}

func (p *rateLimitGemini) callProviderMethod(ctx context.Context, providersMethodTask callMethodTask) error {
	p.logger.Debug("start waiting for new slot in active task channel", slog.Any("method", providersMethodTask.nameOfMethod))
	select {
	case <-ctx.Done():
		{
			p.logger.Debug("context is done, returning from callProviderMethod")
			return ctx.Err()
		}
	case p.activeTaskChan <- struct{}{}:
		{
			p.logger.Debug("active task channel free for new call method")
			defer p.tryStartNewRequestPeriod(ctx)
			return providersMethodTask.method()
		}
	}
}

// This method calls every time when calls one of provider`s method,
// because minute timer need start only after first request to gemini
func (p *rateLimitGemini) tryStartNewRequestPeriod(ctx context.Context) {
	p.mt.Lock()
	defer p.mt.Unlock()

	p.isStartedMinutePeriodChan <- struct{}{}

	p.logger.Debug("start new timer")
	minuteTimer := time.NewTimer(time.Minute)

	go func() {
		defer func() { <-p.isStartedMinutePeriodChan }()
		p.logger.Debug("start goroutine for waiting timer end")
		select {
		case <-ctx.Done():
			{
				p.logger.Debug("context is done, end 'tryStartNewRequestPeriod' goroutine")
				return
			}
		case <-minuteTimer.C:
			{
				p.logger.Debug("timer is done, cleaning queue")
				for range len(p.activeTaskChan) {
					<-p.activeTaskChan
				}
				p.logger.Debug("len active task chan after cleaning", slog.Any("len", len(p.activeTaskChan)))
			}
		}
	}()
}
