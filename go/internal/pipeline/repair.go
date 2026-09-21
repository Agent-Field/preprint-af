package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

const maxRepairTasks = 8

var repairSectionRE = regexp.MustCompile(`\d+_([a-z0-9_]+)\.tex`)

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
	Personas  []compactPersona `json:"personas"`
	Narrative compactNarrative `json:"narrative"`
	Fidelity  compactFidelity  `json:"fidelity"`
	Slop      []string         `json:"slop"`
}

type compactPersona struct {
	AcceptanceRisk float64        `json:"acceptance_risk"`
	Verdict        string         `json:"verdict"`
	Issues         []LocatedIssue `json:"issues"`
}

type compactTransitionIssue struct {
	Section string `json:"section"`
	Issue   string `json:"issue"`
	FixHint string `json:"fix_hint"`
}

type compactNarrative struct {
	TransitionIssues       []compactTransitionIssue `json:"transition_issues"`
	PromiseAlignmentIssues []string                 `json:"promise_alignment_issues"`
	ArcAssessment          string                   `json:"arc_assessment"`
}

type compactFidelity struct {
	Blocking          bool     `json:"blocking"`
	UnsupportedClaims []string `json:"unsupported_claims"`
	NumberMismatches  []string `json:"number_mismatches"`
	CitationIssues    []string `json:"citation_issues"`
}

func compactCritique(c CritiqueBundle) compactReview {
	personas := make([]compactPersona, 0, len(c.PersonaReviews))
	for _, pr := range c.PersonaReviews {
		majors := []LocatedIssue{}
		minors := []LocatedIssue{}
		for _, x := range pr.Issues {
			if strings.EqualFold(x.Severity, "major") {
				majors = append(majors, x)
			} else {
				minors = append(minors, x)
			}
		}
		if len(minors) > 3 {
			minors = minors[:3]
		}
		kept := append(majors, minors...)
		personas = append(personas, compactPersona{AcceptanceRisk: pr.AcceptanceRisk, Verdict: pr.Verdict, Issues: kept})
	}
	transitions := make([]compactTransitionIssue, len(c.Narrative.TransitionIssues))
	for i, issue := range c.Narrative.TransitionIssues {
		transitions[i] = compactTransitionIssue{Section: issue.Section, Issue: issue.Issue, FixHint: issue.FixHint}
	}
	slop := make([]string, 0, len(c.Slop.Violations))
	for _, v := range c.Slop.Violations {
		slop = append(slop, fmt.Sprintf("%s:%d %s — %s", v.File, v.Line, v.Rule, v.Excerpt))
	}
	return compactReview{
		Personas: personas,
		Narrative: compactNarrative{
			TransitionIssues:       transitions,
			PromiseAlignmentIssues: c.Narrative.PromiseAlignmentIssues,
			ArcAssessment:          c.Narrative.ArcAssessment,
		},
		Fidelity: compactFidelity{
			Blocking:          c.Fidelity.Blocking,
			UnsupportedClaims: c.Fidelity.UnsupportedClaims,
			NumberMismatches:  c.Fidelity.NumberMismatches,
			CitationIssues:    c.Fidelity.CitationIssues,
		},
		Slop: slop,
	}
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
		if strings.Contains(filepath.ToSlash(v.File), "sections/") {
			if match := repairSectionRE.FindStringSubmatch(v.File); len(match) == 2 {
				target = match[1]
			}
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
	system, user := prompts.RepairPlanPrompt(pythonPrettyJSON(compact))
	result, err := aiInto[RepairPlan](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
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
	if task.Target == "global" {
		return []string{"paper/sections/ (any section file needed to resolve the findings)"}, []string{"paper/sections/", "paper/main.tex"}
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
	files, _ := resolveRepair(t, ws)
	prompt := prompts.RepairTaskPrompt(files, t.Instructions)
	_, hr, err := harnessInto[WorkerResult](ctx, s, prompt, stringValue(model), ws.Root, ws.Root)
	if err != nil && ctx.Err() != nil {
		return false, ctx.Err()
	}
	return err == nil && hr != nil && !hr.IsError, nil
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
	applied := 0
	if len(front) > 0 {
		task := front[0]
		before := GitChangedFiles(in.Workspace.Root)
		harnessOK, err := s.runRepair(ctx, in.Workspace, task, in.Model)
		if err != nil {
			return nil, err
		}
		after := GitChangedFiles(in.Workspace.Root)
		_, patterns := resolveRepair(task, in.Workspace)
		if harnessOK && changedMatches(newChangedFiles(before, after), patterns) {
			applied++
		}
	}
	if len(rest) > 0 {
		before := GitChangedFiles(in.Workspace.Root)
		results := make([]bool, len(rest))
		var g errgroup.Group
		for i, task := range rest {
			i, task := i, task
			g.Go(func() error {
				var err error
				results[i], err = s.runRepair(ctx, in.Workspace, task, in.Model)
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		newly := newChangedFiles(before, GitChangedFiles(in.Workspace.Root))
		for i, task := range rest {
			_, patterns := resolveRepair(task, in.Workspace)
			if results[i] && changedMatches(newly, patterns) {
				applied++
			}
		}
	}
	GitSnapshot(in.Workspace.Root, fmt.Sprintf("repairs applied (%d tasks)", applied))
	return applied, nil
}

func newChangedFiles(before, after []string) []string {
	old := make(map[string]bool, len(before))
	for _, path := range before {
		old[path] = true
	}
	newly := make([]string, 0, len(after))
	for _, path := range after {
		if !old[path] {
			newly = append(newly, path)
		}
	}
	return newly
}
