package pipeline

import (
	"encoding/json"
	"testing"
)

func TestStructuredSchemasMatchPythonCardinality(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{
		"frames":    mustSchema[FrameSet](t),
		"blueprint": mustSchema[Blueprint](t),
	} {
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		propertyName := "frames"
		if name == "blueprint" {
			propertyName = "sections"
		}
		property := schema["properties"].(map[string]any)[propertyName].(map[string]any)
		if property["items"] == nil {
			t.Fatalf("%s lacks nested item schema: %s", name, raw)
		}
		if _, exists := property["minItems"]; exists {
			t.Fatalf("%s added a minItems constraint absent from Python Pydantic schema", name)
		}
		if _, exists := property["maxItems"]; exists {
			t.Fatalf("%s added a maxItems constraint absent from Python Pydantic schema", name)
		}
	}
}

func mustSchema[T any](t *testing.T) json.RawMessage {
	t.Helper()
	raw, err := aiSchema[T]()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
