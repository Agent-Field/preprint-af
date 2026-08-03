package pipeline

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

func (s *Service) WritePaper(ctx context.Context, req WriteRequest) (any, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if !req.DryRun {
		if err := runtimePreflight(); err != nil {
			return nil, err
		}
	}
	ws, err := callInto[Workspace](ctx, s, "intake_prepare_workspace", PrepareWorkspaceInput{FolderPath: req.FolderPath, Model: req.Model})
	if err != nil {
		return nil, err
	}
	evidence, err := callInto[EvidenceSummary](ctx, s, "intake_build_evidence_ledger", BuildEvidenceInput{Workspace: ws, Model: req.Model})
	if err != nil {
		return nil, err
	}
	if ok, reason := EvidenceLedgerValid(ws, evidence); !ok {
		report := NewFactualGateReport()
		report.Remaining = []FactualFinding{{Target: "front_matter", File: "EVIDENCE.md", Kind: "evidence_intake_failure", Claim: "evidence ledger", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: reason, RepairInstruction: "Correct the source material or evidence intake, then rerun the paper from the beginning.", Blocking: true}}
		reviewPath := writeFactualGateArtifacts(ws, report)
		return WriteResult{Status: "incomplete", RunID: ws.RunID, Workspace: ws.Root, Rounds: []RoundRecord{}, StopReason: "evidence_quality_gate_failed", TODOPath: ws.TODOPath, ReviewPath: reviewPath, PositioningPath: ws.PositioningPath}, nil
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P0", "fact_count": evidence.FactCount, "confident": evidence.Confident})
	GitSnapshot(ws.Root, "P0 evidence")
	decision, err := callInto[PositioningDecision](ctx, s, "positioning_run_positioning", RunPositioningInput{Workspace: ws, TargetVenue: req.TargetVenue, FieldHint: req.FieldHint, AllowWeb: req.AllowWeb, Model: req.Model})
	if err != nil {
		return nil, err
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P1", "title": decision.FinalTitle, "frame": decision.WinningFrameName})
	GitSnapshot(ws.Root, "P1 positioning")
	bp, err := callInto[Blueprint](ctx, s, "blueprint_design_blueprint", DesignBlueprintInput{Workspace: ws, TargetVenue: req.TargetVenue, Model: req.Model})
	if err != nil {
		return nil, err
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P2", "sections": len(bp.Sections), "figures": len(bp.Figures)})
	GitSnapshot(ws.Root, "P2 blueprint")
	if req.DryRun {
		return WriteResult{Status: "planned", RunID: ws.RunID, Workspace: ws.Root, Title: decision.FinalTitle, Rounds: []RoundRecord{}, StopReason: "dry_run", TODOPath: ws.TODOPath, PositioningPath: ws.PositioningPath}, nil
	}
	build, err := callInto[BuildReport](ctx, s, "build_run_build", RunBuildInput{Workspace: ws, Blueprint: bp, AllowWeb: req.AllowWeb, Model: req.Model})
	if err != nil {
		return nil, err
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P3", "confident": build.Confident})
	GitSnapshot(ws.Root, "P3 build")
	factual, factualErr := callInto[FactualGateReport](ctx, s, "factual_run_precompile_gate", RunFactualGateInput{Workspace: ws, Model: req.Model})
	if factualErr != nil {
		report := NewFactualGateReport()
		report.Remaining = []FactualFinding{{Target: "front_matter", File: "paper/main.tex", Kind: "factual_gate_failure", Claim: "pre-compile factual gate", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: factualErr.Error(), RepairInstruction: "Run the pre-compile factual gate successfully before compilation.", Blocking: true}}
		reviewPath := writeFactualGateArtifacts(ws, report)
		return WriteResult{Status: "incomplete", RunID: ws.RunID, Workspace: ws.Root, Title: decision.FinalTitle, Rounds: []RoundRecord{}, StopReason: "precompile_factual_gate_failed", TODOPath: ws.TODOPath, ReviewPath: reviewPath, PositioningPath: ws.PositioningPath}, nil
	}
	if !factual.Passed {
		reviewPath := filepath.Join(ws.ReviewsDir, "factual-gate.md")
		return WriteResult{Status: "incomplete", RunID: ws.RunID, Workspace: ws.Root, Title: decision.FinalTitle, Rounds: []RoundRecord{}, StopReason: "precompile_factual_gate_failed", TODOPath: ws.TODOPath, ReviewPath: reviewPath, PositioningPath: ws.PositioningPath}, nil
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P3-factual", "passed": factual.Passed, "repairs_applied": factual.RepairsApplied})
	compile, err := callInto[CompileReport](ctx, s, "latex_compile_paper", CompilePaperInput{Workspace: ws, ShowTODOs: req.ShowTODOs, Model: req.Model})
	if err != nil {
		return nil, err
	}
	_ = SaveState(ws.Root, map[string]any{"phase": "P4", "compile_ok": compile.Success})
	GitSnapshot(ws.Root, "P4 compile")
	records := []RoundRecord{}
	var prev *float64
	noImprove := 0
	bestScore := -1.0
	bestRound := 0
	stopReason := "safety_cap_reached"
	var bundle *CritiqueBundle
	for round := 1; round <= req.MaxRounds; round++ {
		b, e := callInto[CritiqueBundle](ctx, s, "critique_run_critique", RunCritiqueInput{Workspace: ws, RoundNo: round, Model: req.Model})
		if e != nil {
			return nil, e
		}
		bundle = &b
		personaScore := .5
		if len(b.PersonaReviews) > 0 {
			risk := 0.0
			for _, p := range b.PersonaReviews {
				risk += p.AcceptanceRisk
			}
			personaScore = 1 - risk/float64(len(b.PersonaReviews))
		}
		total := round4(.35*personaScore + .25*b.Narrative.Score + .25*b.Fidelity.Score + .15*b.Slop.Score)
		record := RoundRecord{Round: round, TotalScore: total, PersonaScore: round4(personaScore), NarrativeScore: round4(b.Narrative.Score), FidelityScore: round4(b.Fidelity.Score), SlopScore: round4(b.Slop.Score), CompileOK: compile.Success, StopReason: "continue"}
		records = append(records, record)
		idx := len(records) - 1
		if total > bestScore {
			bestScore = total
			bestRound = round
		}
		GitSnapshot(ws.Root, fmt.Sprintf("round %d critique score=%v", round, total))
		persist := func() {
			r := records[idx]
			_ = SaveState(ws.Root, map[string]any{"phase": "critique", "round": round, "total_score": total, "persona_score": r.PersonaScore, "narrative_score": r.NarrativeScore, "fidelity_score": r.FidelityScore, "slop_score": r.SlopScore, "compile_ok": compile.Success, "repairs_applied": r.RepairsApplied, "stop": r.Stop, "stop_reason": r.StopReason})
		}
		if build.Confident && compile.Success && total >= req.QualityThreshold && !b.Fidelity.Blocking {
			records[idx].Stop = true
			records[idx].StopReason = "quality_threshold_met"
			stopReason = records[idx].StopReason
			persist()
			break
		}
		if prev != nil && total-*prev < req.PlateauDelta {
			noImprove++
			if noImprove >= 2 {
				records[idx].Stop = true
				records[idx].StopReason = "quality_plateau"
				stopReason = records[idx].StopReason
				persist()
				break
			}
		} else {
			noImprove = 0
		}
		// Never mutate the manuscript after the final independent review. This
		// keeps the returned PDF and factual verdict on the same exact source.
		if round == req.MaxRounds {
			records[idx].Stop = true
			records[idx].StopReason = "safety_cap_reached"
			stopReason = records[idx].StopReason
			persist()
			break
		}
		plan, e := callInto[RepairPlan](ctx, s, "repair_plan_repairs", PlanRepairsInput{Workspace: ws, Critique: b, Model: req.Model})
		if e != nil {
			return nil, e
		}
		if len(plan.Tasks) == 0 {
			records[idx].Stop = true
			records[idx].StopReason = "no_repairs_needed"
			stopReason = records[idx].StopReason
			persist()
			break
		}
		applied, e := callInto[int](ctx, s, "repair_apply_repairs", ApplyRepairsInput{Workspace: ws, Plan: plan, Model: req.Model})
		if e != nil {
			return nil, e
		}
		records[idx].RepairsApplied = applied
		// General reviewer repairs can change factual claims. Run the same
		// parallel, independently re-audited gate before compiling those edits.
		factual, e = callInto[FactualGateReport](ctx, s, "factual_run_precompile_gate", RunFactualGateInput{Workspace: ws, Model: req.Model})
		if e != nil || !factual.Passed {
			if _, syncErr := SyncManuscriptTODOs(ws); syncErr != nil {
				return nil, syncErr
			}
			reason := "precompile_factual_gate_failed_after_repair"
			return WriteResult{Status: "incomplete", RunID: ws.RunID, Workspace: ws.Root, Title: decision.FinalTitle, Rounds: records, StopReason: reason, TODOPath: ws.TODOPath, ReviewPath: filepath.Join(ws.ReviewsDir, "factual-gate.md"), PositioningPath: ws.PositioningPath}, nil
		}
		compile, e = callInto[CompileReport](ctx, s, "latex_compile_paper", CompilePaperInput{Workspace: ws, ShowTODOs: req.ShowTODOs, Model: req.Model})
		if e != nil {
			return nil, e
		}
		v := total
		prev = &v
		persist()
	}
	if _, err := SyncManuscriptTODOs(ws); err != nil {
		return nil, err
	}
	todo := ReadText(ws.TODOPath, 0)
	review := composeReview(bundle, records, stopReason, todo)
	reviewPath, _ := WriteText(filepath.Join(ws.Root, "REVIEW.md"), review)
	GitSnapshot(ws.Root, "REVIEW")
	final := 0.0
	if bestRound > 0 {
		final = bestScore
	}
	pdf := ""
	if compile.Success {
		pdf = compile.PDFPath
	}
	status := "completed"
	if !build.Confident {
		status = "incomplete"
		stopReason = "figure_or_build_quality_gate_failed"
	} else if !compile.Success {
		status = "incomplete"
		stopReason = "compile_gate_failed"
	} else if bundle == nil || bundle.Fidelity.Blocking || !bundle.Fidelity.Confident {
		status = "incomplete"
		stopReason = "factual_fidelity_gate_failed"
	}
	return WriteResult{Status: status, RunID: ws.RunID, Workspace: ws.Root, PDFPath: pdf, Title: decision.FinalTitle, Rounds: records, FinalScore: final, StopReason: stopReason, TODOPath: ws.TODOPath, ReviewPath: reviewPath, PositioningPath: ws.PositioningPath}, nil
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
func composeReview(bundle *CritiqueBundle, records []RoundRecord, stop, todo string) string {
	majors := []string{}
	fidelity := []string{}
	narrative := []string{}
	slop := 0
	if bundle != nil {
		for _, p := range bundle.PersonaReviews {
			for _, x := range p.Issues {
				if x.Severity == "major" {
					majors = append(majors, fmt.Sprintf("[%s / %s] %s — fix: %s", p.Persona, x.Section, x.Issue, x.FixHint))
				}
			}
		}
		fidelity = append(fidelity, bundle.Fidelity.UnsupportedClaims...)
		fidelity = append(fidelity, bundle.Fidelity.NumberMismatches...)
		fidelity = append(fidelity, bundle.Fidelity.CitationIssues...)
		for _, x := range bundle.Narrative.TransitionIssues {
			narrative = append(narrative, fmt.Sprintf("[%s] %s", x.Section, x.Issue))
		}
		narrative = append(narrative, bundle.Narrative.PromiseAlignmentIssues...)
		slop = len(bundle.Slop.Violations)
	}
	return fmt.Sprintf("# REVIEW — unresolved problems\n\nStop reason: **%s**\n\n## Unresolved major persona issues\n\n%s\n## Fidelity findings\n\n%s\n## Narrative issues\n\n%s\n## Slop residue\n\n%d remaining slop violation(s).\n\n## Per-round scores\n\n%s\n\n## TODO digest\n\n%s\n", stop, bullets(majors, "- none"), bullets(fidelity, "- none"), bullets(narrative, "- none"), slop, roundTable(records), defaultText(strings.TrimSpace(todo), "(TODO.md is empty)"))
}
func roundTable(records []RoundRecord) string {
	if len(records) == 0 {
		return "(no critique rounds ran)"
	}
	var b strings.Builder
	b.WriteString("| Round | Total | Persona | Narrative | Fidelity | Slop | Compile | Repairs | Stop |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range records {
		compile := "no"
		if r.CompileOK {
			compile = "yes"
		}
		stop := "-"
		if r.Stop {
			stop = r.StopReason
		}
		fmt.Fprintf(&b, "| %d | %.4f | %.4f | %.4f | %.4f | %.4f | %s | %d | %s |\n", r.Round, r.TotalScore, r.PersonaScore, r.NarrativeScore, r.FidelityScore, r.SlopScore, compile, r.RepairsApplied, stop)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
