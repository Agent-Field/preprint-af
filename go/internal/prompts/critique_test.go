package prompts

import "testing"

func TestPersonaReviewPromptExact(t *testing.T) {
	system, user := PersonaReviewPrompt("PERSONA", 2, "PAPER")
	wantSystem := "PERSONA\n\n" + ReviewDirective
	wantUser := "Reviewing round 2. Here is the complete assembled manuscript (main.tex + all sections, with '--- <file> ---' markers):\n\nPAPER\n\nReturn your located issues, acceptance_risk, verdict, and confidence."
	if system != wantSystem {
		t.Fatalf("system prompt mismatch:\n%q\nwant:\n%q", system, wantSystem)
	}
	if user != wantUser {
		t.Fatalf("user prompt mismatch:\n%q\nwant:\n%q", user, wantUser)
	}
}

func TestNarrativeReviewPromptFallbacksExact(t *testing.T) {
	_, user := NarrativeReviewPrompt(1, "", "", "PAPER")
	want := "Round 1.\n\nPOSITIONING.md (the chosen frame, title, abstract):\n(POSITIONING.md unavailable)\n\nBLUEPRINT.md (section beats and transition contract):\n(BLUEPRINT.md unavailable)\n\nComplete assembled manuscript:\n\nPAPER\n\nReturn transition_issues, promise_alignment_issues, arc_assessment, score, confident."
	if user != want {
		t.Fatalf("user prompt mismatch:\n%q\nwant:\n%q", user, want)
	}
}

func TestFidelityAuditPromptPythonListFormatting(t *testing.T) {
	_, user := FidelityAuditPrompt(3, "", []string{"smith2024", "o'neil"}, "PAPER")
	want := "Round 3.\n\nEVIDENCE.md (the authoritative fact ledger):\n(EVIDENCE.md unavailable — treat all numbers as unsupported)\n\nBib keys present in paper/refs.bib (2): ['smith2024', \"o'neil\"]\n\nComplete assembled manuscript:\n\nPAPER\n\nReturn unsupported_claims, number_mismatches, citation_issues, blocking, score, confident."
	if user != want {
		t.Fatalf("user prompt mismatch:\n%q\nwant:\n%q", user, want)
	}
}
