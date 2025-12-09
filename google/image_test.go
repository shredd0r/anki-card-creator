package google

import (
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/browser"
	mock_downloader "github.com/shredd0r/anki-card-creator/downloader/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestPositiveGetFile(t *testing.T) {
	filename := "test-filename"
	mockDownloader, googleImageProvider := initAllStructs(t)

	mockDownloader.EXPECT().
		Download(gomock.Any(), filename, gomock.Any()).
		Times(1).
		Return(nil, nil)

	r, err := googleImageProvider.Request(t.Context(), "test")
	assert.Nil(t, err, "provider return some errors")

	_, err = r.Get(t.Context(), filename, 0)
	assert.Nil(t, err, "query provider return some errors")

}

func TestNegativeIndexOutOfRange(t *testing.T) {
	mockDownloader, googleImageProvider := initAllStructs(t)

	mockDownloader.EXPECT().
		Download(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	p, err := googleImageProvider.Request(t.Context(), "test")
	assert.Nil(t, err, "provider return some errors")

	_, err = p.Get(t.Context(), "filename", 9999999)
	assert.NotNil(t, err, "provider didn't return error")
	assert.ErrorIs(t, err, errIndexOutOfRange)

}

func initAllStructs(t *testing.T) (*mock_downloader.MockFile, Image) {
	logger := slog.Default()
	controller := gomock.NewController(t)
	browser, err := browser.LaunchFirefox()
	assert.Nil(t, err, "browser was created with error")
	mockDownloader := mock_downloader.NewMockFile(controller)
	googleImageProvider := NewImageProvider(logger, browser, mockDownloader)

	return mockDownloader, googleImageProvider
}
