package duckduckgo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/mxschmitt/playwright-go"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/imagesearch"
	"github.com/shredd0r/anki-card-creator/models"
)

const (
	image_search_url               = "https://duckduckgo.com/?q=%s&iax=images&ia=images"
	selector_for_matched_image     = "figure>div>img"
	selector_for_detail_view_image = "a>div>img"
)

var errIndexOutOfRange = errors.New("index out of range")

func NewImage(logger *slog.Logger, browser playwright.Browser, fileDownloader downloader.File) imagesearch.Image {
	return &implImage{
		logger:         logger.WithGroup("duckduckgo-image-provider"),
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

func (p *implImage) Request(ctx context.Context, searchQuery string) (imagesearch.Result, error) {
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

// Get - method for getting picture from DuckDuckGo Images. Picture returns as buffer reader, not saving in disk
// numOfPicture - index for getting picture from list of matched pictures
func (p *implResult) Get(ctx context.Context, filenameWithoutType string, numOfPicture uint) (*models.File, error) {
	matchedImgLocators, err := p.page.Locator(selector_for_matched_image).All()
	if err != nil {
		p.logger.Error("failed get images from duckduckgo page", slog.Any("err", err.Error()))
		return nil, err
	}

	p.logger.Debug("count of matched images", slog.Any("count", len(matchedImgLocators)))

	if int(numOfPicture) >= len(matchedImgLocators) {
		p.logger.Error("number of picture out of range array images")
		return nil, errIndexOutOfRange
	}

	// Clicking a thumbnail opens DuckDuckGo's detail panel, which renders the
	// original (non-thumbnail) image - that's the URL we actually want to
	// download, not the low-res thumbnail src.
	if err := matchedImgLocators[numOfPicture].Click(); err != nil {
		p.logger.Error("failed click to searching image", slog.Any("err", err.Error()))
		return nil, err
	}

	imgUrl, err := p.getUrlToImage(p.page)
	if err != nil {
		return nil, err
	}

	return p.fileDownloader.Download(ctx, filenameWithoutType, imgUrl)
}

func (p *implResult) Close() error {
	if err := p.page.Close(); err != nil {
		p.logger.Error("failed close page with duckduckgo image site", slog.Any("err", err.Error()))
		return err
	}

	return nil
}

func (p *implImage) moveToPageWithImages(search string) (playwright.Page, error) {
	page, err := p.browser.NewPage()
	if err != nil {
		p.logger.Error("failed create new page", slog.Any("err", err.Error()))
		return nil, err
	}

	searchUrl := fmt.Sprintf(image_search_url, url.QueryEscape(search))
	if _, err := page.Goto(searchUrl); err != nil {
		p.logger.Error("failed go to duckduckgo images page", slog.Any("err", err.Error()))
		return nil, err
	}

	// Waiting for loading all matched pictures on page - the results grid
	// loads asynchronously via JS after navigation.
	if err := page.Locator(selector_for_matched_image).First().WaitFor(); err != nil {
		p.logger.Error("failed wait for loading images on page or no one image is matched", slog.Any("err", err.Error()))
		return nil, err
	}

	return page, nil
}

func (p *implResult) getUrlToImage(page playwright.Page) (string, error) {
	imgViewLocator := page.Locator(selector_for_detail_view_image).First()
	imgUrlWithoutProtocol, err := imgViewLocator.GetAttribute("src")
	if err != nil {
		p.logger.Error("failed get url to image from image detail locator", slog.Any("err", err.Error()))
		return "", err
	}

	imgUrl := fmt.Sprintf("https:%s", imgUrlWithoutProtocol)

	return imgUrl, nil
}
