package downloader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/shredd0r/anki-card-creator/models"
)

var errMoreThanOneType = errors.New("matched more than 1 content types")

type FileDownloader interface {
	Download(ctx context.Context, urlToFile string) (*models.File, error)
}

type implFileDownloader struct {
	logger *slog.Logger
}

func NewFileDownloader(logger *slog.Logger) FileDownloader {
	return &implFileDownloader{
		logger: logger.WithGroup("file-downloader"),
	}
}

// Download - method for downloading file by url, using http requests.
func (d *implFileDownloader) Download(ctx context.Context, urlToFile string) (*models.File, error) {
	d.logger.Debug("start download file", slog.Any("url", urlToFile))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToFile, nil)
	if err != nil {
		d.logger.Error("failed create request object", slog.Any("err", err.Error()))
		return nil, err
	}

	// Put user agent for give access to downloading from some sources (for example Wikipedia)
	request.Header.Add("User-Agent", "flashcard-creator/v0.0.1 (https://flashcard-creator.org; flashcard-creator@card.org)")

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		d.logger.Error("failed download file", slog.Any("err", err.Error()))
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("returned failed status code, when tried download file", slog.Any("status", resp.Status))
		return nil, http.ErrNotSupported
	}

	bufferReader := &bytes.Buffer{}
	_, err = io.Copy(bufferReader, resp.Body)
	if err != nil {
		d.logger.Error("failed copy file from response body to buffer", slog.Any("err", err.Error()))
		return nil, err
	}

	typeOfFile, err := d.getType(resp.Header)
	if err != nil {
		return nil, err
	}
	file := &models.File{
		Content: bufferReader,
		Type:    *typeOfFile,
	}

	return file, nil
}

func (d *implFileDownloader) getType(responseHeader http.Header) (*string, error) {
	d.logger.Debug("start getting type from content-type")
	contentType := responseHeader.Get("content-type")
	d.logger.Debug("received content type", slog.Any("content-type", contentType))

	// I use regex, because if image is svg, content-type is 'image/svg+html'
	// I need just type of file, in this case - 'svg'
	regex, err := regexp.Compile(`\/([a-z\/]*)`)
	if err != nil {
		d.logger.Error("failed create regex by expression", slog.Any("err", err.Error()))
		return nil, err
	}

	typeOfFiles := regex.FindStringSubmatch(contentType)
	if len(typeOfFiles) != 2 {
		d.logger.Error("regex matched more than 1 types")
		return nil, errMoreThanOneType
	}

	// The first element is full match with symbol '/': /svg
	// The second element is capture group: svg
	typeOfFile := typeOfFiles[1]
	d.logger.Debug("type of file", slog.Any("type", typeOfFile))

	return &typeOfFile, nil
}
