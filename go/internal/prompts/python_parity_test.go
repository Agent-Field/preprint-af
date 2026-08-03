package prompts

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// These hashes were frozen from the original Python implementation. They keep
// exact prompt parity testable after the legacy Python agent is removed. The
// plotting executable is normalized because its absolute path is host-specific.
func TestOriginalPythonPromptParityGoldens(t *testing.T) {
	const figurePython = "<FIGURE_PYTHON>"
	ws := Workspace{Root: "/tmp/run", InputDir: "/tmp/run/input", EvidencePath: "/tmp/run/EVIDENCE.md", InputFiles: []string{"results.csv", "plot.py"}, HasExistingDraft: true}
	sec := SectionSpec{Index: 1, Slug: "intro", Heading: "Introduction", Beats: []string{"state the problem"}, Establishes: "the gap matters", EvidenceIDs: []string{"E1"}, FigureSlugs: []string{"result"}, TargetWords: 400}
	next := &SectionSpec{Index: 2, Slug: "method", Heading: "Method", Beats: []string{}, Establishes: "the method works", Requires: "the gap matters", EvidenceIDs: []string{}, FigureSlugs: []string{}, TargetWords: 500}
	fig := FigureSpec{Index: 1, Slug: "result", Purpose: "Accuracy improves", DataSources: []string{"results.csv"}, Buildable: true, CaptionTakeaway: "The method wins"}
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
	want := map[string]string{
		"bib_off":          "5e6f5b0925be471f187bd67a611f1fcadb181bfe21eae4f8f2853959242c12a8",
		"bib_web":          "8a4c32565220dcf7553fcfff10b414ccb5cbaee844fffa859b776d61ee9b8d15",
		"blueprint_system": "b590aa7e5b93cc15e183799cc40f42b35ce9aecd1fbff8d10749f35f4598a73b",
		"blueprint_user":   "a05de6a64662b4575d5a6c0e9e969d00e2e61d30983fff77c489443700bef13b",
		"evidence":         "ffd2e74b48cba89a2f46f1d80f13ff31608be93d5517b3ad2e20b3c9ff55f021",
		"figure":           "5a21cf29323e6082fb56153e0a7176cd5c5b16112db6ad609017edd9df609978",
		"latex":            "51e092137199123ed9c9ecd8fcef84a4b9ddf315c04719466baf474ce7a866b0",
		"section":          "08e947eb109b928947287b09c18d298fe2be1113cfccbcc720ed41ade73dcbea",
	}
	for name, prompt := range got {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(prompt)))
		if sum != want[name] {
			t.Errorf("%s prompt drifted from original Python golden: got %s want %s", name, sum, want[name])
		}
	}
}
