package pipeline

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBlueprintSchemaIncludesNestedSectionContract(t *testing.T) {
	b, err := aiSchema[Blueprint]()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("missing properties in schema: %s", b)
	}
	sections, ok := properties["sections"].(map[string]any)
	if !ok || sections["items"] == nil {
		t.Fatalf("sections lacks an item contract: %s", b)
	}
	if sections["minItems"] != float64(6) || sections["maxItems"] != float64(9) {
		t.Fatalf("sections lacks prompt cardinality: %s", b)
	}
}

func TestSemanticallyEmptyTreatsSeededSlicesAsEmpty(t *testing.T) {
	if !semanticallyEmpty(reflect.ValueOf(NewBlueprint())) {
		t.Fatal("default-initialized blueprint must trigger transport repair")
	}
	nonempty := NewBlueprint()
	nonempty.Sections = []SectionSpec{{Heading: "Introduction"}}
	if semanticallyEmpty(reflect.ValueOf(nonempty)) {
		t.Fatal("blueprint with substantive content must not trigger repair")
	}
}

func TestFrameSetSchemaRequiresPromptCardinality(t *testing.T) {
	b, err := aiSchema[FrameSet]()
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	frames := properties["frames"].(map[string]any)
	if frames["items"] == nil || frames["minItems"] != float64(5) || frames["maxItems"] != float64(6) {
		t.Fatalf("frames lacks its complete prompt contract: %s", b)
	}
}
