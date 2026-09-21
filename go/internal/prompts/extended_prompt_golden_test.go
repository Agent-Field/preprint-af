package prompts

import (
	"encoding/json"
	"os"
	"testing"
)

// Captured byte-for-byte from the final Python reference before its removal.
// Changes require intentional golden regeneration and review.
func TestPositioningAndCritiquePromptGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/prompts-extended.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]string
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	positioningText := want["_positioning_text"]
	delete(want, "_positioning_text")

	frame := StoryFrame{Name: "speed", Angle: "speed", CentralThesis: "2x faster", Title: "A Faster Method", MiniAbstract: "Measured abstract.", ContributionOrder: []string{"latency"}, FigureEmphasis: []string{"result"}, WhyItCouldWin: "measured", RiskOfFailure: "scope", EvidenceAlignment: .8, ImpactPotential: .7}
	judgment := FrameJudgment{FrameName: "speed", Persona: "reader", Comprehension: .8, Excitement: .7, Credibility: .9, Naturalness: .6, Concerns: []string{"scope"}, Confident: true}
	novelty := NoveltyScan{ClosestTitles: []string{"Prior"}, CollisionRisks: []string{"collision"}, PositioningOpenings: []string{"opening"}, Confident: true}
	got := map[string]string{}
	got["frame_system"], got["frame_user"] = FrameGenerationPrompts("", "", "EVIDENCE")
	got["judge_system"], got["judge_user"] = JudgeFramePrompts(frame, "DIGEST", "reader", "")
	judgments := []FrameJudgment{judgment, judgment, judgment, judgment}
	got["meta_system"], got["meta_user"] = MetaSelectionPrompts("", "", []StoryFrame{frame}, judgments, novelty)
	const paper = "--- main.tex ---\nPAPER"
	got["persona_system"], got["persona_user"] = PersonaReviewPrompt("reader", 2, paper)
	got["narrative_system"], got["narrative_user"] = NarrativeReviewPrompt(2, positioningText, "BLUEPRINT", paper)
	got["fidelity_system"], got["fidelity_user"] = FidelityAuditPrompt(2, "EVIDENCE", []string{"smith2024"}, paper)

	for key, expected := range want {
		if actual, ok := got[key]; !ok {
			t.Errorf("missing prompt fixture %q", key)
		} else if actual != expected {
			t.Errorf("%s prompt drifted from the reference golden\n--- Go ---\n%s\n--- Golden ---\n%s", key, actual, expected)
		}
	}
}
