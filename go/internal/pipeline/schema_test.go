package pipeline

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStructuredSchemasMatchReferenceCardinality(t *testing.T) {
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
			t.Fatalf("%s added a minItems constraint absent from the reference schema", name)
		}
		if _, exists := property["maxItems"]; exists {
			t.Fatalf("%s added a maxItems constraint absent from the reference schema", name)
		}
	}
}

func TestDirectAISchemaUsesPythonStrictResponseContract(t *testing.T) {
	raw, err := aiSchema[RepairPlan]()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if got := schema["additionalProperties"]; got != false {
		t.Fatalf("direct AI additionalProperties = %#v, want false", got)
	}
	if got, want := requiredNames(schema), []string{"confident", "notes", "tasks"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("direct AI required = %v, want %v", got, want)
	}
}

func TestHarnessSchemaKeepsPydanticDefaultsOptional(t *testing.T) {
	raw, err := harnessSchema[EvidenceSummary]()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if got := requiredNames(schema); len(got) != 0 {
		t.Fatalf("harness schema made defaulted fields required: %v", got)
	}
	if got, exists := schema["additionalProperties"]; exists && got == false {
		t.Fatalf("harness schema unexpectedly closed object: %#v", schema)
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
