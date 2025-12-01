package anki

//go:generate mockgen -source service.go -destination mock/service_mock.go
//go:generate mockgen -destination mock/ankiconnect/ankiconnect_mock.go github.com/atselvan/ankiconnect MediaManager,DecksManager,NotesManager,ModelsManager

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/atselvan/ankiconnect"
	"github.com/shredd0r/anki-card-creator/models"
	"golang.org/x/sync/errgroup"
)

var (
	errMoreThanOneType      = errors.New("matched more than 1 content types")
	errCardAlreadyExist     = errors.New("card already exist")
	errTemplateNameNotExist = errors.New("template name for card not exist")
)

const query_for_get_notes = `"subject:%s" "deck:%s"`

type Service interface {
	AddTemplate(ctx context.Context) error
	StoreNewCard(ctx context.Context, templateName string, flashcard *models.Flashcard) error
	IsCardAlreadyExist(ctx context.Context, subject string, deckname string) (bool, error)
	HealthCheck(ctx context.Context) error
}

type implService struct {
	logger            *slog.Logger
	client            *ankiconnect.Client
	alreadyExistDecks map[string]bool
}

func NewService(logger *slog.Logger, client *ankiconnect.Client) Service {
	return &implService{
		logger:            logger.WithGroup("anki-service"),
		client:            client,
		alreadyExistDecks: map[string]bool{},
	}
}

// TODO add creation template in anki collection
func (s *implService) AddTemplate(ctx context.Context) error {
	panic("not implement")
}

func (s *implService) StoreNewCard(ctx context.Context, templateName string, flashcard *models.Flashcard) error {
	s.logger.Debug("start store new card")
	err := s.isTemplateExist(templateName)
	if err != nil {
		return err
	}

	isExist, err := s.IsCardAlreadyExist(ctx, flashcard.Subject, flashcard.DeckName)
	if err != nil {
		return err
	}
	if isExist {
		return errCardAlreadyExist
	}

	err = s.createDeckIfItNotExist(flashcard.DeckName)
	if err != nil {
		return nil
	}

	ankiCardFields := map[string]string{
		"Subject":      flashcard.Subject,
		"Paraphrase":   flashcard.Paraphrase,
		"Example":      s.formatExampleField(flashcard),
		"Has_Spelling": "1",
	}

	if flashcard.Transcription != nil {
		ankiCardFields["Transcription"] = *flashcard.Transcription
	}

	g := errgroup.Group{}
	fieldsMutex := sync.Mutex{}

	//  Store picture file in anki with goroutine if card has picture
	if flashcard.Picture != nil {
		g.Go(func() error {
			filename, err := s.storeMediafileAndReturnFilename(ctx, flashcard.Subject, flashcard.Picture)
			if err != nil {
				return err
			}
			fieldsMutex.Lock()
			ankiCardFields["Picture"] = s.formatPictureField(filename)
			fieldsMutex.Unlock()
			return nil
		})

	}

	//  Store picture file in anki with goroutine if card has pronunciation
	if flashcard.Pronunciation != nil {
		g.Go(func() error {
			filename, err := s.storeMediafileAndReturnFilename(ctx, flashcard.Subject, flashcard.Pronunciation)
			if err != nil {
				return err
			}
			fieldsMutex.Lock()
			ankiCardFields["Pronunciation"] = filename
			fieldsMutex.Unlock()
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		s.logger.Error("failed store file in anki", slog.Any("err", err.Error()))
		return err
	}

	restErr := s.client.Notes.Add(ankiconnect.Note{
		DeckName:  flashcard.DeckName,
		ModelName: templateName,
		Fields:    ankiCardFields,
	})

	if restErr != nil {
		s.logger.Error("failed add new card to anki", slog.Any("err", restErr.Message))
		return errors.New(restErr.Message)
	}

	return nil
}

func (s *implService) IsCardAlreadyExist(ctx context.Context, subject string, deckname string) (bool, error) {
	select {
	case <-ctx.Done():
		{
			s.logger.Debug("context is done, returning from IsCardAlreadyExist")
			return false, ctx.Err()
		}
	default:
		{
			s.logger.Debug("check if card already exist")
			query := fmt.Sprintf(query_for_get_notes, subject, deckname)
			resp, restErr := s.client.Notes.Get(query)

			if restErr != nil {
				s.logger.Error("failed get note from anki", slog.Any("err", restErr.Message))
				s.logger.Error("query for get note ", slog.Any("query", query))
				return false, errors.New(restErr.Message)
			}

			for _, r := range *resp {
				if strings.Compare(r.Fields["Subject"].Value, subject) == 0 {
					s.logger.Debug("card already exist", slog.Any("subject", subject))
					return true, nil
				}
			}
			return false, nil
		}
	}

}

func (s *implService) HealthCheck(ctx context.Context) error {
	s.logger.Debug("start healthcheck")
	restErr := s.client.Ping()
	if restErr != nil {
		s.logger.Error("connection with anki server not set")
		return errors.New(restErr.Message)
	}
	return nil
}

func (s *implService) storeMediafile(ctx context.Context, filename string, mediafile *models.File) error {
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

func (s *implService) makeMediafileName(subject string, mediafile *models.File) (string, error) {
	typeOfFile, err := s.getType(mediafile.MIMEType)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("_%s.%s", strings.ToLower(strings.ReplaceAll(subject, " ", "-")), typeOfFile), nil
}

func (s *implService) encodeMediaContent(mediafile *models.File) (*string, error) {
	s.logger.Debug("start encode media content")
	encodedMediaContent := base64.StdEncoding.EncodeToString(mediafile.Content)
	return &encodedMediaContent, nil
}

func (s *implService) isTemplateExist(templateName string) error {
	s.logger.Debug("check if template exist")
	_, restErr := s.client.Models.GetFields(templateName)
	if restErr != nil {
		// If models not found, server return error with message:
		// "model was not found: 'name-of-model'"
		if strings.Contains(restErr.Message, templateName) {
			s.logger.Debug("template note exist", slog.Any("templateName", templateName))
			return errTemplateNameNotExist
		}
		return errors.New(restErr.Message)
	}
	return nil
}

func (s *implService) createDeckIfItNotExist(deckname string) error {
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
func (s *implService) getType(MIMEType string) (string, error) {
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

func (s *implService) storeMediafileAndReturnFilename(ctx context.Context, subject string, mediaFile *models.File) (string, error) {
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
