package node

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Agent-Field/agentfield/sdk/go/agent"
	"github.com/Agent-Field/preprint-af/go/internal/afx"
	"github.com/Agent-Field/preprint-af/go/internal/pipeline"
)

func (n *Node) RegisterAll() {
	regReasoner[pipeline.PrepareWorkspaceInput, pipeline.Workspace](n, "intake_prepare_workspace", []string{"intake"}, n.Pipeline.PrepareWorkspace)
	regReasoner[pipeline.BuildEvidenceInput, pipeline.EvidenceSummary](n, "intake_build_evidence_ledger", []string{"intake"}, n.Pipeline.BuildEvidenceLedger)
	regReasoner[pipeline.GenerateFramesInput, pipeline.FrameSet](n, "positioning_generate_frames", []string{"positioning"}, n.Pipeline.GenerateFrames)
	regReasoner[pipeline.JudgeFrameInput, pipeline.FrameJudgment](n, "positioning_judge_frame", []string{"positioning"}, n.Pipeline.JudgeFrame)
	regReasoner[pipeline.ScanNoveltyInput, pipeline.NoveltyScan](n, "positioning_scan_novelty", []string{"positioning"}, n.Pipeline.ScanNovelty)
	regReasoner[pipeline.RunPositioningInput, pipeline.PositioningDecision](n, "positioning_run_positioning", []string{"positioning"}, n.Pipeline.RunPositioning)
	regReasoner[pipeline.DesignBlueprintInput, pipeline.Blueprint](n, "blueprint_design_blueprint", []string{"blueprint"}, n.Pipeline.DesignBlueprint)
	regReasoner[pipeline.WriteSectionInput, pipeline.WorkerResult](n, "build_write_section", []string{"build"}, n.Pipeline.WriteSection)
	regReasoner[pipeline.BuildFigureInput, pipeline.WorkerResult](n, "build_build_figure", []string{"build"}, n.Pipeline.BuildFigure)
	regReasoner[pipeline.BuildBibliographyInput, pipeline.WorkerResult](n, "build_build_bibliography", []string{"build"}, n.Pipeline.BuildBibliography)
	regReasoner[pipeline.RunBuildInput, pipeline.BuildReport](n, "build_run_build", []string{"build"}, n.Pipeline.RunBuild)
	regReasoner[pipeline.CompilePaperInput, pipeline.CompileReport](n, "latex_compile_paper", []string{"latex"}, n.Pipeline.CompilePaper)
	regReasoner[pipeline.PersonaReviewInput, pipeline.PersonaReview](n, "critique_persona_review", []string{"critique"}, n.Pipeline.PersonaReview)
	regReasoner[pipeline.NarrativeReviewInput, pipeline.NarrativeReview](n, "critique_narrative_review", []string{"critique"}, n.Pipeline.NarrativeReview)
	regReasoner[pipeline.FidelityAuditInput, pipeline.FidelityAudit](n, "critique_fidelity_audit", []string{"critique"}, n.Pipeline.FidelityAudit)
	regReasoner[pipeline.RunCritiqueInput, pipeline.CritiqueBundle](n, "critique_run_critique", []string{"critique"}, n.Pipeline.RunCritique)
	regReasoner[pipeline.PlanRepairsInput, pipeline.RepairPlan](n, "repair_plan_repairs", []string{"repair"}, n.Pipeline.PlanRepairs)
	regReasoner[pipeline.ApplyRepairsInput, int](n, "repair_apply_repairs", []string{"repair"}, n.Pipeline.ApplyRepairs)
	regReasoner[pipeline.WriteRequest, pipeline.WriteResult](n, "write_paper", []string{"workflow", "entry"}, n.Pipeline.WritePaper,
		agent.WithOutputSchema(json.RawMessage(`{"type":"object"}`)))
}

func regReasoner[I, O any](n *Node, name string, tags []string, fn func(context.Context, I) (any, error), extra ...agent.ReasonerOption) {
	opts := []agent.ReasonerOption{
		agent.WithReasonerTags(tags...),
		agent.WithInputSchema(reasonerInputSchema(name)),
		agent.WithOutputSchema(pipeline.SchemaFor[O]()),
	}
	if description := reasonerDescriptions[name]; description != "" {
		opts = append(opts, agent.WithDescription(description))
	}
	opts = append(opts, extra...)
	n.App.RegisterReasoner(name, func(ctx context.Context, input map[string]any) (result any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = nil
				err = fmt.Errorf("%s panic: %v", name, recovered)
			}
		}()
		in, err := afx.Bind[I](input)
		if err != nil {
			return nil, err
		}
		return fn(ctx, in)
	}, opts...)
	n.registered = append(n.registered, name)
}

func (n *Node) RegisteredNames() []string { return append([]string(nil), n.registered...) }

