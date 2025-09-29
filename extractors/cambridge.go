package extractors

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/models"
)

var errInvalidStatusCode = errors.New("invalid status code")

const (
	base_url                             = "https://dictionary.cambridge.org"
	dictionary_url_page                  = "https://dictionary.cambridge.org/dictionary/english"
	selector_for_explain_cambridge       = "div[class*='def ddef']"
	selector_for_example_cambridge       = "div.examp.dexamp"
	selector_for_transcription_cambridge = "span[class='pron dpron']"
	selector_for_pronouns_cambridge      = "#audio2 > source:nth-child(3)"
)

type CambridgeCardExtractor interface {
	GetExplains(ctx context.Context, subject string) (*[]string, error)
	GetTransacription(ctx context.Context, subject string) (*string, error)
	GetCard(ctx context.Context, subject string) (*models.CambridgeCard, error)
}

type implCambridgeCardExtractor struct {
	logger         *slog.Logger
	browser        playwright.Browser
	fileDownloader downloader.FileDownloader
}

func NewCambridgeCardExtractor(logger *slog.Logger, browser playwright.Browser, fileDownloader downloader.FileDownloader) CambridgeCardExtractor {
	return &implCambridgeCardExtractor{
		logger:         logger,
		browser:        browser,
		fileDownloader: fileDownloader,
	}
}

func (e *implCambridgeCardExtractor) GetExplains(ctx context.Context, subject string) (*[]string, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetExplains'")
			return nil, ctx.Err()
		}
	default:
		{
			e.logger.Debug(fmt.Sprintf("start get card for subject: %s", subject))

			page, err := e.gotoSubjectPage(ctx, subject)
			if err != nil {
				return nil, err
			}

			mainPageLocator := page.Locator("html")
			return e.getExplains(mainPageLocator)
		}
	}
}

func (e *implCambridgeCardExtractor) GetTransacription(ctx context.Context, subject string) (*string, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'GetTransacription'")
			return nil, ctx.Err()
		}
	default:
		{
			e.logger.Debug(fmt.Sprintf("start get card for subject: %s", subject))

			page, err := e.gotoSubjectPage(ctx, subject)
			if err != nil {
				return nil, err
			}

			mainPageLocator := page.Locator("html")
			return e.getTransacription(mainPageLocator)
		}
	}
}

func (e *implCambridgeCardExtractor) GetCard(ctx context.Context, subject string) (*models.CambridgeCard, error) {
	e.logger.Debug(fmt.Sprintf("start get card for subject: %s", subject))

	page, err := e.gotoSubjectPage(ctx, subject)
	if err != nil {
		return nil, err
	}

	mainPageLocator := page.Locator("html")
	var pronouns *models.File
	var errPronouns error
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		pronouns, errPronouns = e.getPronouns(ctx, mainPageLocator)
	}()

	subjectType := getSubjectType(subject)

	transcription, err := e.getTransacription(mainPageLocator)
	if err != nil {
		e.logger.Error("failed get transcription from parent locator", slog.Any("err", err.Error()))
		return nil, err
	}

	explains, err := e.getExplains(mainPageLocator)
	if err != nil {
		e.logger.Error("failed get explains from parent locator", slog.Any("err", err.Error()))
		return nil, err
	}

	examples, err := e.getExamples(mainPageLocator)
	if err != nil {
		e.logger.Error("failed get examples from parent locator", slog.Any("err", err.Error()))
		return nil, err
	}

	wg.Wait()
	if errPronouns != nil {
		return nil, err
	}

	return &models.CambridgeCard{
		Subject:       subject,
		SubjectType:   subjectType,
		Transcription: transcription,
		Pronouns:      pronouns,
		Explains:      *explains,
		Examples:      *examples,
	}, nil
}

