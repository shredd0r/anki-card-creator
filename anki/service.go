package anki

//go:generate mockgen -source service.go -destination mock/service_mock.go

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/npcnixel/genanki-go"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	model_id      = 10011001
	model_name    = "card-generator-v.1.0.0"
	template_name = "card-generator-template"
	default_tag   = "anki-card-creator"
)

type Service interface {
	AddFlashcard(ctx context.Context, flashcard *models.Flashcard)
	SavePackage(ctx context.Context, pathToFile string) error
}

type implService struct {
	logger *slog.Logger
	decks  map[string]*genanki.Deck
}

func NewService(logger *slog.Logger) Service {
	return &implService{
		logger: logger.WithGroup("anki-service"),
		decks:  map[string]*genanki.Deck{},
	}
}

// AddFlashcard - method for putting down flashcard in appropriate deck by deckname in flashcard
func (s *implService) AddFlashcard(ctx context.Context, flashcard *models.Flashcard) {
	s.logger.Debug("start adding flashcard to deck", slog.Any("deck", flashcard.DeckName), slog.Any("subject", flashcard.Subject))

	deck, ok := s.decks[flashcard.DeckName]
	if !ok {
		deck = genanki.NewDeck(s.getIdForDeck(), flashcard.DeckName, "")
		s.decks[flashcard.DeckName] = deck
	}

	pictureFilename := ""
	if flashcard.Picture != nil {
		pictureFilename = s.formatPictureField(flashcard.Picture.Filename)
		deck.AddMedia(flashcard.Picture.Filename, flashcard.Picture.Content)
	}

	pronunciationTag := ""
	if flashcard.Pronunciation != nil {
		pronunciationTag = flashcard.Pronunciation.Filename
		deck.AddMedia(flashcard.Pronunciation.Filename, flashcard.Pronunciation.Content)
	}

	transcription := ""
	if flashcard.Transcription != nil {
		transcription = *flashcard.Transcription
	}

	noteId := s.getIdForNote(deck)
	deck.AddNote(&genanki.Note{
		ID:      noteId,
		ModelID: model_id,
		Fields: []string{
			flashcard.Subject,
			pronunciationTag,
			transcription,
			flashcard.Paraphrase,
			s.formatSynonymsField(flashcard),
			pictureFilename,
			s.formatExampleField(flashcard),
		},
		Tags: append(flashcard.Tags, default_tag),
	})

	s.logger.Debug("added new note to deck", slog.Any("deck", deck.Name), slog.Any("note id", noteId))
}

// SavePackage - create the package with all decks from service and save it to file
func (s *implService) SavePackage(ctx context.Context, pathToFile string) error {
	sliceDecks := []*genanki.Deck{}
	for _, deck := range s.decks {
		sliceDecks = append(sliceDecks, deck)
	}

	pkg := genanki.NewPackage(sliceDecks).AddModel(&genanki.Model{
		ID:        model_id,
		Name:      model_name,
		Fields:    s.getFieldsForModel(),
		Templates: s.getTemplatesForModel(),
		CSS:       css,
	})

	return pkg.WriteToFile(pathToFile)

}

func (s *implService) getIdForDeck() int64 {
	return int64(len(s.decks) + 1)
}

func (s *implService) getIdForNote(deck *genanki.Deck) int64 {
	return int64(len(deck.Notes) + 1)
}

func (s *implService) getFieldsForModel() []genanki.Field {
	return []genanki.Field{
		{
			Name: "Subject",
			Ord:  0,
			Font: "Helvetica",
			Size: 30,
		},
		{
			Name: "Pronunciation",
			Ord:  1,
		},
		{
			Name: "Transcription",
			Ord:  2,
			Font: "Helvetica",
			Size: 20,
		},
		{
			Name: "Paraphrase",
			Ord:  3,
			Font: "Helvetica",
			Size: 20,
		},
		{
			Name: "Synonyms",
			Ord:  4,
			Font: "Helvetica",
			Size: 20,
		},
		{
			Name: "Picture",
			Ord:  5,
		},
		{
			Name: "Example",
			Ord:  6,
			Font: "Helvetica",
			Size: 18,
		},
	}
}

func (s *implService) getTemplatesForModel() []genanki.Template {
	return []genanki.Template{
		{
			Name: template_name,
			Ord:  0,
			Qfmt: front_side_template,
			Afmt: back_side_template,
		},
	}
}

// Flashcard in anki expect example string like that:
// <ul>
//
//	<li>The lime is sour.</li>
//	<li>We have a lime tree.</li>
//	<li>Cut the lime in half.</li>
//
// </ul>
func (s *implService) formatExampleField(flashcard *models.Flashcard) string {
	formatExamples := "<ul>%s</ul>"

	examples := ""
	for _, example := range flashcard.Examples {
		examples += fmt.Sprintf("<li>%s</li>", example)
	}

	s.logger.Debug("examples: ", slog.String("example", examples))

	return fmt.Sprintf(formatExamples, examples)
}

func (s *implService) formatPictureField(filename string) string {
	return fmt.Sprintf("<img src='%s'>", filename)
}

func (s *implService) formatSynonymsField(flashcard *models.Flashcard) string {
	return strings.Join(flashcard.Synonyms, ", ")
}