// Python's reasoner decorator registers the first paragraph of a function's
// docstring. Entries without a docstring intentionally remain undescribed.
var reasonerDescriptions = map[string]string{
	"intake_prepare_workspace":     "P0a: deterministically scaffold the run workspace from the user's folder.",
	"intake_build_evidence_ledger": "P0b: one OpenCode harness pass that reads input/ and writes EVIDENCE.md.",
	"blueprint_design_blueprint":   "P2: design the paper blueprint and write BLUEPRINT.md + paper/main.tex.",
	"build_write_section":          "P3: write one section .tex file via a single OpenCode harness pass.",
	"build_build_figure":           "P3: build one figure by writing and running a matplotlib script against real data.",
	"build_build_bibliography":     "P3: consolidate and resolve the paper's references into paper/refs.bib.",
	"build_run_build":              "P3 orchestrator: build all sections, figures, and the bibliography concurrently.",
	"latex_compile_paper":          "P4: compile the paper with latexmk, repairing LaTeX errors between attempts (up to 3).",
	"critique_persona_review":      "One .ai review of the full paper from a single reviewer persona.",
	"critique_narrative_review":    "One .ai review of the paper's transitions, promise alignment, and story arc.",
	"critique_fidelity_audit":      "One .ai audit tracing every quantitative claim and citation to evidence. Fails CLOSED.",
	"critique_run_critique":        "Orchestrator: all persona/narrative/fidelity reviewers in ONE gather + deterministic slop.",
	"repair_plan_repairs":          "One .ai call routing critique findings into at most 8 targeted RepairTasks.",
	"repair_apply_repairs":         "Apply the repair plan: front_matter first alone, then the rest in one gather.",
}

type inputContract struct {
	properties map[string]map[string]any
	required   []string
}

var (
	objectInput  = map[string]any{"type": "object"}
	stringInput  = map[string]any{"type": "string"}
	integerInput = map[string]any{"type": "integer"}
	numberInput  = map[string]any{"type": "number"}
	booleanInput = map[string]any{"type": "boolean"}
	stringsInput = map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
)

// These schemas intentionally mirror AgentField Python's function-annotation
// discovery output. Raw dict parameters stay untyped objects; resolved
// Optional[str] annotations are exposed as strings and made optional by absence
// from the required list.
var reasonerInputContracts = map[string]inputContract{
	"intake_prepare_workspace":     {map[string]map[string]any{"folder_path": stringInput, "model": stringInput}, []string{"folder_path"}},
	"intake_build_evidence_ledger": {map[string]map[string]any{"workspace": objectInput, "model": stringInput}, []string{"workspace"}},
	"positioning_generate_frames":  {map[string]map[string]any{"workspace": objectInput, "target_venue": stringInput, "field_hint": stringInput, "model": stringInput}, []string{"workspace"}},
	"positioning_judge_frame":      {map[string]map[string]any{"frame": objectInput, "evidence_digest": stringInput, "persona": stringInput, "target_venue": stringInput, "model": stringInput}, []string{"frame", "evidence_digest", "persona"}},
	"positioning_scan_novelty":     {map[string]map[string]any{"workspace": objectInput, "frame_set": objectInput, "model": stringInput}, []string{"workspace", "frame_set"}},
	"positioning_run_positioning":  {map[string]map[string]any{"workspace": objectInput, "target_venue": stringInput, "field_hint": stringInput, "allow_web": booleanInput, "model": stringInput}, []string{"workspace"}},
	"blueprint_design_blueprint":   {map[string]map[string]any{"workspace": objectInput, "target_venue": stringInput, "model": stringInput}, []string{"workspace"}},
	"build_write_section":          {map[string]map[string]any{"workspace": objectInput, "section": objectInput, "prev_section": objectInput, "next_section": objectInput, "model": stringInput}, []string{"workspace", "section"}},
	"build_build_figure":           {map[string]map[string]any{"workspace": objectInput, "figure": objectInput, "model": stringInput}, []string{"workspace", "figure"}},
	"build_build_bibliography":     {map[string]map[string]any{"workspace": objectInput, "citation_needs": stringsInput, "allow_web": booleanInput, "model": stringInput}, []string{"workspace", "citation_needs"}},
	"build_run_build":              {map[string]map[string]any{"workspace": objectInput, "blueprint": objectInput, "allow_web": booleanInput, "model": stringInput}, []string{"workspace", "blueprint"}},
	"latex_compile_paper":          {map[string]map[string]any{"workspace": objectInput, "model": stringInput}, []string{"workspace"}},
	"critique_persona_review":      {map[string]map[string]any{"workspace": objectInput, "persona": stringInput, "round_no": integerInput, "model": stringInput}, []string{"workspace", "persona", "round_no"}},
	"critique_narrative_review":    {map[string]map[string]any{"workspace": objectInput, "round_no": integerInput, "model": stringInput}, []string{"workspace", "round_no"}},
	"critique_fidelity_audit":      {map[string]map[string]any{"workspace": objectInput, "round_no": integerInput, "model": stringInput}, []string{"workspace", "round_no"}},
	"critique_run_critique":        {map[string]map[string]any{"workspace": objectInput, "round_no": integerInput, "model": stringInput}, []string{"workspace", "round_no"}},
	"repair_plan_repairs":          {map[string]map[string]any{"workspace": objectInput, "critique": objectInput, "model": stringInput}, []string{"workspace", "critique"}},
	"repair_apply_repairs":         {map[string]map[string]any{"workspace": objectInput, "plan": objectInput, "model": stringInput}, []string{"workspace", "plan"}},
	"write_paper":                  {map[string]map[string]any{"folder_path": stringInput, "target_venue": stringInput, "field_hint": stringInput, "max_rounds": integerInput, "allow_web": booleanInput, "dry_run": booleanInput, "quality_threshold": numberInput, "plateau_delta": numberInput, "model": stringInput}, []string{"folder_path"}},
}

func reasonerInputSchema(name string) json.RawMessage {
	contract, ok := reasonerInputContracts[name]
	if !ok {
		panic("missing Python input schema contract for " + name)
	}
	schema := map[string]any{"type": "object", "properties": contract.properties}
	if len(contract.required) > 0 {
		schema["required"] = contract.required
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

var _ = pipeline.WriteRequest{}
