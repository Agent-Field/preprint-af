package prompts

import "testing"

func TestRepairPlanPromptExact(t *testing.T) {
	system, user := RepairPlanPrompt("{\n  \"fidelity\": {}\n}")
	if system != RepairPlanSystem {
		t.Fatalf("system prompt mismatch:\n%q", system)
	}
	want := "Critique digest (compacted CritiqueBundle):\n{\n  \"fidelity\": {}\n}\n\nReturn a RepairPlan with tasks (target, instructions, priority), notes, confident."
	if user != want {
		t.Fatalf("user prompt mismatch:\n%q\nwant:\n%q", user, want)
	}
}

func TestRepairTaskPromptExact(t *testing.T) {
	got := RepairTaskPrompt(
		[]string{"paper/main.tex", "paper/refs.bib"},
		[]string{"Fix the title.", "Correct the citation."},
	)
	want := "You may modify ONLY these files: paper/main.tex, paper/refs.bib.\n\nApply exactly these instructions:\n1. Fix the title.\n2. Correct the citation.\n\nObey AGENTS.md. Never change evidence-backed numbers unless an instruction states the number contradicts EVIDENCE.md. Preserve \\todobox items unless an instruction resolves one."
	if got != want {
		t.Fatalf("repair task prompt mismatch:\n%q\nwant:\n%q", got, want)
	}
}
