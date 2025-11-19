package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/shredd0r/anki-card-creator/models"
	"google.golang.org/genai"
)

var notSupportedMIMEType = errors.New("unsupported MIME type")

const (
	model_for_generate_text = "gemini-2.5-flash" //free model
	rpm                     = 10                 //request per minute
)

type ratingPictureResponse struct {
	Rating   uint
	Analysis string
}

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
			SystemInstruction: genai.NewContentFromText(
				`I send you word or phrase or idiom, you should generate paraphrase, example of using and synonyms for this subject.
				In examples you must highlighted in bold subject with html tag like: <b>{subject}</b>.
				Max count of generated examples and synonyms have to be 5.
				Min count of generated examples and synonyms have to be 3.
				Paraphrase mustn't has this word, phrase, idiom. 
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
		genai.Text(p.getContentTextForRequest(subject, usingContext)),
		p.cfgForGenerateContentCard,
	)

	if err != nil {
		p.logger.Error(fmt.Sprintf("failed generate content for subject: %s", subject), slog.Any("err", err.Error()))
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

// RatingPicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// I am using rating picture approach, because free gemini api access allows send image. Generation is not allowed.
func (p *implGemini) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	p.logger.Debug("start method 'RatingPicture'")

	// skip picture with mimetype svg, because this type not supported gemini server
	if picture.MIMEType == "image/svg+xml" {
		return nil, notSupportedMIMEType
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

func (p *implGemini) wrapError(err error) error {
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
			return errServerIsOverload
		}
	default:
		return err
	}
}

func (p *implGemini) getContentTextForRequest(subject string, usingContext *[]string) string {
	if usingContext != nil {
		return fmt.Sprintf("{'subject': '%s', 'using-context': '%s'}", subject, strings.Join(*usingContext, ", "))
	}
	return fmt.Sprintf("{'subject': '%s'}", subject)
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

func NewRateLimitGemini(ctx context.Context, logger *slog.Logger, client *genai.Client) Provider {
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

// RatingPicture - method for send request to gemini backend, which checking how suitable is this picture for describe the subject.
// Calling this method start process for calculate requesting per minutes for all calls to gemini backend
func (p *rateLimitGemini) RatingPicture(ctx context.Context, subject string, picture *models.File) (*uint, error) {
	var rating *uint

	generateExplainTask := callMethodTask{
		nameOfMethod: "RatingPicture",
		method: func() error {
			ratingResp, err := p.geminiProvider.RatingPicture(ctx, subject, picture)
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
