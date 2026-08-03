package pipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

const (
	maxFactualConcurrency = 8
	maxFactualFindings    = 24
)

type AuditFactualScopeInput struct {
	Workspace Workspace `json:"workspace"`
	Target    string    `json:"target"`
	File      string    `json:"file"`
	RoundNo   int       `json:"round_no"`
	Model     *string   `json:"model"`
}

type RunFactualGateInput struct {
	Workspace Workspace `json:"workspace"`
	Model     *string   `json:"model"`
}

type factualScope struct {
	Target string
	File   string
}

var (
	evidenceHeadingRE = regexp.MustCompile(`(?m)^###\s+(E\d+)\s*$`)
	evidenceRefRE     = regexp.MustCompile(`\bE\d+\b`)
	digitRE           = regexp.MustCompile(`\d`)
)

func EvidenceLedgerValid(ws Workspace, summary EvidenceSummary) (bool, string) {
	body := ReadText(ws.EvidencePath, 0)
	facts := evidenceHeadingRE.FindAllStringSubmatch(body, -1)
	if len(facts) == 0 {
		return false, "EVIDENCE.md contains no E<n> facts"
	}
	seen := map[string]bool{}
	for _, fact := range facts {
		if seen[fact[1]] {
			return false, "EVIDENCE.md contains duplicate fact " + fact[1]
		}
		seen[fact[1]] = true
	}
	if summary.FactCount <= 0 {
		return false, "evidence summary reports zero facts"
	}
	if !summary.Confident {
		return false, "evidence intake did not report confidence"
	}
	return true, ""
}

func factualScopes(ws Workspace) ([]factualScope, error) {
	scopes := []factualScope{{Target: "front_matter", File: "paper/main.tex"}}
	files, err := filepath.Glob(filepath.Join(ws.SectionsDir, "*.tex"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	seen := map[string]bool{"front_matter": true}
	for _, abs := range files {
		rel, err := filepath.Rel(ws.Root, abs)
		if err != nil {
			return nil, err
		}
		target := slugOf(abs)
		if target == "" || seen[target] {
			return nil, fmt.Errorf("duplicate or empty factual target %q", target)
		}
		seen[target] = true
		scopes = append(scopes, factualScope{Target: target, File: filepath.ToSlash(rel)})
	}
	return scopes, nil
}

func safePaperPath(ws Workspace, rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe manuscript path %q", rel)
	}
	abs := filepath.Join(ws.Root, clean)
	within, err := filepath.Rel(ws.PaperDir, abs)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("manuscript path escapes paper directory: %q", rel)
	}
	return abs, nil
}

func ledgerIDs(evidence string) map[string]bool {
	ids := map[string]bool{}
	for _, match := range evidenceHeadingRE.FindAllStringSubmatch(evidence, -1) {
		ids[match[1]] = true
	}
	return ids
}

func normalizeAudit(a FactualScopeAudit, scope factualScope, allowed map[string]bool) FactualScopeAudit {
	if scope.Target != "cross_document" {
		a.Target = scope.Target
	}
	if a.Findings == nil {
		a.Findings = []FactualFinding{}
	}
	if len(a.Findings) > maxFactualFindings {
		a.Findings = a.Findings[:maxFactualFindings]
	}
	for i := range a.Findings {
		f := &a.Findings[i]
		if scope.Target != "cross_document" {
			f.Target = scope.Target
			f.File = scope.File
		} else if !allowed[f.Target] {
			f.Blocking = true
			f.Explanation = strings.TrimSpace(f.Explanation + " Cross-document finding did not identify a valid exact target.")
		}
		if f.EvidenceIDs == nil {
			f.EvidenceIDs = []string{}
		}
		if f.SourcePaths == nil {
			f.SourcePaths = []string{}
		}
	}
	return a
}

