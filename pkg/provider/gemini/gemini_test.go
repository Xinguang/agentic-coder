package gemini

import (
	"encoding/json"
	"testing"
)

func TestCleanSchemaForGemini(t *testing.T) {
	// Schema with additionalProperties
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"nested": {
				"type": "object",
				"additionalProperties": false,
				"properties": {
					"value": {"type": "number"}
				}
			}
		},
		"additionalProperties": false,
		"required": ["name"]
	}`)

	cleaned := cleanSchemaForGemini(schema)

	var result map[string]interface{}
	if err := json.Unmarshal(cleaned, &result); err != nil {
		t.Fatalf("Failed to unmarshal cleaned schema: %v", err)
	}

	// Check additionalProperties is removed at root
	if _, exists := result["additionalProperties"]; exists {
		t.Error("additionalProperties should be removed from root")
	}

	// Check nested additionalProperties is removed
	props := result["properties"].(map[string]interface{})
	nested := props["nested"].(map[string]interface{})
	if _, exists := nested["additionalProperties"]; exists {
		t.Error("additionalProperties should be removed from nested object")
	}

	// Check required is preserved
	if _, exists := result["required"]; !exists {
		t.Error("required should be preserved")
	}
}

func TestCleanSchemaForGeminiEmpty(t *testing.T) {
	// Empty schema
	schema := json.RawMessage(`{}`)
	cleaned := cleanSchemaForGemini(schema)

	var result map[string]interface{}
	if err := json.Unmarshal(cleaned, &result); err != nil {
		t.Fatalf("Failed to unmarshal cleaned schema: %v", err)
	}

	if len(result) != 0 {
		t.Error("Empty schema should remain empty")
	}
}

func TestCleanSchemaForGeminiArray(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "array",
		"items": {
			"type": "object",
			"additionalProperties": true,
			"properties": {
				"id": {"type": "string"}
			}
		}
	}`)

	cleaned := cleanSchemaForGemini(schema)

	var result map[string]interface{}
	if err := json.Unmarshal(cleaned, &result); err != nil {
		t.Fatalf("Failed to unmarshal cleaned schema: %v", err)
	}

	items := result["items"].(map[string]interface{})
	if _, exists := items["additionalProperties"]; exists {
		t.Error("additionalProperties should be removed from array items")
	}
}
