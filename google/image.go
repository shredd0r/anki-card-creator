package google

//go:generate mockgen -source image.go -destination mock/image_mock.go

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/mxschmitt/playwright-go"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	main_page                                     = "https://www.google.com/imghp?hl=en"
	image_search_url                              = "https://www.google.com/search?q=%s&udm=2"
	selector_for_matched_image                    = "img[style*='object-position']"
	selector_for_detail_view_image                = "a[role='link'] > img[jsaction]:first-child"
	selector_for_detail_view_image_for_some_cases = "a[role='link'] > img[jsname]:first-child"
	selector_for_button_search                    = "input[value='Google Search']"
)

var errIndexOutOfRange = errors.New("index out of range")

// Image - interface for access to make request to searching image by query
type Image interface {
	// Request - create struct where you can sort through images on page
	Request(ctx context.Context, searchQuery string) (Result, error)
}

type Result interface {
	// Get - return image from opened web page
	Get(ctx context.Context, filenameWithoutType string, numOfPicture uint) (*models.File, error)
}

func NewImageProvider(logger *slog.Logger, browser playwright.Browser, fileDownloader downloader.File) Image {
	return &implImage{
		logger:         logger.WithGroup("google-image-provider"),
		browser:        browser,
		fileDownloader: fileDownloader,
	}
}

type implImage struct {
	logger         *slog.Logger
	browser        playwright.Browser
	fileDownloader downloader.File
}

type implResult struct {
	logger         *slog.Logger
	page           playwright.Page
	fileDownloader downloader.File
}

func (p *implImage) Request(ctx context.Context, searchQuery string) (Result, error) {
	page, err := p.moveToPageWithImages(searchQuery)
	if err != nil {
		return nil, err
	}

	return &implResult{
		logger:         p.logger,
		page:           page,
		fileDownloader: p.fileDownloader,
	}, nil
}

// Get - method for getting picture from Google Image. Picture returns as buffer reader, not saving in disk
// numOfPicture - index for getting picture from list of matched pictures
// searchQuery - query for searching pictures
func (p *implResult) Get(ctx context.Context, filenameWithoutType string, numOfPicture uint) (*models.File, error) {
	defer func() {
		err := p.page.Close()
		if err != nil {
			p.logger.Error("failed close page with google image site", slog.Any("err", err.Error()))
		}

	}()

	matchedImgLocators, err := p.page.Locator(selector_for_matched_image).All()
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

	var imgUrl string
	imgUrl, err = p.getUrlToImage(p.page, selector_for_detail_view_image)
	if err != nil {
		// In some cases, css selector 'selector_for_detail_view_image' doesn't return detail view image
		// If returned timeout err, workflow will try get url using another selector
		if errors.Is(err, playwright.ErrTimeout) {
			imgUrl, err = p.getUrlToImage(p.page, selector_for_detail_view_image_for_some_cases)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return p.fileDownloader.Download(ctx, filenameWithoutType, imgUrl)
}

func (p *implImage) moveToPageWithImages(search string) (playwright.Page, error) {
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

	// This selector return more than one locator.
	// I dont know why and when but structure of web page changed
	// Thats why I use simple selector, but this selector return more than one locators
	inputLocators, err := page.Locator(selector_for_button_search).All()
	if err != nil {
		p.logger.Error("failed get locators by selector for button")
		return nil, err
	}

	// Click every found button
	wg := sync.WaitGroup{}
	for _, inputLocator := range inputLocators {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := inputLocator.Click(playwright.LocatorClickOptions{
				Timeout: playwright.Float(5000),
			})
			if err != nil {
				p.logger.Debug("failed click to input", slog.Any("err", err))
			}
		}()
	}

	// Waiting for end all goroutines
	wg.Wait()

	// Waiting for loading all matched pictures on page
	err = page.Locator(selector_for_matched_image).First().WaitFor()
	if err != nil {
		p.logger.Error("failed wait for loading images on page or no one image is matched", slog.Any("err", err.Error()))
		return nil, err
	}
	return page, nil
}

func (p *implResult) getUrlToImage(page playwright.Page, selectorToDetailView string) (string, error) {
	imgViewLocator := page.Locator(selectorToDetailView).First()
	imgUrl, err := imgViewLocator.GetAttribute("src")
	if err != nil {
		p.logger.Error("failed get url to image from image view locator", slog.Any("err", err.Error()))
		return "", err
	}

	return imgUrl, nil
}
