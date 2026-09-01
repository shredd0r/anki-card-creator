package utils

import (
	"strings"

	"github.com/shredd0r/anki-card-creator/models"
)

func GetSubjectType(subject string) models.SubjectType {
	if strings.Contains(subject, " ") || strings.Contains(subject, "-") {
		return models.SubjectTypePhrase
	}
	return models.SubjectTypeWord
}
