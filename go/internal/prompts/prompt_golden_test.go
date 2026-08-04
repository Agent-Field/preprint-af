package prompts

import (
	"encoding/json"
	"os"
	"testing"
)

// Captured byte-for-byte from the final Python reference before its removal.
// Changes require intentional golden regeneration and review.
func TestCorePromptGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/prompts-core.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}

	ws := Workspace{Root: "/tmp/run", InputDir: "/tmp/run/input", EvidencePath: "/tmp/run/EVIDENCE.md", InputFiles: []string{"results.csv", "plot.py"}, HasExistingDraft: true}
	sec := SectionSpec{Index: 1, Slug: "intro", Heading: "Introduction", Beats: []string{"state the problem"}, Establishes: "the gap matters", EvidenceIDs: []string{"E1"}, FigureSlugs: []string{"result"}, TargetWords: 400}
	next := &SectionSpec{Index: 2, Slug: "method", Heading: "Method", Beats: []string{}, Establishes: "the method works", Requires: "the gap matters", EvidenceIDs: []string{}, FigureSlugs: []string{}, TargetWords: 500}
	fig := FigureSpec{Index: 1, Slug: "result", Purpose: "Accuracy improves", DataSources: []string{"results.csv"}, Buildable: true, CaptionTakeaway: "The method wins"}
	const figurePython = "/opt/preprint-af/python3"
	got := map[string]string{
		"evidence":         EvidencePrompt(ws, figurePython),
		"blueprint_system": BlueprintSystemPrompt(),
		"blueprint_user":   BlueprintUserPrompt("E1: exact result", "# Title: Small Model", []string{"results.csv", "plot.py"}, "NeurIPS"),
		"section":          SectionPrompt(ws, sec, nil, next),
		"figure":           FigurePrompt(ws, fig, figurePython),
		"bib_web":          BibliographyPrompt(ws, []string{"Smith 2024"}, true),
		"bib_off":          BibliographyPrompt(ws, []string{"Smith 2024"}, false),
		"latex":            LatexRepairPrompt("! Undefined control sequence"),
	}
	for key, expected := range want {
		if actual, ok := got[key]; !ok {
			t.Errorf("missing prompt fixture %q", key)
		} else if actual != expected {
			t.Errorf("%s prompt drifted from the reference golden\n--- Go ---\n%s\n--- Golden ---\n%s", key, actual, expected)
		}
	}
}
