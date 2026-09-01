package anki

//go:generate mockgen -source service.go -destination mock/service_mock.go

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math/rand"
	"strings"

	"github.com/npcnixel/genanki-go"
	"github.com/shredd0r/anki-card-creator/internal/version"
	"github.com/shredd0r/anki-card-creator/models"
)

var (
	model_name = fmt.Sprintf("card-generator-%s", version.Version)
)

const (
	model_id      = 10011001
	template_name = "card-generator-template"
	default_tag   = "anki-card-creator"
)

type Service interface {
	// AddFlashcard - method for putting down flashcard in appropriate deck by deckname in flashcard
	AddFlashcard(ctx context.Context, flashcard *models.Flashcard)
	// SavePackage - create the package with all decks from service and save it to file
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
	s.logger.Info("start adding flashcard to deck", slog.Any("deck", flashcard.DeckName), slog.Any("subject", flashcard.Subject))

	deck, ok := s.decks[flashcard.DeckName]
	if !ok {
		s.logger.Debug("Deckname not exist in cache, create", slog.Any("deckname", flashcard.DeckName))
		deck = genanki.NewDeck(s.getIdByString(flashcard.DeckName), flashcard.DeckName, "Deck created by anki card creator")
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

	noteId := s.getIdForNote(deck, flashcard.Subject)
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

func (s *implService) getIdForNote(deck *genanki.Deck, subject string) int64 {
	return s.getIdByString(fmt.Sprintf("%s,%s", deck.Name, subject))
}

func (s *implService) getIdByString(str string) int64 {
	h := fnv.New64a()
	h.Write([]byte(str))
	seed := int64(h.Sum64())

	return rand.NewSource(seed).Int63()
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
