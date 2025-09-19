package provider

import (
	"context"
	"errors"
	"log/slog"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/models"
)

const main_page = "https://www.google.com/imghp?hl=en"

var errIndexOutOfRange = errors.New("index out of range")

type GoogleImageProvider interface {
	Get(ctx context.Context, numOfPicture uint, search string) (*models.File, error)
}

func NewGoogleImageProvider(logger *slog.Logger, browser playwright.Browser, fileDownloader downloader.FileDownloader) GoogleImageProvider {
	return &implGoogleImageProvider{
		logger:         logger.WithGroup("google-image-provider"),
		browser:        browser,
		fileDownloader: fileDownloader,
	}
}

type implGoogleImageProvider struct {
	logger         *slog.Logger
	browser        playwright.Browser
	fileDownloader downloader.FileDownloader
}

func (p *implGoogleImageProvider) Get(ctx context.Context, numOfPicture uint, search string) (*models.File, error) {
	page, err := p.moveToPageWithImages(search)
	if err != nil {
		return nil, err
	}

	page.Locator("g-img > img").WaitFor()
	imgLocators, err := page.Locator("g-img > img").All()
	if err != nil {
		p.logger.Error("failed get images from google page", slog.Any("err", err.Error()))
		return nil, err
	}

	if int(numOfPicture) > len(imgLocators) {
		p.logger.Error("number of picture out of range array images")
		return nil, errIndexOutOfRange
	}

	imgUrl, err := imgLocators[numOfPicture].GetAttribute("src")
	if err != nil {
		p.logger.Error("failed get attribute from image locator", slog.Any("err", err.Error()))
		return nil, err
	}
	return p.fileDownloader.Download(ctx, imgUrl)
}

func (p *implGoogleImageProvider) moveToPageWithImages(search string) (playwright.Page, error) {
	page, err := p.browser.NewPage()
	if err != nil {
		p.logger.Error("failed create new page", slog.Any("err", err.Error()))
		return nil, err
	}

	_, err = page.Goto(main_page)
	if err != nil {
		p.logger.Error("failed go to main page", slog.Any("err", err.Error()))
		return nil, err
	}

	err = page.Locator("textarea").First().Fill(search)
	if err != nil {
		p.logger.Error("failed fill text to textarea", slog.Any("err", err.Error()))
		return nil, err
	}

	// This css selector returned 2 input, where needed input is second
	inputLocators, err := page.Locator("input[value='Google Search']").All()
	if err != nil {
		p.logger.Error("failed get input from page", slog.Any("err", err.Error()))
		return nil, err
	}

	err = inputLocators[1].Click()
	if err != nil {
		p.logger.Error("failed click 'Google Search'", slog.Any("err", err.Error()))
		return nil, err
	}

	return page, nil
}
