package prompts

import "testing"

func TestLatexRepairPromptGolden(t *testing.T) {
	want := "LaTeX compilation failed. Fix ONLY LaTeX/compilation errors: undefined control sequences, missing\n" +
		"packages (prefer removing the dependency), math-mode errors, unescaped %, &, #, _ in text, missing\n" +
		"\\includegraphics files (replace with \\todobox{Figure <name> pending} if the pdf is genuinely\n" +
		"absent), bib issues. Do NOT change scientific content, numbers, citations, prose wording, or\n" +
		"section structure. Error log: ! Undefined control sequence"
	if got := LatexRepairPrompt("! Undefined control sequence"); got != want {
		t.Fatalf("repair prompt drifted\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBlueprintUserPromptGolden(t *testing.T) {
	want := "TARGET VENUE: NeurIPS\n\n" +
		"=== EVIDENCE.md (fact ledger; use these fact ids and figure candidates) ===\nE1: exact result\n\n" +
		"=== POSITIONING.md (the winning frame, final title/abstract, contribution order) ===\n# Title: Small Model\n\n" +
		"=== INPUT FILES AVAILABLE (a figure may set buildable=True ONLY if its data_sources are among these paths) ===\n" +
		"  - input/results.csv\n  - input/plot.py\n\n" +
		"Design the blueprint now. The story must follow the winning positioning frame: the section order and beats must deliver the promise made by the final title and abstract, in the contribution order chosen in POSITIONING.md. Produce a consistent establishes/requires chain across all sections. Return a Blueprint with sections, figures, citation_needs, venue_notes, and confident."
	got := BlueprintUserPrompt("E1: exact result", "# Title: Small Model", []string{"results.csv", "plot.py"}, "NeurIPS")
	if got != want {
		t.Fatalf("blueprint prompt drifted\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPythonReprMatchesPydanticModelDumpStyle(t *testing.T) {
	frame := StoryFrame{Name: "speed", Angle: "speed", ContributionOrder: []string{"latency"}, FigureEmphasis: []string{}, EvidenceAlignment: 1.0, ImpactPotential: 0.75}
	want := "[{'name': 'speed', 'angle': 'speed', 'central_thesis': '', 'title': '', 'mini_abstract': '', 'contribution_order': ['latency'], 'figure_emphasis': [], 'why_it_could_win': '', 'risk_of_failure': '', 'evidence_alignment': 1.0, 'impact_potential': 0.75}]"
	if got := PythonRepr([]StoryFrame{frame}); got != want {
		t.Fatalf("python repr mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestPythonReprChoosesPythonQuoteStyle(t *testing.T) {
	if got, want := PythonRepr("paper's claim"), `"paper's claim"`; got != want {
		t.Fatalf("python string repr mismatch: got %s want %s", got, want)
	}
}

func TestSectionPromptContract(t *testing.T) {
	spec := SectionSpec{Index: 1, Slug: "intro", Heading: "Introduction", Beats: []string{"state the problem"}, Establishes: "the gap matters", EvidenceIDs: []string{"E1"}, TargetWords: 400}
	next := &SectionSpec{Requires: "the gap matters"}
	got := SectionPrompt(Workspace{}, spec, nil, next)
	for _, required := range []string{
		"paper/sections/01_intro.tex",
		"\\section{Introduction}",
		"stay within 280–520 words",
		"% E<n>",
		"what the NEXT section requires: \"the gap matters\"",
	} {
		if !contains(got, required) {
			t.Errorf("section prompt missing %q", required)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
