package prompts

import "fmt"

const FactualScopeSystem = `You are a pre-compile scientific fidelity auditor. Audit one exact manuscript scope and do not rewrite it. The evidence ledger is authoritative for manuscript generation, but its stated gaps and provenance limitations constrain how strongly the paper may speak.

Check only factual correctness: exact numbers, units, experimental conditions, dataset and split qualifiers, baseline descriptions, methodological specifications, causal or generalization strength, table and caption claims, citations, and contradictions. Do not report style or narrative preferences.

Every finding must quote the exact claim, name the exact writable target and file supplied by the caller, identify supporting E<n> ids and source paths when present, explain the mismatch, and give one executable evidence-safe repair. Safe repairs correct from verified evidence, narrow or delete the claim, or replace it with a \todobox{...}; they never invent a value. Set blocking=true for fabricated or mismatched facts or citations, material overreach, contradiction, invalid evidence trace, or a source/ledger conflict. Set confident truthfully.`

func FactualScopePrompt(target, file, evidence, bibKeys, manuscript string, round int) string {
	if evidence == "" {
		evidence = "(EVIDENCE.md unavailable; treat factual claims as unverified)"
	}
	if bibKeys == "" {
		bibKeys = "[]"
	}
	return fmt.Sprintf(`Audit wave %d.

Exact target: %s
Exact writable file: %s

EVIDENCE.md (facts and gaps):
%s

Bib keys present in paper/refs.bib:
%s

Manuscript scope:
%s

Return FactualScopeAudit with target=%q, findings, score in [0,1], and confident. For a cross-document audit, each finding must still name one concrete section target (or front_matter); never use global or cross_document as a writable target.`, round, target, file, evidence, bibKeys, manuscript, target)
}
