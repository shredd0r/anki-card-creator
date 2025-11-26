package llm

//go:generate mockgen -source llm.go -destination mock/llm_mock.go

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shredd0r/anki-card-creator/models"
)

const (
	systemInstructionForCardContent = `
		I send you word or phrase or idiom, you should generate paraphrase, example of using and synonyms for this subject.
		In examples you must highlighted in bold subject with html tag like: <b>{subject}</b>, 
		If this subject changed structure (irregular word or word ending) this also have to be highlighted, for example: apple -> <b>apples</b>
		Maximum count of generated examples and synonyms have to be 5.
		Minimum count of generated examples and synonyms have to be 3.
		Paraphrase mustn't has this word, phrase, idiom. 
		Result have to be concise, structured for easy reading.`
	systemInstructionForRatePicture = `
		You have to analyze this picture. 
		Picture will be use for flashcard, the picture have to describe word, idiom, phrase. 
		Word, idion or phrase can't be written on picture. 
		You should give me rating between 1 - 10, where 10 its best match.`
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

type ratingPictureResponse struct {
	Rating   uint
	Analysis string
}

type Provider interface {
	GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error)
	RatePicture(ctx context.Context, subject string, picture *models.File) (*uint, error)
}

func getPromptRequestBy(subject string, usingContext *[]string) string {
	if usingContext != nil {
		return fmt.Sprintf("{'subject': '%s', 'using-context': '%s'}", subject, strings.Join(*usingContext, ", "))
	}
	return fmt.Sprintf("{'subject': '%s'}", subject)
}
