package node

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestRegisteredReasonerSurfaceMatchesReference(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	n, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	n.RegisterAll()
	want := []string{
		"intake_prepare_workspace", "intake_build_evidence_ledger",
		"positioning_generate_frames", "positioning_judge_frame", "positioning_scan_novelty", "positioning_run_positioning",
		"blueprint_design_blueprint",
		"build_write_section", "build_build_figure", "build_build_bibliography", "build_run_build",
		"latex_compile_paper",
		"critique_persona_review", "critique_narrative_review", "critique_fidelity_audit", "critique_run_critique",
		"repair_plan_repairs", "repair_apply_repairs",
		"write_paper",
	}
	if got := n.RegisteredNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reasoner surface drifted\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestDefaultModelMatchesReference(t *testing.T) {
	if DefaultModel != "openrouter/deepseek/deepseek-v4-pro" {
		t.Fatalf("unexpected default model %q", DefaultModel)
	}
}

func TestReasonerInputSchemasMatchReferenceGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/input-schemas.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(reasonerInputContracts) {
		t.Fatalf("golden reasoners=%d Go contracts=%d", len(want), len(reasonerInputContracts))
	}
	for name, expected := range want {
		var actual map[string]any
		if err := json.Unmarshal(reasonerInputSchema(name), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s input schema drifted\nGo: %#v\nGolden: %#v", name, actual, expected)
		}
	}
}