func deterministicFactualFindings(target, file, body, evidence string, bib []string) []FactualFinding {
	knownEvidence := ledgerIDs(evidence)
	out := []FactualFinding{}
	seenRef := map[string]bool{}
	for _, ref := range evidenceRefRE.FindAllString(body, -1) {
		if seenRef[ref] {
			continue
		}
		seenRef[ref] = true
		if !knownEvidence[ref] {
			out = append(out, FactualFinding{Target: target, File: file, Kind: "invalid_evidence_trace", Claim: ref, EvidenceIDs: []string{ref}, SourcePaths: []string{}, Explanation: ref + " is cited by the manuscript but absent from EVIDENCE.md.", RepairInstruction: "Remove the invalid evidence id or replace the claim with wording supported by an existing ledger fact.", Blocking: true})
		}
	}
	for _, key := range inventedCiteKeys(body, bib) {
		out = append(out, FactualFinding{Target: target, File: file, Kind: "citation", Claim: `\cite{` + key + `}`, EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: "Citation key is absent from paper/refs.bib.", RepairInstruction: "Remove the citation or replace it only with a verified key already present in paper/refs.bib.", Blocking: true})
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !digitRE.MatchString(trimmed) || strings.HasPrefix(trimmed, "%") || strings.Contains(trimmed, `\todobox{`) || structuralLatexLine(trimmed) {
			continue
		}
		if strings.Contains(line, "% E") || nextEvidenceComment(lines, i+1) {
			continue
		}
		out = append(out, FactualFinding{Target: target, File: file, Line: i + 1, Kind: "missing_evidence_trace", Claim: strings.TrimSpace(line), EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: "Numeric manuscript line has no adjacent E<n> evidence trace.", RepairInstruction: "Verify the quantitative claim against EVIDENCE.md, then add its real E<n> comment on the next line; otherwise narrow, remove, or replace it with a todobox.", Blocking: true})
		if len(out) >= maxFactualFindings {
			break
		}
	}
	return dedupeFactualFindings(out)
}

func structuralLatexLine(line string) bool {
	for _, prefix := range []string{`\section`, `\subsection`, `\label`, `\includegraphics`, `\begin`, `\end`, `\input`, `\documentclass`, `\usepackage`, `\newcommand`, `\renewcommand`, `\newtcolorbox`, `\graphicspath`, `\title`, `\bibliographystyle`, `\bibliography`, `\author`, `\date`} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func nextEvidenceComment(lines []string, start int) bool {
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		return strings.HasPrefix(line, "%") && evidenceRefRE.MatchString(line)
	}
	return false
}

func dedupeFactualFindings(in []FactualFinding) []FactualFinding {
	seen := map[string]int{}
	out := []FactualFinding{}
	for _, f := range in {
		key := strings.Join([]string{f.Target, f.File, fmt.Sprint(f.Line), f.Kind, f.Claim}, "\x00")
		if at, ok := seen[key]; ok {
			out[at].Blocking = out[at].Blocking || f.Blocking
			continue
		}
		seen[key] = len(out)
		out = append(out, f)
	}
	return out
}

func (s *Service) AuditFactualScope(ctx context.Context, in AuditFactualScopeInput) (any, error) {
	evidence := ReadText(in.Workspace.EvidencePath, 60000)
	bib := parseBibKeys(in.Workspace.PaperDir)
	scope := factualScope{Target: in.Target, File: in.File}
	body := ""
	if in.Target == "cross_document" {
		body = truncate(AssemblePaperText(in.Workspace.PaperDir), prompts.PaperCap)
		scope.File = "paper/main.tex + paper/sections/*.tex"
	} else {
		abs, err := safePaperPath(in.Workspace, in.File)
		if err != nil {
			return nil, err
		}
		body = ReadText(abs, 0)
		if body == "" {
			return FactualScopeAudit{Target: in.Target, Findings: []FactualFinding{{Target: in.Target, File: in.File, Kind: "missing_scope", Claim: in.File, EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: "Manuscript scope is missing or empty.", RepairInstruction: "Restore the required manuscript file before compilation.", Blocking: true}}, Score: 0, Confident: false}, nil
		}
	}
	system := prompts.FactualScopeSystem
	user := prompts.FactualScopePrompt(in.Target, scope.File, evidence, prettyJSON(bib), body, in.RoundNo)
	audit, err := aiInto[FactualScopeAudit](ctx, s, system, user, stringValue(in.Model))
	allowed := map[string]bool{}
	if scopes, scopeErr := factualScopes(in.Workspace); scopeErr == nil {
		for _, item := range scopes {
			allowed[item.Target] = true
		}
	}
	if err != nil {
		return FactualScopeAudit{Target: in.Target, Findings: []FactualFinding{{Target: in.Target, File: scope.File, Kind: "audit_failure", Claim: "factual audit", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: err.Error(), RepairInstruction: "Re-run the factual audit successfully before compilation.", Blocking: true}}, Score: 0, Confident: false}, nil
	}
	audit = normalizeAudit(audit, scope, allowed)
	if in.Target != "cross_document" {
		audit.Findings = dedupeFactualFindings(append(audit.Findings, deterministicFactualFindings(in.Target, in.File, body, evidence, bib)...))
	}
	return audit, nil
}

