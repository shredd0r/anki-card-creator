package downloader

//go:generate mockgen -source file.go -destination mock/file_mock.go

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/shredd0r/anki-card-creator/models"
)

var errMoreThanOneType = errors.New("matched more than 1 content types")

type File interface {
	Download(ctx context.Context, nameWithoutType string, urlToFile string) (*models.File, error)
}

type implFile struct {
	logger *slog.Logger
}

func NewFile(logger *slog.Logger) File {
	return &implFile{
		logger: logger.WithGroup("file-downloader"),
	}
}

// Download - method for downloading file by url, using http requests.
func (d *implFile) Download(ctx context.Context, nameWithoutType string, urlToFile string) (*models.File, error) {
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

	fileContent, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("failed copy file from response body to buffer", slog.Any("err", err.Error()))
		return nil, err
	}

	mimeType := d.getMIMEType(resp.Header)
	fileType, err := d.getFileType(mimeType)
	if err != nil {
		return nil, err
	}

	file := &models.File{
		Filename: fmt.Sprintf("%s.%s", strings.ToLower(strings.ReplaceAll(nameWithoutType, " ", "-")), fileType),
		Content:  fileContent,
		MIMEType: d.getMIMEType(resp.Header),
	}

	return file, nil
}

func (d *implFile) getMIMEType(responseHeader http.Header) string {
	d.logger.Debug("start getting type from content-type")
	contentType := responseHeader.Get("content-type")
	d.logger.Debug("received content type", slog.Any("content-type", contentType))

	return contentType
}

func (d *implFile) getFileType(MIMEType string) (string, error) {
	d.logger.Debug("start getting type from MIMEType")

	// I use regex, because if image is svg, content-type is 'image/svg+html'
	// I need just type of file, in this case - 'svg'
	regex, err := regexp.Compile(`\/([a-z\/]*)`)
	if err != nil {
		d.logger.Error("failed create regex by expression", slog.Any("err", err.Error()))
		return "", err
	}

	typeOfFiles := regex.FindStringSubmatch(MIMEType)
	if len(typeOfFiles) != 2 {
		d.logger.Error("regex matched more than 1 types")
		return "", errMoreThanOneType
	}

	// The first element is full match with symbol '/': /svg
	// The second element is capture group: svg
	typeOfFile := typeOfFiles[1]
	d.logger.Debug("type of file", slog.Any("type", typeOfFile))

	return typeOfFile, nil
}
