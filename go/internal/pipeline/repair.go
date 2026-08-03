package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

const maxRepairTasks = 8

type PlanRepairsInput struct {
	Workspace Workspace      `json:"workspace"`
	Critique  CritiqueBundle `json:"critique"`
	Model     *string        `json:"model"`
}
type ApplyRepairsInput struct {
	Workspace Workspace  `json:"workspace"`
	Plan      RepairPlan `json:"plan"`
	Model     *string    `json:"model"`
}

type compactReview struct {
	Personas  []map[string]any `json:"personas"`
	Narrative map[string]any   `json:"narrative"`
	Fidelity  map[string]any   `json:"fidelity"`
	Slop      []string         `json:"slop"`
}

func compactCritique(c CritiqueBundle) compactReview {
	personas := make([]map[string]any, 0, len(c.PersonaReviews))
	for _, pr := range c.PersonaReviews {
		kept := []LocatedIssue{}
		minors := 0
		for _, x := range pr.Issues {
			if strings.EqualFold(x.Severity, "major") {
				kept = append(kept, x)
			} else if minors < 3 {
				kept = append(kept, x)
				minors++
			}
		}
		personas = append(personas, map[string]any{"acceptance_risk": pr.AcceptanceRisk, "verdict": pr.Verdict, "issues": kept})
	}
	trans := c.Narrative.TransitionIssues
	fid := map[string]any{"blocking": c.Fidelity.Blocking, "unsupported_claims": c.Fidelity.UnsupportedClaims, "number_mismatches": c.Fidelity.NumberMismatches, "citation_issues": c.Fidelity.CitationIssues}
	slop := make([]string, 0, len(c.Slop.Violations))
	for _, v := range c.Slop.Violations {
		slop = append(slop, fmt.Sprintf("%s:%d %s — %s", v.File, v.Line, v.Rule, v.Excerpt))
	}
	return compactReview{Personas: personas, Narrative: map[string]any{"transition_issues": trans, "promise_alignment_issues": c.Narrative.PromiseAlignmentIssues, "arc_assessment": c.Narrative.ArcAssessment}, Fidelity: fid, Slop: slop}
}

func critiqueDirty(c CritiqueBundle) bool {
	if c.Fidelity.Blocking || len(c.Fidelity.UnsupportedClaims)+len(c.Fidelity.NumberMismatches)+len(c.Fidelity.CitationIssues) > 0 || len(c.Narrative.TransitionIssues) > 0 {
		return true
	}
	for _, p := range c.PersonaReviews {
		for _, x := range p.Issues {
			if strings.EqualFold(x.Severity, "major") {
				return true
			}
		}
	}
	return false
}

func synthesizePlan(c CritiqueBundle, reason string) RepairPlan {
	by := map[string][]string{}
	add := func(k, v string) {
		if k == "" {
			k = "global"
		}
		by[k] = append(by[k], v)
	}
	for _, x := range append(append(append([]string{}, c.Fidelity.UnsupportedClaims...), c.Fidelity.NumberMismatches...), c.Fidelity.CitationIssues...) {
		add("global", "Resolve fidelity finding: "+x)
	}
	for _, p := range c.PersonaReviews {
		for _, x := range p.Issues {
			if strings.EqualFold(x.Severity, "major") {
				add(x.Section, strings.TrimSpace(x.Issue+" Fix: "+x.FixHint))
			}
		}
	}
	for _, x := range c.Narrative.TransitionIssues {
		add(x.Section, strings.TrimSpace("Transition: "+x.Issue+" Fix: "+x.FixHint))
	}
	for _, v := range c.Slop.Violations {
		target := "global"
		base := filepath.Base(v.File)
		if i := strings.Index(base, "_"); i >= 0 && strings.HasSuffix(base, ".tex") {
			target = strings.TrimSuffix(base[i+1:], ".tex")
		}
		add(target, fmt.Sprintf("Slop violation, apply mechanically: %s:%d %s — %s", v.File, v.Line, v.Rule, v.Excerpt))
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "global" {
			return true
		}
		if keys[j] == "global" {
			return false
		}
		return keys[i] < keys[j]
	})
	plan := RepairPlan{Tasks: []RepairTask{}, Notes: "deterministic synthesized plan (" + reason + ")"}
	for _, k := range keys {
		if len(plan.Tasks) >= maxRepairTasks {
			break
		}
		ins := by[k]
		if len(ins) > 12 {
			ins = ins[:12]
		}
		priority := "medium"
		if k == "global" {
			priority = "high"
		}
		plan.Tasks = append(plan.Tasks, RepairTask{Target: k, Instructions: ins, Priority: priority})
	}
	return plan
}

