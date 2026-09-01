package llm

var (
	cardContentSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paraphrase": map[string]any{
				"type": "string",
			},
			"transcription": map[string]any{
				"type": "string",
			},
			"examples": map[string]any{
				"type":     "array",
				"minItems": 3,
				"maxItems": 5,
				"items": map[string]any{
					"type": "string",
				},
			},
			"synonyms": map[string]any{
				"type":     "array",
				"minItems": 3,
				"maxItems": 5,
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{
			"paraphrase",
			"transcription",
			"examples",
			"synonyms",
		},
		"additionalProperties": false,
	}
	ratePictureSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rating": map[string]any{"type": "integer"},
		},
		"required": []string{
			"rating",
		},
		"additionalProperties": false,
	}
)
