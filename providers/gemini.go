package providers

//go:generate mockgen -source gemini.go -destination mock/gemini_mock.go

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shredd0r/anki-card-creator/models"
	"google.golang.org/genai"
)

var (
	errServerIsOverload = errors.New("gemini server is overload")
	errQuotaIsOver      = errors.New("quota for requests is over")
)

const (
	model_for_generate_text = "gemini-2.5-flash" //free model
	rpm                     = 10                 //request per minute
)

type ratingPictureResponse struct {
	Rating   uint
	Analysis string
}

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
				`I will send you new word or phrase or idiom, you should create 5 examples using it.
				 An example sentence showing its usage, and a synonyms.
				 Result have to be concise, structured for easy reading.
				 As result, expecting this format for each example: 'example-text. [Synonym: synonym-text]'`,
				genai.RoleUser),
		},
		cfgForGenerateExplain: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema: &genai.Schema{
				Type: genai.TypeString,
			},
			SystemInstruction: genai.NewContentFromText(
				`I will send you word or phrase or idiom, you should generate paraphrase this word. Answer could be only 1 sentence.
				Paraphrase mustn't have this word, phrase, idiom. 
				Result have to be concise, structured for easy reading.`,
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
				`You have to analyze this picture. 
				Picture will be use for flashcard, the picture have to describe word, idiom, phrase. 
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
		return nil, p.wrapError(err)
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
		return nil, p.wrapError(err)
	}

	var explain string
	explainStr := result.Text()
	err = json.Unmarshal([]byte(explainStr), &explain)
	if err != nil {
		p.logger.Error("failed unmarshal generated explain to str")
	}

	return &explain, nil
}

// RatingPicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// I am using rating picture approach, because free gemini api access allows send image. Generation is not allowed.
func (p *implGeminiProvider) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	p.logger.Debug("start method 'RatingPicture'")

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

func (p *implGeminiProvider) wrapError(err error) error {
	if !errors.As(err, &genai.APIError{}) {
		return err
	}

	apiErr := err.(genai.APIError)

	switch apiErr.Code {
	case http.StatusServiceUnavailable:
		{
			return errServerIsOverload
		}
	case http.StatusTooManyRequests:
		{
			return errQuotaIsOver
		}
	default:
		return err
	}
}

type callMethodTask struct {
	nameOfMethod string
	method       func() error
}

// rateLimitGeminiProvider decorator for GeminiProvider, which calculate using quota for all requests and wait for new tokens if it is out.
// Add calls to queue if quota is out and wait for new one
type rateLimitGeminiProvider struct {
	mt            sync.Mutex
	minuteTimer   *time.Timer
	queueTaskChan chan struct{}

	logger   *slog.Logger
	provider GeminiProvider
}

func NewRateLimitGeminiProvider(ctx context.Context, logger *slog.Logger, client *genai.Client) GeminiProvider {
	provider := &rateLimitGeminiProvider{
		logger:        logger,
		queueTaskChan: make(chan struct{}, rpm),
		provider:      NewGeminiProvider(logger, client),
	}

	return provider
}

func (p *rateLimitGeminiProvider) GenerateExamples(ctx context.Context, subject string) (*[]string, error) {
	var examples *[]string

	generateExamplesTask := callMethodTask{
		nameOfMethod: "GenerateExamples",
		method: func() error {
			examplesResp, err := p.provider.GenerateExamples(ctx, subject)
			examples = examplesResp
			return err
		},
	}

	err := p.callProviderMethod(ctx, generateExamplesTask)

	if err != nil {
		return nil, err
	}

	return examples, nil
}
func (p *rateLimitGeminiProvider) GenerateExplain(ctx context.Context, subject string) (*string, error) {
	var explain *string

	generateExplainTask := callMethodTask{
		nameOfMethod: "GenerateExplain",
		method: func() error {
			explainResp, err := p.provider.GenerateExplain(ctx, subject)
			explain = explainResp
			return err
		},
	}

	err := p.callProviderMethod(ctx, generateExplainTask)
	if err != nil {
		return nil, err
	}

	return explain, nil
}
func (p *rateLimitGeminiProvider) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	var rating *uint

	generateExplainTask := callMethodTask{
		nameOfMethod: "RatingPicture",
		method: func() error {
			ratingResp, err := p.provider.RatingPicture(ctx, subject, picture)
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

func (p *rateLimitGeminiProvider) callProviderMethod(ctx context.Context, providersMethodTask callMethodTask) error {
	p.logger.Debug("start call method", slog.Any("method", providersMethodTask.nameOfMethod))
	select {
	case <-ctx.Done():
		{
			p.logger.Debug("context is done, returning from callProviderMethod")
			return ctx.Err()
		}
	case p.queueTaskChan <- struct{}{}:
		{
			defer p.tryStartNewRequestPeriod(ctx)
			return providersMethodTask.method()
		}
	}
}

// This method calls every time when calls one of provider`s method,
// because minute timer need start only after first request to gemini
func (p *rateLimitGeminiProvider) tryStartNewRequestPeriod(ctx context.Context) {
	p.mt.Lock()
	defer p.mt.Unlock()
	// Timer is nil means that period with requests end or haven`tn started yet
	// Thats why is inited new timer and start goroutine for handling ending request period
	if p.minuteTimer == nil {
		p.logger.Debug("start new timer")
		p.minuteTimer = time.NewTimer(time.Minute)

		go func() {
			p.logger.Debug("start goroutine for waiting timer end")
			select {
			case <-ctx.Done():
				{
					p.logger.Debug("context is done, end 'tryStartNewRequestPeriod' goroutine")
					return
				}
			case <-p.minuteTimer.C:
				{
					p.minuteTimer = nil
					p.logger.Debug("timer is done, cleaning queue")
					for range len(p.queueTaskChan) {
						<-p.queueTaskChan
					}

				}
			}
		}()
	} else {
		p.logger.Debug("timer is already started, skip initiation new one")
	}
}
