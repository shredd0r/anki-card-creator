package downloader

//go:generate mockgen -source file.go -destination mock/file_mock.go

import (
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/shredd0r/anki-card-creator/models"
)

type File interface {
	Download(ctx context.Context, urlToFile string) (*models.File, error)
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
func (d *implFile) Download(ctx context.Context, urlToFile string) (*models.File, error) {
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

	file := &models.File{
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
