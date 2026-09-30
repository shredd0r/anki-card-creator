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
						Description: "3 to 5 example sentences",
						Items: &openai.ResponseFormatJSONSchemaProperty{
							Type: "string",
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
