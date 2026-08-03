package node

import (
	"context"
	"encoding/json"

	"github.com/Agent-Field/agentfield/sdk/go/agent"
	"github.com/Agent-Field/preprint-af/go/internal/afx"
	"github.com/Agent-Field/preprint-af/go/internal/pipeline"
)

func (n *Node) RegisterAll() {
	regReasoner(n, "intake_prepare_workspace", []string{"intake"}, n.Pipeline.PrepareWorkspace)
	regReasoner(n, "intake_build_evidence_ledger", []string{"intake"}, n.Pipeline.BuildEvidenceLedger)
	regReasoner(n, "positioning_generate_frames", []string{"positioning"}, n.Pipeline.GenerateFrames)
	regReasoner(n, "positioning_judge_frame", []string{"positioning"}, n.Pipeline.JudgeFrame)
	regReasoner(n, "positioning_scan_novelty", []string{"positioning"}, n.Pipeline.ScanNovelty)
	regReasoner(n, "positioning_run_positioning", []string{"positioning"}, n.Pipeline.RunPositioning)
	regReasoner(n, "blueprint_design_blueprint", []string{"blueprint"}, n.Pipeline.DesignBlueprint)
	regReasoner(n, "build_write_section", []string{"build"}, n.Pipeline.WriteSection)
	regReasoner(n, "build_build_figure", []string{"build"}, n.Pipeline.BuildFigure)
	regReasoner(n, "build_build_bibliography", []string{"build"}, n.Pipeline.BuildBibliography)
	regReasoner(n, "build_run_build", []string{"build"}, n.Pipeline.RunBuild)
	regReasoner(n, "latex_compile_paper", []string{"latex"}, n.Pipeline.CompilePaper)
	regReasoner(n, "critique_persona_review", []string{"critique"}, n.Pipeline.PersonaReview)
	regReasoner(n, "critique_narrative_review", []string{"critique"}, n.Pipeline.NarrativeReview)
	regReasoner(n, "critique_fidelity_audit", []string{"critique"}, n.Pipeline.FidelityAudit)
	regReasoner(n, "critique_run_critique", []string{"critique"}, n.Pipeline.RunCritique)
	regReasoner(n, "repair_plan_repairs", []string{"repair"}, n.Pipeline.PlanRepairs)
	regReasoner(n, "repair_apply_repairs", []string{"repair"}, n.Pipeline.ApplyRepairs)
	regReasoner(n, "write_paper", []string{"entry", "workflow"}, n.Pipeline.WritePaper,
		agent.WithInputSchema(writePaperInputSchema))
	// Additive visual-quality subgraph. The original 19 public reasoners and
	// their prompts remain intact; build_build_figure composes these children so
	// ideation, rendering, and image critique are visible in the AgentField DAG.
	regReasoner(n, "figure_ideate", []string{"build", "figure", "vision"}, n.Pipeline.IdeateFigure)
	regReasoner(n, "figure_render", []string{"build", "figure"}, n.Pipeline.RenderFigure)
	regReasoner(n, "figure_review", []string{"build", "figure", "vision", "verification"}, n.Pipeline.ReviewFigure)
	regReasoner(n, "factual_audit_scope", []string{"factual", "verification"}, n.Pipeline.AuditFactualScope)
	regReasoner(n, "factual_run_precompile_gate", []string{"factual", "verification", "orchestration"}, n.Pipeline.RunFactualGate)
}

func regReasoner[T any](n *Node, name string, tags []string, fn func(context.Context, T) (any, error), extra ...agent.ReasonerOption) {
	opts := []agent.ReasonerOption{
		agent.WithReasonerTags(tags...),
		agent.WithDescription("Go preprint-af capability: " + name + "."),
	}
	opts = append(opts, extra...)
	n.App.RegisterReasoner(name, func(ctx context.Context, input map[string]any) (any, error) {
		in, err := afx.Bind[T](input)
		if err != nil {
			return nil, err
		}
		return fn(ctx, in)
	}, opts...)
	n.registered = append(n.registered, name)
}

func (n *Node) RegisteredNames() []string { return append([]string(nil), n.registered...) }

var writePaperInputSchema = json.RawMessage(`{
  "type":"object",
  "properties":{
    "folder_path":{"type":"string","description":"Folder with the user's research: data, results, drafts, or an existing paper."},
    "target_venue":{"type":["string","null"]},
    "field_hint":{"type":["string","null"]},
    "max_rounds":{"type":"integer","minimum":1,"maximum":8,"default":6},
    "allow_web":{"type":"boolean","default":true},
    "show_todos":{"type":"boolean","default":false,"description":"Render TODO boxes in the PDF. Defaults to false; unresolved work remains in TODO.md and REVIEW.md."},
    "dry_run":{"type":"boolean","default":false},
    "quality_threshold":{"type":"number","minimum":0,"maximum":1,"default":0.9},
    "plateau_delta":{"type":"number","minimum":0,"maximum":0.1,"default":0.01},
    "model":{"type":["string","null"]}
  },
  "required":["folder_path"],
  "additionalProperties":false
}`)

var _ = pipeline.WriteRequest{}
