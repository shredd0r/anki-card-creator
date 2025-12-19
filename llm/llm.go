package llm

//go:generate mockgen -source llm.go -destination mock/llm_mock.go

import (
	"context"
	"encoding/json"
	"errors"

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
Result have to be concise, structured for easy reading.

EXAMPLE INPUT:
subject: row; using context: tables
		
EXAMPLE JSON OUTPUT
{
	"paraphrase": "Arrange entries side‑by‑side across the table.",
	"examples": [
		"Insert a new <b>row</b> below the header to add additional data.",
		"The first <b>row</b> in the sheet lists the product names.",
		"Delete the last <b>row</b> in the table to remove outdated information.",
		"To view all details, scroll down through the successive <b>rows</b> of the report."
	],
	"synonyms": [
		"line",
		"tier",
		"track",
		"rank",
		sequence"
	]
}"`
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
	Paraphrase    string   `json:"paraphrase"`
	Transcription *string  `json:"transcription"`
	Examples      []string `json:"examples"`
	Synonyms      []string `json:"synonyms"`
}

type ratingPictureResponse struct {
	Rating   uint8
	Analysis string
}

type promptRequest struct {
	Subject      string    `json:"subject"`
	UsingContext *[]string `json:"using-context,omitempty"`
}

type Provider interface {
	GenerateCardContent(ctx context.Context, subject string, usingContext *[]string) (*GeneratedCardContent, error)
	RatePicture(ctx context.Context, subject string, picture *models.File) (*uint8, error)
	HealthCheck(ctx context.Context) error
}

func getPromptRequestBy(subject string, usingContext *[]string) string {
	request := promptRequest{
		Subject:      subject,
		UsingContext: usingContext,
	}
	bytes, _ := json.Marshal(&request)
	return string(bytes)
}
