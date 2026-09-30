package anki

import (
	"context"
	"fmt"
	"slices"
)

func noteTypeFieldNames() []string {
	return []string{"Subject", "Pronunciation", "Transcription", "Paraphrase", "Synonyms", "Picture", "Example"}
}

type createModelParams struct {
	ModelName     string             `json:"modelName"`
	InOrderFields []string           `json:"inOrderFields"`
	CSS           string             `json:"css"`
	IsCloze       bool               `json:"isCloze"`
	CardTemplates []cardTemplateSpec `json:"cardTemplates"`
}

type cardTemplateSpec struct {
	Name  string `json:"Name"`
	Front string `json:"Front"`
	Back  string `json:"Back"`
}

type updateModelTemplatesParams struct {
	Model modelTemplatesSpec `json:"model"`
}

type modelTemplatesSpec struct {
	Name      string                   `json:"name"`
	Templates map[string]templateSides `json:"templates"`
}

type templateSides struct {
	Front string `json:"Front"`
	Back  string `json:"Back"`
}

type updateModelStylingParams struct {
	Model modelStylingSpec `json:"model"`
}

type modelStylingSpec struct {
	Name string `json:"name"`
	CSS  string `json:"css"`
}

// ensureNoteType makes sure a note type named s.modelName exists in Anki with
// the fields/templates/CSS defined in anki/const.go, creating it on first run
// and self-healing its templates/styling on every subsequent run. A note type
// that already exists with a different field set is treated as a hard error
// rather than auto-migrated, since reshaping fields on a live note type can
// destroy existing card data.
func (s *implService) ensureNoteType(ctx context.Context) error {
	var modelNames []string
	if err := s.client.Invoke(ctx, "modelNames", nil, &modelNames); err != nil {
		return fmt.Errorf("list model names: %w", err)
	}

	if !slices.Contains(modelNames, s.modelName) {
		return s.createModel(ctx)
	}

	var fieldNames []string
	if err := s.client.Invoke(ctx, "modelFieldNames", map[string]string{"modelName": s.modelName}, &fieldNames); err != nil {
		return fmt.Errorf("list model field names: %w", err)
	}
	if !slices.Equal(fieldNames, noteTypeFieldNames()) {
		return fmt.Errorf("note type %q already exists in Anki with a different field set (found %v, expected %v) - fix it by hand in Anki before running again", s.modelName, fieldNames, noteTypeFieldNames())
	}

	if err := s.updateModelTemplates(ctx); err != nil {
		return err
	}
	return s.updateModelStyling(ctx)
}

func (s *implService) createModel(ctx context.Context) error {
	return s.client.Invoke(ctx, "createModel", createModelParams{
		ModelName:     s.modelName,
		InOrderFields: noteTypeFieldNames(),
		CSS:           css,
		CardTemplates: []cardTemplateSpec{
			{Name: template_name, Front: front_side_template, Back: back_side_template},
		},
	}, nil)
}

func (s *implService) updateModelTemplates(ctx context.Context) error {
	return s.client.Invoke(ctx, "updateModelTemplates", updateModelTemplatesParams{
		Model: modelTemplatesSpec{
			Name: s.modelName,
			Templates: map[string]templateSides{
				template_name: {Front: front_side_template, Back: back_side_template},
			},
		},
	}, nil)
}

func (s *implService) updateModelStyling(ctx context.Context) error {
	return s.client.Invoke(ctx, "updateModelStyling", updateModelStylingParams{
		Model: modelStylingSpec{Name: s.modelName, CSS: css},
	}, nil)
}
