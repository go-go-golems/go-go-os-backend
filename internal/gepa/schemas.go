package gepa

var schemaDocuments = map[string]map[string]any{
	"gepa.scripts.list.response.v1": {
		"type": "object",
		"properties": map[string]any{
			"scripts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "string"},
						"name": map[string]any{"type": "string"},
						"path": map[string]any{"type": "string"},
					},
					"required":             []any{"id", "name", "path"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []any{"scripts"},
		"additionalProperties": false,
	},
	"gepa.runs.start.request.v1": {
		"type": "object",
		"properties": map[string]any{
			"script_id": map[string]any{"type": "string"},
			"arguments": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
			"input": map[string]any{
				"type": "object",
			},
		},
		"required":             []any{"script_id"},
		"additionalProperties": false,
	},
	"gepa.runs.start.response.v1": {
		"type": "object",
		"properties": map[string]any{
			"run": map[string]any{
				"type": "object",
			},
		},
		"required": []any{"run"},
	},
	"gepa.runs.get.response.v1": {
		"type": "object",
		"properties": map[string]any{
			"run": map[string]any{
				"type": "object",
			},
		},
		"required": []any{"run"},
	},
	"gepa.error.v1": {
		"type": "object",
		"properties": map[string]any{
			"error": map[string]any{"type": "string"},
		},
		"required": []any{"error"},
	},
}