func (s *Service) PlanRepairs(ctx context.Context, in PlanRepairsInput) (any, error) {
	compact := compactCritique(in.Critique)
	system, user := prompts.RepairPlanPrompt(prettyJSON(compact))
	result, err := aiInto[RepairPlan](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		if critiqueDirty(in.Critique) {
			return synthesizePlan(in.Critique, "planner crashed: "+err.Error()), nil
		}
		return RepairPlan{Tasks: []RepairTask{}, Notes: err.Error()}, nil
	}
	seen := map[string]bool{}
	dedup := []RepairTask{}
	for _, t := range result.Tasks {
		if seen[t.Target] {
			continue
		}
		seen[t.Target] = true
		dedup = append(dedup, t)
		if len(dedup) == maxRepairTasks {
			break
		}
	}
	result.Tasks = dedup
	if len(result.Tasks) == 0 && critiqueDirty(in.Critique) {
		return synthesizePlan(in.Critique, "planner returned empty on dirty critique"), nil
	}
	return result, nil
}

func resolveRepair(task RepairTask, ws Workspace) ([]string, []string) {
	switch task.Target {
	case "front_matter":
		return []string{"paper/main.tex"}, []string{"paper/main.tex"}
	case "figures":
		return []string{"paper/figures/ (figure scripts and their pdfs)"}, []string{"paper/figures/"}
	case "bibliography":
		return []string{"paper/refs.bib", "paper/CITATIONS_LOG.md"}, []string{"paper/refs.bib", "paper/CITATIONS_LOG.md"}
	case "global":
		return []string{"paper/sections/ (any section file needed to resolve the findings)"}, []string{"paper/sections/", "paper/main.tex"}
	}
	matches, _ := filepath.Glob(filepath.Join(ws.SectionsDir, "*_"+task.Target+".tex"))
	if len(matches) > 0 {
		rels := make([]string, len(matches))
		for i, m := range matches {
			r, _ := filepath.Rel(ws.Root, m)
			rels[i] = filepath.ToSlash(r)
		}
		return rels, rels
	}
	return []string{"paper/sections/NN_" + task.Target + ".tex"}, []string{"paper/sections/*_" + task.Target + ".tex", "paper/sections/"}
}

func changedMatches(changed []string, patterns []string) bool {
	for _, c := range changed {
		c = filepath.ToSlash(c)
		for _, p := range patterns {
			if strings.HasSuffix(p, "/") && strings.HasPrefix(c, p) {
				return true
			}
			ok, _ := filepath.Match(p, c)
			if c == p || strings.HasSuffix(c, "/"+p) || ok {
				return true
			}
		}
	}
	return false
}

func (s *Service) runRepair(ctx context.Context, ws Workspace, t RepairTask, model *string) (bool, error) {
	files, patterns := resolveRepair(t, ws)
	before := GitChangedFiles(ws.Root)
	prompt := prompts.RepairTaskPrompt(files, t.Instructions)
	_, hr, err := harnessInto[WorkerResult](ctx, s, prompt, stringValue(model), ws.Root, ws.Root)
	if err != nil {
		return false, err
	}
	if hr == nil || hr.IsError {
		return false, nil
	}
	after := GitChangedFiles(ws.Root)
	old := map[string]bool{}
	for _, x := range before {
		old[x] = true
	}
	fresh := []string{}
	for _, x := range after {
		if !old[x] {
			fresh = append(fresh, x)
		}
	}
	return changedMatches(fresh, patterns), nil
}

func (s *Service) ApplyRepairs(ctx context.Context, in ApplyRepairsInput) (any, error) {
	if len(in.Plan.Tasks) == 0 {
		return 0, nil
	}
	front := []RepairTask{}
	rest := []RepairTask{}
	for _, t := range in.Plan.Tasks {
		if t.Target == "front_matter" {
			front = append(front, t)
		} else {
			rest = append(rest, t)
		}
	}
	var applied int32
	if len(front) > 0 {
		ok, _ := s.runRepair(ctx, in.Workspace, front[0], in.Model)
		if ok {
			atomic.AddInt32(&applied, 1)
		}
	}
	g, gctx := errgroup.WithContext(ctx)
	for _, t := range rest {
		t := t
		g.Go(func() error {
			ok, _ := s.runRepair(gctx, in.Workspace, t, in.Model)
			if ok {
				atomic.AddInt32(&applied, 1)
			}
			return nil
		})
	}
	_ = g.Wait()
	GitSnapshot(in.Workspace.Root, fmt.Sprintf("repairs applied (%d tasks)", applied))
	return int(applied), nil
}
