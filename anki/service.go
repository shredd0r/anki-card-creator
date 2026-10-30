package anki

//go:generate mockgen -source service.go -destination mock/service_mock.go

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	model_name    = "card-generator"
	template_name = "card-generator-template"
	default_tag   = "anki-card-creator"
)

type Service interface {
	// AddFlashcard ensures the flashcard's deck exists, uploads any media,
	// and adds or updates (by Subject match within the deck) the note in
	// Anki via AnkiConnect.
	AddFlashcard(ctx context.Context, flashcard *models.Flashcard) error
	// ListDecks returns all deck names currently known to AnkiConnect.
	ListDecks(ctx context.Context) ([]string, error)
}

type implService struct {
	logger    *slog.Logger
	client    Client
	modelName string
	cfg       config.AnkiConnectConfig
	decks     map[string]struct{}
	fields    fieldFormatter
}

// NewService connects to AnkiConnect and ensures the note type (fields,
// templates and CSS from anki/const.go) exists and is up to date before
// returning a ready-to-use Service.
func NewService(ctx context.Context, logger *slog.Logger, cfg config.AnkiConnectConfig) (Service, error) {
	s := &implService{
		logger:    logger.WithGroup("anki-service"),
		client:    NewClient(newAnkiConnectHTTPClient(), cfg),
		modelName: model_name,
		cfg:       cfg,
		decks:     map[string]struct{}{},
	}

	if err := s.ensureNoteType(ctx); err != nil {
		return nil, fmt.Errorf("ensure anki note type: %w", err)
	}

	return s, nil
}

func (s *implService) AddFlashcard(ctx context.Context, flashcard *models.Flashcard) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	s.logger.Info("adding flashcard to deck", slog.Any("deck", flashcard.DeckName), slog.Any("subject", flashcard.Subject))

	if err := s.ensureDeck(ctx, flashcard.DeckName); err != nil {
		return fmt.Errorf("ensure deck %q: %w", flashcard.DeckName, err)
	}

	pictureField := ""
	if flashcard.Picture != nil {
		storedName, err := s.storeMedia(ctx, flashcard.Picture)
		if err != nil {
			return fmt.Errorf("store picture media: %w", err)
		}
		pictureField = s.fields.Picture(storedName)
	}

	pronunciationField := ""
	if flashcard.Pronunciation != nil {
		storedName, err := s.storeMedia(ctx, flashcard.Pronunciation)
		if err != nil {
			return fmt.Errorf("store pronunciation media: %w", err)
		}
		pronunciationField = storedName
	}

	transcription := ""
	if flashcard.Transcription != nil {
		transcription = *flashcard.Transcription
	}

	fields := map[string]string{
		"Subject":       flashcard.Subject,
		"Pronunciation": pronunciationField,
		"Transcription": transcription,
		"Paraphrase":    flashcard.Paraphrase,
		"Synonyms":      s.fields.Synonyms(flashcard.Synonyms),
		"Picture":       pictureField,
		"Example":       s.fields.Example(flashcard.Subject, flashcard.Examples),
	}
	tags := append(flashcard.Tags, default_tag)

	if err := s.upsertNote(ctx, flashcard.DeckName, flashcard.Subject, fields, tags); err != nil {
		return fmt.Errorf("upsert note %q in deck %q: %w", flashcard.Subject, flashcard.DeckName, err)
	}

	s.logger.Debug("added flashcard to deck", slog.Any("deck", flashcard.DeckName), slog.Any("subject", flashcard.Subject))
	return nil
}

func (s *implService) ListDecks(ctx context.Context) ([]string, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()

	var names []string
	if err := s.client.Invoke(ctx, "deckNames", nil, &names); err != nil {
		return nil, fmt.Errorf("list decks: %w", err)
	}
	return names, nil
}

func (s *implService) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.cfg.Timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.cfg.Timeout)
}

func (s *implService) ensureDeck(ctx context.Context, deckName string) error {
	if _, ok := s.decks[deckName]; ok {
		return nil
	}

	if err := s.client.Invoke(ctx, "createDeck", map[string]string{"deck": deckName}, nil); err != nil {
		return err
	}

	s.decks[deckName] = struct{}{}
	return nil
}

func (s *implService) storeMedia(ctx context.Context, file *models.File) (string, error) {
	var storedName string
	err := s.client.Invoke(ctx, "storeMediaFile", map[string]string{
		"filename": file.Filename,
		"data":     base64.StdEncoding.EncodeToString(file.Content),
	}, &storedName)
	if err != nil {
		return "", err
	}
	return storedName, nil
}

type addNoteParams struct {
	Note addNoteSpec `json:"note"`
}

type addNoteSpec struct {
	DeckName  string            `json:"deckName"`
	ModelName string            `json:"modelName"`
	Fields    map[string]string `json:"fields"`
	Tags      []string          `json:"tags"`
}

type updateNoteFieldsParams struct {
	Note updateNoteFieldsSpec `json:"note"`
}

type updateNoteFieldsSpec struct {
	ID     int64             `json:"id"`
	Fields map[string]string `json:"fields"`
}

type addTagsParams struct {
	Notes []int64 `json:"notes"`
	Tags  string  `json:"tags"`
}

func (s *implService) upsertNote(ctx context.Context, deckName string, subject string, fields map[string]string, tags []string) error {
	query := fmt.Sprintf("deck:%q Subject:%q", deckName, subject)

	var noteIds []int64
	if err := s.client.Invoke(ctx, "findNotes", map[string]string{"query": query}, &noteIds); err != nil {
		return fmt.Errorf("find existing note: %w", err)
	}

	if len(noteIds) > 0 {
		if err := s.client.Invoke(ctx, "updateNoteFields", updateNoteFieldsParams{
			Note: updateNoteFieldsSpec{ID: noteIds[0], Fields: fields},
		}, nil); err != nil {
			return fmt.Errorf("update note fields: %w", err)
		}

		if err := s.client.Invoke(ctx, "addTags", addTagsParams{
			Notes: noteIds,
			Tags:  strings.Join(tags, " "),
		}, nil); err != nil {
			return fmt.Errorf("update note tags: %w", err)
		}
		return nil
	}

	if err := s.client.Invoke(ctx, "addNote", addNoteParams{
		Note: addNoteSpec{
			DeckName:  deckName,
			ModelName: s.modelName,
			Fields:    fields,
			Tags:      tags,
		},
	}, nil); err != nil {
		return fmt.Errorf("add note: %w", err)
	}
	return nil
}
