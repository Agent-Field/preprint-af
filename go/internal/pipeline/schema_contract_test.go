package pipeline

import (
	"encoding/json"
	"reflect"
	"testing"
)

func schemaMap[T any](t *testing.T) map[string]any {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(SchemaFor[T](), &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func property(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %#v", schema)
	}
	value, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("schema has no %q property", name)
	}
	return value
}

func requiredNames(schema map[string]any) []string {
	raw, _ := schema["required"].([]any)
	out := make([]string, len(raw))
	for i, value := range raw {
		out[i], _ = value.(string)
	}
	return out
}

func TestReferenceSchemaRequiredAndDefaults(t *testing.T) {
	section := schemaMap[SectionSpec](t)
	wantRequired := []string{"index", "slug", "heading", "beats", "establishes"}
	if got := requiredNames(section); !reflect.DeepEqual(got, wantRequired) {
		t.Fatalf("SectionSpec required=%v want %v", got, wantRequired)
	}
	for name, want := range map[string]any{"requires": "", "target_words": float64(400)} {
		if got := property(t, section, name)["default"]; got != want {
			t.Errorf("SectionSpec.%s default=%#v want %#v", name, got, want)
		}
	}

	evidence := schemaMap[EvidenceSummary](t)
	if got := requiredNames(evidence); len(got) != 0 {
		t.Fatalf("EvidenceSummary fields should all be defaulted, required=%v", got)
	}
	if got := property(t, evidence, "fact_count")["default"]; got != float64(0) {
		t.Errorf("EvidenceSummary.fact_count default=%#v", got)
	}

	plan := schemaMap[RepairPlan](t)
	if got, want := requiredNames(plan), []string{"tasks", "confident"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("RepairPlan required=%v want %v", got, want)
	}
	if got := property(t, plan, "notes")["default"]; got != "" {
		t.Errorf("RepairPlan.notes default=%#v", got)
	}

	runPositioning := schemaMap[RunPositioningInput](t)
	if got := property(t, runPositioning, "allow_web")["default"]; got != true {
		t.Fatalf("RunPositioningInput.allow_web default=%#v want true", got)
	}
	for _, name := range []string{"target_venue", "field_hint", "model"} {
		p := property(t, runPositioning, name)
		if value, exists := p["default"]; !exists || value != nil {
			t.Errorf("RunPositioningInput.%s default=%#v, exists=%t; want explicit null", name, value, exists)
		}
	}
}

func TestReferenceSchemaBoundsAndDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		field  map[string]any
		minima float64
		maxima float64
	}{
		{"StoryFrame.evidence_alignment", property(t, schemaMap[StoryFrame](t), "evidence_alignment"), 0, 1},
		{"FrameJudgment.naturalness", property(t, schemaMap[FrameJudgment](t), "naturalness"), 0, 1},
		{"PositioningDecision.score", property(t, schemaMap[PositioningDecision](t), "score"), 0, 1},
		{"SlopReport.score", property(t, schemaMap[SlopReport](t), "score"), 0, 1},
		{"PersonaReview.acceptance_risk", property(t, schemaMap[PersonaReview](t), "acceptance_risk"), 0, 1},
		{"NarrativeReview.score", property(t, schemaMap[NarrativeReview](t), "score"), 0, 1},
		{"FidelityAudit.score", property(t, schemaMap[FidelityAudit](t), "score"), 0, 1},
	} {
		if got := tc.field["minimum"]; got != tc.minima {
			t.Errorf("%s minimum=%#v want %v", tc.name, got, tc.minima)
		}
		if got := tc.field["maximum"]; got != tc.maxima {
			t.Errorf("%s maximum=%#v want %v", tc.name, got, tc.maxima)
		}
	}

	for _, tc := range []struct {
		name, got, want string
	}{
		{"StoryFrame.angle", property(t, schemaMap[StoryFrame](t), "angle")["description"].(string), "What the frame makes primary: e.g. speed, memory, mechanism, benchmark, reliability."},
		{"SectionSpec.slug", property(t, schemaMap[SectionSpec](t), "slug")["description"].(string), "File slug, e.g. 'intro' -> sections/01_intro.tex."},
		{"FigureSpec.buildable", property(t, schemaMap[FigureSpec](t), "buildable")["description"].(string), "False when required data is missing; becomes a TODO brief instead."},
		{"WorkerResult.status", property(t, schemaMap[WorkerResult](t), "status")["description"].(string), "done | failed | skipped | todo"},
		{"FidelityAudit.blocking", property(t, schemaMap[FidelityAudit](t), "blocking")["description"].(string), "True when claims drift beyond EVIDENCE.md and the round must not pass."},
	} {
		if tc.got != tc.want {
			t.Errorf("%s description=%q want %q", tc.name, tc.got, tc.want)
		}
	}
}
