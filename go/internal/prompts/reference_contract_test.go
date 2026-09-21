package prompts

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// This fixture was captured from the final reference implementation before
// the repository became Go-only. It protects prompts and production personas
// that were not exercised by the original parity suite.
func TestReferenceConstantsGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/reference-constants.json")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		NoveltyPrompt       string   `json:"novelty_prompt"`
		PositioningPersonas []string `json:"positioning_personas"`
		CritiquePersonas    []string `json:"critique_personas"`
		ReviewDirective     string   `json:"review_directive"`
		Constants           struct {
			CritiqueContextCap int `json:"critique_context_cap"`
			CritiquePaperCap   int `json:"critique_paper_cap"`
		} `json:"constants"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}

	frame := StoryFrame{Title: "A Faster Method", CentralThesis: "2x faster"}
	if got := NoveltyScanPrompt("/tmp/preprint-af-novelty-golden", []StoryFrame{frame}); got != want.NoveltyPrompt {
		t.Fatalf("novelty prompt drifted from reference golden\n--- Go ---\n%s\n--- Golden ---\n%s", got, want.NoveltyPrompt)
	}
	if !reflect.DeepEqual(PositioningPersonas, want.PositioningPersonas) {
		t.Fatalf("positioning personas drifted")
	}
	gotCritique := make([]string, len(Personas))
	for i, persona := range Personas {
		gotCritique[i] = persona.Brief
	}
	if !reflect.DeepEqual(gotCritique, want.CritiquePersonas) {
		t.Fatalf("critique personas drifted")
	}
	if ReviewDirective != want.ReviewDirective {
		t.Fatalf("review directive drifted")
	}
	if ContextCap != want.Constants.CritiqueContextCap || PaperCap != want.Constants.CritiquePaperCap {
		t.Fatalf("critique caps drifted: context=%d paper=%d", ContextCap, PaperCap)
	}
}
