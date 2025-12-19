package fetcher

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
)

type factoryPositiveCase struct {
	Name         string
	OnlyAI       bool
	SubjectType  models.SubjectType
	ExpectedType string
}

func TestPositiveTests(t *testing.T) {
	testcases := []factoryPositiveCase{
		{
			Name:         "get field fetcher for word by cambridge dictionary",
			OnlyAI:       false,
			SubjectType:  models.SubjectTypeWord,
			ExpectedType: "*fetcher.wordCardContentByCambridge",
		},
		{
			Name:         "get field fetcher for word by llm",
			OnlyAI:       true,
			SubjectType:  models.SubjectTypeWord,
			ExpectedType: "*fetcher.wordCardContentByAI",
		},
		{
			Name:         "get field fetcher for phrase",
			SubjectType:  models.SubjectTypePhrase,
			ExpectedType: "*fetcher.phraseCardContent",
		},
	}

	logger := slog.Default()
	for _, testcase := range testcases {
		factory := NewCardContentFactory(config.Config{OnlyAI: testcase.OnlyAI}, logger, nil, nil)

		t.Run(testcase.Name, func(t *testing.T) {
			fieldFetcher, err := factory.Get(testcase.SubjectType)

			assert.Equal(t, testcase.ExpectedType, reflect.TypeOf(fieldFetcher).String())
			assert.Nil(t, err)
		})
	}
}

func TestUnsupportedSubjectType(t *testing.T) {
	logger := slog.Default()
	factory := NewCardContentFactory(config.Config{}, logger, nil, nil)

	fieldFetcher, err := factory.Get(models.SubjectTypeNone)
	assert.Nil(t, fieldFetcher)
	assert.EqualError(t, err, "unsupported subject type")
}
