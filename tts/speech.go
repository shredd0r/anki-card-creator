package tts

//go:generate mockgen -source speech.go -destination mock/speech_mock.go

import (
	"context"
	"io"
)

// Language is a locale/voice tag understood by whichever Speech
// implementation is in use (e.g. Google Translate's "tl" query param, or a
// future local synthesizer's own voice/language selection).
type Language string

const (
	LanguageEnglishUK Language = "en-gb"
	LanguageEnglishUS Language = "en-us"
)

// Speech synthesizes audio for a piece of text. Callers depend only on this
// interface - not on any particular backend - so the implementation can be
// swapped between a cloud provider (see NewGoogleSpeech) and a local
// synthesizer (e.g. a future espeak-ng-backed implementation, see
// FEATURE.md) without any caller changes.
type Speech interface {
	// Create synthesizes speech audio for text and returns it as MP3 bytes.
	Create(ctx context.Context, text string) (io.Reader, error)
}
