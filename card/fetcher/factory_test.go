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
	SubjectType  models.SubjectType
	ExpectedType string
}

func TestPositiveTests(t *testing.T) {
	testcases := []factoryPositiveCase{
		{
			Name:         "get field fetcher for word",
			SubjectType:  models.SubjectTypeWord,
			ExpectedType: "*fetcher.wordCardContent",
		},
		{
			Name:         "get field fetcher for phrase",
			SubjectType:  models.SubjectTypePhrase,
			ExpectedType: "*fetcher.phraseCardContent",
		},
	}

	logger := slog.Default()
	for _, testcase := range testcases {
		factory := NewCardContentFactory(config.PictureConfig{}, logger, nil, nil, nil)

		t.Run(testcase.Name, func(t *testing.T) {
			fieldFetcher, err := factory.Get(testcase.SubjectType)

			assert.Equal(t, testcase.ExpectedType, reflect.TypeOf(fieldFetcher).String())
			assert.Nil(t, err)
		})
	}
}

func TestUnsupportedSubjectType(t *testing.T) {
	logger := slog.Default()
	factory := NewCardContentFactory(config.PictureConfig{}, logger, nil, nil, nil)

	fieldFetcher, err := factory.Get(models.SubjectTypeNone)
	assert.Nil(t, fieldFetcher)
	assert.EqualError(t, err, "unsupported subject type")
}
