package providers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	main_page                      = "https://www.google.com/imghp?hl=en"
	selector_for_matched_image     = "img[style*='object-position']"
	selector_for_detail_view_image = "a[role='link'] > img[jsaction='']:first-child"
)

var errIndexOutOfRange = errors.New("index out of range")

type GoogleImageProvider interface {
	Get(ctx context.Context, numOfPicture uint, searchQuery string) (*models.File, error)
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

// Get - method for getting picture from Google Image. Picture returns as buffer reader, not saving in disk
// numOfPicture - index for getting picture from list of matched pictures
// searchQuery - query for searching pictures
func (p *implGoogleImageProvider) Get(ctx context.Context, numOfPicture uint, searchQuery string) (*models.File, error) {
	page, err := p.moveToPageWithImages(searchQuery)
	if err != nil {
		return nil, err
	}

	// Waiting for when loading all matched pictures on page
	err = page.Locator(selector_for_matched_image).First().WaitFor()
	if err != nil {
		p.logger.Error("failed wait for loading images on page or no one image is matched", slog.Any("err", err.Error()))
		return nil, err
	}

	matchedImgLocators, err := page.Locator(selector_for_matched_image).All()
	if err != nil {
		p.logger.Error("failed get images from google page", slog.Any("err", err.Error()))
		return nil, err
	}

	if int(numOfPicture) > len(matchedImgLocators) {
		p.logger.Error("number of picture out of range array images")
		return nil, errIndexOutOfRange
	}

	// Click to matched picture for open detail view.
	// Detail view has source to picture for downloading.
	err = matchedImgLocators[numOfPicture].Click()
	if err != nil {
		p.logger.Error("failed click to searching image", slog.Any("err", err.Error()))
		return nil, err
	}

	imgViewLocator := page.Locator(selector_for_detail_view_image)
	imgUrl, err := imgViewLocator.GetAttribute("src")
	if err != nil {
		p.logger.Error("failed get url to image from image view locator", slog.Any("err", err.Error()))
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
