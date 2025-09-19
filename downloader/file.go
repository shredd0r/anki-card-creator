package downloader

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"

	"github.com/shredd0r/anki-card-creator/models"
)

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

func (d *implFileDownloader) Download(ctx context.Context, urlToFile string) (*models.File, error) {
	resp, err := http.Get(urlToFile)
	if err != nil {
		d.logger.Error("failed download file", slog.Any("err", err.Error()))
		return nil, err
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		d.logger.Error("returned failed status code, when tried download file")
		return nil, http.ErrNotSupported
	}

	bufferReader := &bytes.Buffer{}

	_, err = io.Copy(bufferReader, resp.Body)
	if err != nil {
		d.logger.Error("failed copy file from response body to buffer", slog.Any("err", err.Error()))
		return nil, err
	}

	file := &models.File{
		Content: bufferReader,
		Type:    "",
	}

	return file, nil
}
