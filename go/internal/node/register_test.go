package node

import (
	"reflect"
	"testing"
)

func TestRegisteredReasonerSurfaceMatchesPython(t *testing.T) {
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

func TestRequestedQwenModelIsDefault(t *testing.T) {
	if DefaultModel != "openrouter/qwen/qwen3.7-flash" {
		t.Fatalf("unexpected default model %q", DefaultModel)
	}
}
