package extractor_test

import (
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/browser"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/stretchr/testify/assert"
)

func TestGet(t *testing.T) {
	b, _ := browser.LaunchFirefox()

	fd := downloader.NewFile(slog.Default())
	ce := extractor.NewCambridge(slog.Default(), b, fd)

	card, err := ce.GetCard(t.Context(), "car")
	assert.Nil(t, err)
	assert.NotNil(t, card)

	assert.Equal(t, card.Subject, "car")
}
