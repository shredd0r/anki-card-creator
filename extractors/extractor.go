package extractors

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/playwright-community/playwright-go"
	"github.com/shredd0r/anki-card-creator/models"
)

var errIndexOutOfRange = errors.New("index out of range")

type CardExtractor[TYPE_OF_CARD any] interface {
	GetLength(ctx context.Context) (*int, error)
	GetLessons(ctx context.Context) (*[]string, error)
	GetCards(ctx context.Context, index int) (*[]*TYPE_OF_CARD, error)
}

func getInnerTextFromChild(logger *slog.Logger, parentLocator playwright.Locator, selector string) (*string, error) {
	childLocator := parentLocator.Locator(selector).First()
	innerText, err := childLocator.InnerText()
	if err != nil {
		logger.Error("failed get inner text from locator", slog.Any("err", err.Error()))
		return nil, err
	}
	return &innerText, nil
}

func getSubjectType(subject string) models.SubjectType {
	if strings.Contains(subject, " ") {
		return models.SubjectTypePhrase
	}
	return models.SubjectTypeWord
}