func runFactualAudits(ctx context.Context, s *Service, ws Workspace, scopes []factualScope, round int, model *string) []FactualScopeAudit {
	all := append(append([]factualScope{}, scopes...), factualScope{Target: "cross_document", File: ""})
	results := make([]FactualScopeAudit, len(all))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxFactualConcurrency)
	for i, scope := range all {
		i, scope := i, scope
		g.Go(func() error {
			v, err := callInto[FactualScopeAudit](gctx, s, "factual_audit_scope", AuditFactualScopeInput{Workspace: ws, Target: scope.Target, File: scope.File, RoundNo: round, Model: model})
			if err != nil {
				v = FactualScopeAudit{Target: scope.Target, Findings: []FactualFinding{{Target: scope.Target, File: scope.File, Kind: "audit_failure", Claim: "factual audit", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: err.Error(), RepairInstruction: "Re-run the factual audit successfully before compilation.", Blocking: true}}, Confident: false}
			}
			results[i] = v
			return nil
		})
	}
	_ = g.Wait()
	return results
}

func auditState(audits []FactualScopeAudit) (bool, []FactualFinding) {
	confident := true
	remaining := []FactualFinding{}
	for _, audit := range audits {
		confident = confident && audit.Confident
		for _, finding := range audit.Findings {
			if finding.Blocking {
				remaining = append(remaining, finding)
			}
		}
	}
	return confident, dedupeFactualFindings(remaining)
}

func factualRepairPlan(audits []FactualScopeAudit, allowed map[string]bool) RepairPlan {
	byTarget := map[string][]string{}
	for _, audit := range audits {
		for _, finding := range audit.Findings {
			if !finding.Blocking || !allowed[finding.Target] {
				continue
			}
			instruction := strings.TrimSpace(finding.RepairInstruction)
			if instruction == "" {
				instruction = "Resolve factual finding without inventing evidence: " + finding.Explanation
			}
			byTarget[finding.Target] = append(byTarget[finding.Target], instruction)
		}
	}
	targets := make([]string, 0, len(byTarget))
	for target := range byTarget {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i] == "front_matter" {
			return true
		}
		if targets[j] == "front_matter" {
			return false
		}
		return targets[i] < targets[j]
	})
	plan := RepairPlan{Tasks: []RepairTask{}, Notes: "deterministic pre-compile factual repair wave", Confident: true}
	for _, target := range targets {
		if len(plan.Tasks) == maxRepairTasks {
			break
		}
		instructions := uniqueStrings(byTarget[target])
		if len(instructions) > 12 {
			instructions = instructions[:12]
		}
		plan.Tasks = append(plan.Tasks, RepairTask{Target: target, Instructions: instructions, Priority: "high"})
	}
	return plan
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func scopeHashes(ws Workspace, scopes []factualScope) map[string][32]byte {
	out := map[string][32]byte{}
	for _, scope := range scopes {
		if abs, err := safePaperPath(ws, scope.File); err == nil {
			out[scope.Target] = sha256.Sum256([]byte(ReadText(abs, 0)))
		}
	}
	return out
}