func (e *implCambridgeCardExtractor) gotoSubjectPage(ctx context.Context, subject string) (playwright.Page, error) {
	select {
	case <-ctx.Done():
		{
			e.logger.Debug("context is done, leaving from 'gotoSubjectPage'")
			return nil, ctx.Err()
		}
	default:
		{
			e.logger.Debug(fmt.Sprintf("start go to dictionary page for subject: %s", subject))
			page, err := e.browser.NewPage()
			if err != nil {
				e.logger.Error("failed create new page for cambridge dictionary", slog.Any("err", err.Error()))
				return nil, err
			}

			subjectUrl, err := url.JoinPath(dictionary_url_page, subject)
			if err != nil {
				e.logger.Error("failed join subjec to dictionary url", slog.Any("err", err.Error()))
				return nil, err
			}
			response, err := page.Goto(subjectUrl)
			if err != nil {
				e.logger.Error("failed go to subject page", slog.Any("err", err.Error()))
				return nil, err
			}
			if !response.Ok() {
				e.logger.Error("call subject url return invalid http status code, stopped")
				return nil, errInvalidStatusCode
			}
			return page, nil
		}
	}
}

func (e *implCambridgeCardExtractor) getTransacription(mainPageLocator playwright.Locator) (*string, error) {
	e.logger.Debug("start get transcription from cambridge page")
	transcription, err := getInnerTextFromChild(e.logger, mainPageLocator, selector_for_transcription_cambridge)
	if err != nil {
		e.logger.Error("failed get transcriptinf from parent locator", slog.Any("err", err.Error()))
		return nil, err
	}

	return transcription, nil
}

func (e *implCambridgeCardExtractor) getExplains(mainPageLocator playwright.Locator) (*[]string, error) {
	e.logger.Debug("start get explains from cambridge page")
	return e.getAllStringsBySelector(mainPageLocator, selector_for_explain_cambridge, 3)
}

func (e *implCambridgeCardExtractor) getExamples(mainPageLocator playwright.Locator) (*[]string, error) {
	e.logger.Debug("start get examples from cambridge page")
	return e.getAllStringsBySelector(mainPageLocator, selector_for_example_cambridge, 5)
}

// getAllStringsBySelector return all inner text from found locators by selector
func (e *implCambridgeCardExtractor) getAllStringsBySelector(mainPageLocator playwright.Locator, selector string, maxCount uint8) (*[]string, error) {
	stringLocators, err := mainPageLocator.Locator(selector).All()
	if err != nil {
		e.logger.Error("failed get locators from main page", slog.Any("err", err.Error()))
		return nil, err
	}

	countOfStrings := maxCount

	if len(stringLocators) < int(maxCount) {
		countOfStrings = uint8(len(stringLocators))
	}

	arrOfStr := make([]string, countOfStrings)

	for index := range countOfStrings {
		str, err := stringLocators[index].InnerText()
		if err != nil {
			e.logger.Error("failed get from parent locator", slog.Any("err", err.Error()))
			return nil, err
		}
		arrOfStr[index] = str
	}

	return &arrOfStr, nil
}

func (e *implCambridgeCardExtractor) getPronouns(ctx context.Context, mainPageLocator playwright.Locator) (*models.File, error) {
	pronounsFileLocator := mainPageLocator.Locator(selector_for_pronouns_cambridge)
	pronounsFilePath, err := pronounsFileLocator.GetAttribute("src")
	if err != nil {
		e.logger.Error("failed get pronouns file from notes", slog.Any("err", err.Error()))
		return nil, err
	}

	// Prononcations Audio files is placed on two ways:
	// - amazon s3
	// - cambridge dictionary servers
	// If file is placed on cambridge dictionary servers, path from src doenst have protol and domain
	var pronounsFileUrl string
	if strings.Contains(pronounsFilePath, "https://s3") {
		pronounsFileUrl = pronounsFilePath
	} else {
		pronounsFileUrl, err = url.JoinPath(base_url, pronounsFilePath)
		if err != nil {
			e.logger.Error("failed make url for download file", slog.Any("err", err.Error()))
			return nil, err
		}
	}

	return e.fileDownloader.Download(ctx, pronounsFileUrl)
}
