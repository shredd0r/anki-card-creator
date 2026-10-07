package anki

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shredd0r/anki-card-creator/models"
)

// fieldFormatter turns flashcard data into the plain-HTML field values
// AnkiConnect note fields expect. Kept separate from implService so this
// pure, stateless HTML-shaping logic doesn't mix with AnkiConnect I/O.
//
// Bolding the subject inside each example used to be split between an LLM
// prompt instruction (unreliable on small/local models) and a client-side
// JS regex in the card template (anki/const.go) - both are gone now; it's
// done here once, deterministically, at generation time.
type fieldFormatter struct{}

func (fieldFormatter) Example(subject string, examples []models.Example) string {
	// Fallback for examples with no reported SubjectForm (e.g. Cambridge
	// Dictionary's, which are plain scraped sentences): guess based on the
	// subject's own spelling. Doesn't catch irregular inflections (e.g. "go"
	// -> "went"), but there's no better signal available for those sources.
	fallback := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(subject) + `\w*`)

	items := ""
	for _, example := range examples {
		items += fmt.Sprintf("<li>%s</li>", boldExample(example, fallback))
	}
	return fmt.Sprintf("<ul>%s</ul>", items)
}

func boldExample(example models.Example, fallback *regexp.Regexp) string {
	bold := func(match string) string { return "<b>" + match + "</b>" }

	if example.SubjectForm != "" {
		if form, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(example.SubjectForm)); err == nil && form.MatchString(example.Sentence) {
			return form.ReplaceAllStringFunc(example.Sentence, bold)
		}
	}
	return fallback.ReplaceAllStringFunc(example.Sentence, bold)
}

func (fieldFormatter) Picture(filename string) string {
	return fmt.Sprintf("<img src='%s'>", filename)
}

func (fieldFormatter) Synonyms(synonyms []string) string {
	return strings.Join(synonyms, ", ")
}