func writeFactualGateArtifacts(ws Workspace, report FactualGateReport) string {
	jsonPath := filepath.Join(ws.ReviewsDir, "factual-gate.json")
	_, _ = WriteText(jsonPath, prettyJSON(report)+"\n")
	status := "FAILED"
	if report.Passed {
		status = "PASSED"
	}
	var body strings.Builder
	fmt.Fprintf(&body, "# Pre-compile factual gate — %s\n\nRepairs applied: %d  \nConfident: %t\n\n## Remaining blocking findings\n\n", status, report.RepairsApplied, report.Confident)
	if len(report.Remaining) == 0 {
		body.WriteString("- none\n")
	} else {
		for _, finding := range report.Remaining {
			fmt.Fprintf(&body, "- `%s` %s:%d — %s\n", finding.Target, finding.File, finding.Line, defaultText(finding.Explanation, finding.Claim))
		}
	}
	mdPath := filepath.Join(ws.ReviewsDir, "factual-gate.md")
	_, _ = WriteText(mdPath, body.String())
	return mdPath
}

func (s *Service) RunFactualGate(ctx context.Context, in RunFactualGateInput) (any, error) {
	report := NewFactualGateReport()
	scopes, err := factualScopes(in.Workspace)
	if err != nil || len(scopes) < 2 {
		reason := "no manuscript sections available for factual audit"
		if err != nil {
			reason = err.Error()
		}
		report.Remaining = []FactualFinding{{Target: "front_matter", File: "paper/main.tex", Kind: "gate_failure", Claim: "manuscript scopes", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: reason, RepairInstruction: "Restore the complete manuscript before compilation.", Blocking: true}}
		writeFactualGateArtifacts(in.Workspace, report)
		return report, nil
	}
	allowed := map[string]bool{}
	for _, scope := range scopes {
		allowed[scope.Target] = true
	}
	report.Initial = runFactualAudits(ctx, s, in.Workspace, scopes, 1, in.Model)
	_, initialBlocking := auditState(report.Initial)
	plan := factualRepairPlan(report.Initial, allowed)
	if len(initialBlocking) > 0 && len(plan.Tasks) > 0 {
		before := scopeHashes(in.Workspace, scopes)
		applied, callErr := callInto[int](ctx, s, "repair_apply_repairs", ApplyRepairsInput{Workspace: in.Workspace, Plan: plan, Model: in.Model})
		if callErr != nil {
			report.Remaining = append(report.Remaining, FactualFinding{Target: "front_matter", File: "paper/main.tex", Kind: "repair_failure", Claim: "factual repair wave", EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: callErr.Error(), RepairInstruction: "Re-run the factual repair wave successfully.", Blocking: true})
		} else {
			report.RepairsApplied = applied
			after := scopeHashes(in.Workspace, scopes)
			for _, task := range plan.Tasks {
				if before[task.Target] == after[task.Target] {
					report.Remaining = append(report.Remaining, FactualFinding{Target: task.Target, Kind: "repair_no_change", Claim: task.Target, EvidenceIDs: []string{}, SourcePaths: []string{}, Explanation: "Factual repair task reported no verified change to its exact target.", RepairInstruction: "Apply the factual repair to the target file and verify the file changed.", Blocking: true})
				}
			}
			report.Reaudit = runFactualAudits(ctx, s, in.Workspace, scopes, 2, in.Model)
		}
	}
	finalAudits := report.Initial
	if len(report.Reaudit) > 0 {
		finalAudits = report.Reaudit
	}
	finalConfident, finalBlocking := auditState(finalAudits)
	report.Remaining = dedupeFactualFindings(append(report.Remaining, finalBlocking...))
	for _, audit := range finalAudits {
		for _, finding := range audit.Findings {
			if strings.HasPrefix(finding.Kind, "invalid_") || strings.HasPrefix(finding.Kind, "missing_evidence_") || finding.Kind == "citation" {
				report.DeterministicFindings = append(report.DeterministicFindings, finding)
			}
		}
	}
	report.DeterministicFindings = dedupeFactualFindings(report.DeterministicFindings)
	report.Confident = finalConfident
	report.Passed = report.Confident && len(report.Remaining) == 0
	writeFactualGateArtifacts(in.Workspace, report)
	GitSnapshot(in.Workspace.Root, fmt.Sprintf("P3 factual gate passed=%t", report.Passed))
	return report, nil
}
