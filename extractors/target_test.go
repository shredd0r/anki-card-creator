package extractors

import (
	"context"
	"log/slog"
	"testing"

	"github.com/shredd0r/anki-card-creator/models"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	Name             string
	PathToTargets    string
	CallTestesMethod func(context.Context, string) (*[]models.Target, error)
}

func TestPositiveGetTargets(t *testing.T) {
	logger := slog.Default()
	targetExtractor := NewTargetExtractor(logger)

	testcases := []testCase{
		{
			Name:             "get targets from file",
			PathToTargets:    "./../assets/test-targets.json",
			CallTestesMethod: targetExtractor.GetTargetsFromFile,
		},
		{
			Name:             "get targets from directory",
			PathToTargets:    "./../assets",
			CallTestesMethod: targetExtractor.GetTargetsFromDir,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			targets, err := testcase.CallTestesMethod(t.Context(), testcase.PathToTargets)

			assert.Nil(t, err)
			assert.Equal(t, []models.Target{
				{
					DeckName: "deckname-1",
					Subjects: []string{
						"word-subject",
						"phrase subject",
					},
				},
				{
					DeckName: "deckname-2",
					Subjects: []string{
						"word-subject",
					},
				},
			}, *targets)
		})
	}
}

func TestNegativeCases(t *testing.T) {
	logger := slog.Default()
	targetExtractor := NewTargetExtractor(logger)

	testcases := []testCase{
		{
			Name:             "not exist file",
			PathToTargets:    "./not-exist-file.json",
			CallTestesMethod: targetExtractor.GetTargetsFromFile,
		},
		{
			Name:             "not exist dir",
			PathToTargets:    "./not-exist-dir",
			CallTestesMethod: targetExtractor.GetTargetsFromDir,
		},
		{
			Name:             "file not json",
			PathToTargets:    "./../assets/jpg-file.jpg",
			CallTestesMethod: targetExtractor.GetTargetsFromFile,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Name, func(t *testing.T) {
			targets, err := testcase.CallTestesMethod(t.Context(), testcase.PathToTargets)

			assert.Nil(t, targets)
			assert.NotNil(t, err)
		})
	}
}
