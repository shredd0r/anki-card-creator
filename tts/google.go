package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// googleTranslateTTSURL is the same undocumented endpoint Google Translate's
// own "listen" button uses. It has no official support/SLA and caps input
// around ~200 characters, but that's more than enough for the single
// words/short phrases pronunciation audio is generated for here.
const googleTranslateTTSURL = "https://translate.google.com/translate_tts?ie=UTF-8&client=tw-ob&tl=%s&q=%s"

// NewGoogleSpeech returns a Speech implementation backed by Google
// Translate's TTS endpoint - a simple cloud HTTP call, no local audio
// library/dependency required.
func NewGoogleSpeech(logger *slog.Logger, language Language) Speech {
	return &implGoogleSpeech{
		logger:     logger.WithGroup("google-speech"),
		language:   language,
		httpClient: http.DefaultClient,
	}
}

type implGoogleSpeech struct {
	logger     *slog.Logger
	language   Language
	httpClient *http.Client
}

func (s *implGoogleSpeech) Create(ctx context.Context, text string) (io.Reader, error) {
	requestUrl := fmt.Sprintf(googleTranslateTTSURL, s.language, url.QueryEscape(text))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("build tts request: %w", err)
	}
	// Google rejects requests that don't look like they came from a browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call google translate tts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google translate tts returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tts response body: %w", err)
	}

	s.logger.Debug("synthesized speech", slog.String("text", text), slog.Int("bytes", len(body)))
	return bytes.NewReader(body), nil
}
