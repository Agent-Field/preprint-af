package node

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

func TestDefaultModelMatchesPython(t *testing.T) {
	if DefaultModel != "openrouter/deepseek/deepseek-v4-pro" {
		t.Fatalf("unexpected default model %q", DefaultModel)
	}
}

func TestReasonerInputSchemasMatchPythonDiscovery(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(repo, ".venv", "bin", "python")
	if _, err := os.Stat(python); err != nil {
		t.Skip("Python reference venv is not installed")
	}
	script := `
import json, sys
sys.path.insert(0, 'src')
from preprint_af.app import build_app
print('SCHEMAS=' + json.dumps({r['id']: r['input_schema'] for r in build_app().reasoners}, sort_keys=True))
`
	cmd := exec.Command(python, "-c", script)
	cmd.Dir = repo
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("read Python discovery schemas: %v", err)
	}
	marker := []byte("SCHEMAS=")
	start := bytes.LastIndex(raw, marker)
	if start < 0 {
		t.Fatalf("Python schema marker missing: %s", raw)
	}
	var want map[string]map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw[start+len(marker):]), &want); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(reasonerInputContracts) {
		t.Fatalf("Python reasoners=%d Go contracts=%d", len(want), len(reasonerInputContracts))
	}
	for name, expected := range want {
		var actual map[string]any
		if err := json.Unmarshal(reasonerInputSchema(name), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s input schema drifted\nGo: %#v\nPython: %#v", name, actual, expected)
		}
	}
}
