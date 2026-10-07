package anki

import (
	"testing"

	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
)

func TestFieldFormatter_Example_WrapsListItemsAndBoldsSubjectForm(t *testing.T) {
	f := fieldFormatter{}

	got := f.Example("apple", []models.Example{
		{Sentence: "i ate an apple.", SubjectForm: "apple"},
		{Sentence: "apples are red.", SubjectForm: "apples"},
	})

	assert.Equal(t, "<ul><li>i ate an <b>apple</b>.</li><li><b>apples</b> are red.</li></ul>", got)
}

func TestFieldFormatter_Example_BoldsIrregularSubjectForm(t *testing.T) {
	f := fieldFormatter{}

	got := f.Example("go", []models.Example{
		{Sentence: "i went to the store yesterday.", SubjectForm: "went"},
	})

	assert.Equal(t, "<ul><li>i <b>went</b> to the store yesterday.</li></ul>", got)
}

func TestFieldFormatter_Example_FallsBackToSubjectSpellingWhenFormMissing(t *testing.T) {
	f := fieldFormatter{}

	// No SubjectForm (e.g. a Cambridge Dictionary example) - falls back to a
	// prefix-based guess on the subject's own spelling.
	got := f.Example("lime", []models.Example{
		{Sentence: "the lime is sour."},
	})

	assert.Equal(t, "<ul><li>the <b>lime</b> is sour.</li></ul>", got)
}

func TestFieldFormatter_Example_FallsBackWhenFormNotFoundInSentence(t *testing.T) {
	f := fieldFormatter{}

	// SubjectForm doesn't actually appear in Sentence (model inconsistency) -
	// falls back rather than leaving the sentence unbolded.
	got := f.Example("apple", []models.Example{
		{Sentence: "i ate an apple.", SubjectForm: "pear"},
	})

	assert.Equal(t, "<ul><li>i ate an <b>apple</b>.</li></ul>", got)
}

func TestFieldFormatter_Example_NoExamples(t *testing.T) {
	f := fieldFormatter{}

	got := f.Example("apple", []models.Example{})

	assert.Equal(t, "<ul></ul>", got)
}

func TestFieldFormatter_Picture(t *testing.T) {
	f := fieldFormatter{}

	got := f.Picture("cat.jpg")

	assert.Equal(t, "<img src='cat.jpg'>", got)
}

func TestFieldFormatter_Synonyms(t *testing.T) {
	f := fieldFormatter{}

	got := f.Synonyms([]string{"big", "large"})

	assert.Equal(t, "big, large", got)
}
