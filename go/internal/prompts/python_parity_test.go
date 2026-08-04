package prompts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This test compares the port directly with the Python reference when the
// repository's development venv is present. CI without that venv still runs
// the self-contained golden tests in this package.
func TestPythonPromptParity(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(repo, ".venv", "bin", "python")
	if _, err := os.Stat(python); err != nil {
		t.Skip("Python reference venv is not installed")
	}
	script := `
import json, sys
sys.path.insert(0, 'src')
from preprint_af.reasoners import intake, blueprint, build, latex
from preprint_af.reasoners.models import SectionSpec, FigureSpec
ws = {'root':'/tmp/run','input_dir':'/tmp/run/input','evidence_path':'/tmp/run/EVIDENCE.md','input_files':['results.csv','plot.py'],'has_existing_draft':True}
sec = SectionSpec(index=1,slug='intro',heading='Introduction',beats=['state the problem'],establishes='the gap matters',requires='',evidence_ids=['E1'],figure_slugs=['result'],target_words=400)
nxt = SectionSpec(index=2,slug='method',heading='Method',beats=[],establishes='the method works',requires='the gap matters',target_words=500)
fig = FigureSpec(index=1,slug='result',purpose='Accuracy improves',data_sources=['results.csv'],buildable=True,caption_takeaway='The method wins')
out = {
 'evidence': intake._evidence_prompt(ws),
 'blueprint_system': blueprint._blueprint_system(),
 'blueprint_user': blueprint._blueprint_user('E1: exact result','# Title: Small Model',['results.csv','plot.py'],'NeurIPS'),
 'section': build._section_prompt(ws,sec,None,nxt),
 'figure': build._figure_prompt(ws,fig),
 'bib_web': build._bibliography_prompt(ws,['Smith 2024'],True),
 'bib_off': build._bibliography_prompt(ws,['Smith 2024'],False),
 'latex': latex._repair_prompt('! Undefined control sequence'),
}
print(json.dumps(out, ensure_ascii=False))
`
	cmd := exec.Command(python, "-c", script)
	cmd.Dir = repo
	raw, err := cmd.Output()
	if err != nil {
		t.Fatalf("run Python reference: %v", err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	ws := Workspace{Root: "/tmp/run", InputDir: "/tmp/run/input", EvidencePath: "/tmp/run/EVIDENCE.md", InputFiles: []string{"results.csv", "plot.py"}, HasExistingDraft: true}
	sec := SectionSpec{Index: 1, Slug: "intro", Heading: "Introduction", Beats: []string{"state the problem"}, Establishes: "the gap matters", EvidenceIDs: []string{"E1"}, FigureSlugs: []string{"result"}, TargetWords: 400}
	next := &SectionSpec{Index: 2, Slug: "method", Heading: "Method", Beats: []string{}, Establishes: "the method works", Requires: "the gap matters", EvidenceIDs: []string{}, FigureSlugs: []string{}, TargetWords: 500}
	fig := FigureSpec{Index: 1, Slug: "result", Purpose: "Accuracy improves", DataSources: []string{"results.csv"}, Buildable: true, CaptionTakeaway: "The method wins"}
	got := map[string]string{
		"evidence":         EvidencePrompt(ws, os.Args[0]),
		"blueprint_system": BlueprintSystemPrompt(),
		"blueprint_user":   BlueprintUserPrompt("E1: exact result", "# Title: Small Model", []string{"results.csv", "plot.py"}, "NeurIPS"),
		"section":          SectionPrompt(ws, sec, nil, next),
		"figure":           FigurePrompt(ws, fig, os.Args[0]),
		"bib_web":          BibliographyPrompt(ws, []string{"Smith 2024"}, true),
		"bib_off":          BibliographyPrompt(ws, []string{"Smith 2024"}, false),
		"latex":            LatexRepairPrompt("! Undefined control sequence"),
	}
	// The executable path is intentionally supplied to both implementations.
	// Replace Python's interpreter occurrence with the same stable fixture path.
	pyPath := filepath.Join(repo, ".venv", "bin", "python")
	got["evidence"] = EvidencePrompt(ws, pyPath)
	got["figure"] = FigurePrompt(ws, fig, pyPath)
	for key, expected := range want {
		if got[key] != expected {
			t.Fatalf("%s prompt drifted from Python reference\n--- Go ---\n%s\n--- Python ---\n%s", key, got[key], expected)
		}
	}
}
