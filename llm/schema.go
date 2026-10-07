package llm

import "github.com/tmc/langchaingo/llms/openai"

var (
	cardContentResponseFormat = &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   "card_content",
			Strict: true,
			Schema: &openai.ResponseFormatJSONSchemaProperty{
				Type: "object",
				Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
					"paraphrase": {
						Type: "string",
					},
					"transcription": {
						Type: "string",
					},
					"examples": {
						Type:        "array",
						Description: "3 to 5 example sentences, each with the exact subject form used",
						Items: &openai.ResponseFormatJSONSchemaProperty{
							Type: "object",
							Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
								"sentence": {
									Type:        "string",
									Description: "the example sentence, as plain text with no markup",
								},
								"subject_form": {
									Type:        "string",
									Description: "the exact word or phrase, copied verbatim from sentence, that is the subject or its inflected form",
								},
							},
							Required: []string{
								"sentence",
								"subject_form",
							},
							AdditionalProperties: false,
						},
					},
					"synonyms": {
						Type:        "array",
						Description: "3 to 5 synonyms",
						Items: &openai.ResponseFormatJSONSchemaProperty{
							Type: "string",
						},
					},
				},
				Required: []string{
					"paraphrase",
					"transcription",
					"examples",
					"synonyms",
				},
				AdditionalProperties: false,
			},
		},
	}

	ratePictureResponseFormat = &openai.ResponseFormat{
		Type: "json_schema",
		JSONSchema: &openai.ResponseFormatJSONSchema{
			Name:   "rate_picture",
			Strict: true,
			Schema: &openai.ResponseFormatJSONSchemaProperty{
				Type: "object",
				Properties: map[string]*openai.ResponseFormatJSONSchemaProperty{
					"rating": {
						Type: "integer",
					},
				},
				Required: []string{
					"rating",
				},
				AdditionalProperties: false,
			},
		},
	}
)
