package providers

import (
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/browsers"
	mock_downloader "github.com/shredd0r/anki-card-creator/downloader/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestPositiveGetFile(t *testing.T) {
	mockDownloader, googleImageProvider := initAllStructs(t)

	mockDownloader.EXPECT().
		Download(gomock.Any(), gomock.All()).
		Times(1).
		Return(nil, nil)

	_, err := googleImageProvider.Get(t.Context(), 0, "test")
	assert.Nil(t, err, "provider return some errors")
}

func TestNegativeIndexOutOfRange(t *testing.T) {
	mockDownloader, googleImageProvider := initAllStructs(t)

	mockDownloader.EXPECT().
		Download(gomock.Any(), gomock.All()).
		Times(0)

	_, err := googleImageProvider.Get(t.Context(), 999999, "test")
	assert.NotNil(t, err, "provider didn't return error")
	assert.ErrorIs(t, err, errIndexOutOfRange)

}

func initAllStructs(t *testing.T) (*mock_downloader.MockFileDownloader, GoogleImageProvider) {
	logger := slog.Default()
	controller := gomock.NewController(t)
	browser, err := browsers.LaunchFirefox()
	assert.Nil(t, err, "browser was created with error")
	mockDownloader := mock_downloader.NewMockFileDownloader(controller)
	googleImageProvider := NewGoogleImageProvider(logger, browser, mockDownloader)

	return mockDownloader, googleImageProvider
}
