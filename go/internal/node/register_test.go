package node

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/Agent-Field/preprint-af/go/internal/afx"
	"github.com/Agent-Field/preprint-af/go/internal/pipeline"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

func TestRegisteredReasonerSurfaceMatchesReference(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENCODE_MAX_CONCURRENT", "")
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

func TestBuildAndRegisterAllWithOpenRouterKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-only-key")
	t.Setenv("OPENCODE_MAX_CONCURRENT", "")
	n, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	n.RegisterAll()
	if got, want := len(n.RegisteredNames()), len(reasonerInputContracts); got != want {
		t.Fatalf("registered %d reasoners with key, want %d", got, want)
	}
}

func TestHarnessConfigurationMatchesReferenceDefaults(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("AI_MODEL", "openrouter/example/direct-only")
	t.Setenv("OPENCODE_MODEL", "")
	t.Setenv("OPENCODE_MAX_CONCURRENT", "")
	t.Setenv("AGENTFIELD_HARNESS_TIMEOUT_SECONDS", "")
	n, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	opts := n.App.HarnessRunner().DefaultOptions
	if opts.Provider != "opencode" || opts.Model != DefaultModel || opts.MaxTurns != 40 || opts.PermissionMode != "auto" || opts.Timeout != 1800 {
		t.Fatalf("harness defaults drifted: %#v", opts)
	}
	if got := os.Getenv("OPENCODE_MAX_CONCURRENT"); got != "10" {
		t.Fatalf("OpenCode concurrency default = %q, want 10", got)
	}
}

func TestExamplePayloadBindsAtWritePaperEntryPoint(t *testing.T) {
	raw, err := os.ReadFile("../../../examples/payload.json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("write-paper.json", bytes.NewReader(reasonerInputSchema("write_paper"))); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("write-paper.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(envelope.Input); err != nil {
		t.Fatalf("write_paper input schema rejected examples/payload.json: %v", err)
	}
	request, err := afx.Bind[pipeline.WriteRequest](envelope.Input)
	if err != nil {
		t.Fatalf("write_paper rejected examples/payload.json: %v", err)
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("write_paper rejected examples/payload.json after binding: %v", err)
	}
	if request.TargetVenue == nil || request.FieldHint == nil || request.MaxRounds != 6 || !request.AllowWeb || request.DryRun {
		t.Fatalf("example payload bound incorrectly: %#v", request)
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
