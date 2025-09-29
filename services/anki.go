package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/atselvan/ankiconnect"
	"github.com/shredd0r/anki-card-creator/models"
	"golang.org/x/sync/errgroup"
)

var errMoreThanOneType = errors.New("matched more than 1 content types")

type AnkiService interface {
	StoreNewCard(ctx context.Context, deckname string, templateName string, flashcard models.Flashcard) error
}

type implAnkiService struct {
	logger            *slog.Logger
	client            *ankiconnect.Client
	alreadyExistDecks map[string]bool
}

func NewAnkiService(logger *slog.Logger, client *ankiconnect.Client) AnkiService {
	return &implAnkiService{
		logger:            logger.WithGroup("anki-service"),
		client:            client,
		alreadyExistDecks: map[string]bool{},
	}
}

// TODO add processing nil picture, pronounce files, transcription
// TODO add checking if card already exist in deckname
func (s *implAnkiService) StoreNewCard(ctx context.Context, deckname string, templateName string, flashcard models.Flashcard) error {
	err := s.createDeckIfItNotExist(deckname)
	if err != nil {
		return nil
	}

	g := errgroup.Group{}
	var filenameOfPronounce string
	var filenameOfPicture string

	// TODO before start store, need check if file not nil
	// Start store pronounce file in anki with goroutine
	g.Go(func() error {
		filename, err := s.storeMediafileAndReturnFilename(ctx, flashcard.Subject, flashcard.Pronouns)
		if err != nil {
			return err
		}
		filenameOfPronounce = filename
		return nil
	})

	// TODO before start store, need check if file not nil
	// Start store picture file in anki with goroutine
	g.Go(func() error {
		filename, err := s.storeMediafileAndReturnFilename(ctx, flashcard.Subject, flashcard.Picture)
		if err != nil {
			return err
		}
		filenameOfPicture = filename
		return nil
	})

	err = g.Wait()
	if err != nil {
		s.logger.Error("failed store file in anki", slog.Any("err", err.Error()))
		return err
	}

	ankiCardFields := map[string]string{
		"Subject":      flashcard.Subject,
		"Paraphrase":   flashcard.Explain,
		"Example":      flashcard.Examples[0],
		"Has_Spelling": "1",

		// TODO Before add it, need check, this fields is not nil
		"Transcription": *flashcard.Transcription,
		"Pronunciation": filenameOfPronounce,
		"Picture":       filenameOfPicture,
	}

	restErr := s.client.Notes.Add(ankiconnect.Note{
		DeckName:  deckname,
		ModelName: templateName,
		Fields:    ankiCardFields,
	})

	if restErr != nil {
		s.logger.Error("failed add new card to anki", slog.Any("err", restErr.Message))
		return errors.New(restErr.Message)
	}

	return nil
}

func (s *implAnkiService) storeMediafile(ctx context.Context, filename string, mediafile *models.File) error {
	s.logger.Debug("start store mediafile to anki")

	select {
	case <-ctx.Done():
		{
			s.logger.Info("context is done, returning from 'storeMediafile'")
			return ctx.Err()
		}
	default:
		{
			encodedMediaContent, err := s.encodeMediaContent(mediafile)
			if err != nil {
				return err
			}
			_, restErr := s.client.Media.StoreMediaFile(filename, *encodedMediaContent)
			if restErr != nil {
				s.logger.Error("failed store mediafile to anki", slog.Any("err", restErr.Error))
				return errors.New(restErr.Message)
			}

			return nil
		}
	}
}

func (s *implAnkiService) makeMediafileName(subject string, mediafile *models.File) (string, error) {
	typeOfFile, err := s.getType(mediafile.MIMEType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("_%s.%s", strings.ToLower(strings.ReplaceAll(subject, " ", "-")), typeOfFile), nil
}

func (s *implAnkiService) encodeMediaContent(mediafile *models.File) (*string, error) {
	s.logger.Debug("start encode media content")
	encodedMediaContent := base64.StdEncoding.EncodeToString(mediafile.Content)
	return &encodedMediaContent, nil
}

func (s *implAnkiService) createDeckIfItNotExist(deckname string) error {
	// Check, if deckname is cached
	if _, ok := s.alreadyExistDecks[deckname]; !ok {
		s.logger.Debug(fmt.Sprintf("%s isn't cached, start checking deck in anki", deckname))
		// If not, check deck exist in anki
		existDecks, restErr := s.client.Decks.GetAll()
		if restErr != nil {
			s.logger.Error("failed get exist decks from anki", slog.Any("err", restErr.Error))
			return errors.New(restErr.Error)
		}

		if slices.Contains(*existDecks, deckname) {
			s.logger.Debug(fmt.Sprintf("deck '%s' found in anki decks, add it in cache", deckname))
			s.alreadyExistDecks[deckname] = true
		} else {
			s.logger.Debug(fmt.Sprintf("deck '%s' not found in anki decs, create it", deckname))
			restErr := s.client.Decks.Create(deckname)
			if restErr != nil {
				s.logger.Error("failed create new deck in anki", slog.Any("err", restErr.Error))
				return errors.New(restErr.Error)
			}
		}
	}
	return nil
}

// TODO move it to another object, because it isnt task AnkiService
func (s *implAnkiService) getType(MIMEType string) (string, error) {
	s.logger.Debug("start getting type from MIMEType")

	// I use regex, because if image is svg, content-type is 'image/svg+html'
	// I need just type of file, in this case - 'svg'
	regex, err := regexp.Compile(`\/([a-z\/]*)`)
	if err != nil {
		s.logger.Error("failed create regex by expression", slog.Any("err", err.Error()))
		return "", err
	}

	typeOfFiles := regex.FindStringSubmatch(MIMEType)
	if len(typeOfFiles) != 2 {
		s.logger.Error("regex matched more than 1 types")
		return "", errMoreThanOneType
	}

	// The first element is full match with symbol '/': /svg
	// The second element is capture group: svg
	typeOfFile := typeOfFiles[1]
	s.logger.Debug("type of file", slog.Any("type", typeOfFile))

	return typeOfFile, nil
}

func (s *implAnkiService) storeMediafileAndReturnFilename(ctx context.Context, subject string, mediaFile *models.File) (string, error) {
	filename, err := s.makeMediafileName(subject, mediaFile)
	if err != nil {
		return "", err
	}
	s.logger.Debug("start store file", slog.Any("filename", filename))
	err = s.storeMediafile(ctx, filename, mediaFile)

	if err != nil {
		s.logger.Error("failed store media file", slog.Any("filename", filename))
		return "", err
	}

	return filename, nil
}
