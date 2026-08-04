package prompts

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Covers the authored prompts that python_parity_test.go does not exercise.
// The Python reasoners are invoked with fake AI/control-plane calls so the
// comparison captures the prompt bytes produced by the reference itself.
func TestPythonPositioningAndCritiquePromptParity(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(repo, ".venv", "bin", "python")
	if _, err := os.Stat(python); err != nil {
		t.Skip("Python reference venv is not installed")
	}
	script := `
import asyncio, contextlib, io, json, os, shutil, sys
sys.path.insert(0, 'src')
from preprint_af.reasoners import positioning, critique
from preprint_af.reasoners.models import (
    FrameJudgment, FrameSet, FidelityAudit, NarrativeReview, NoveltyScan,
    PersonaReview, PositioningDecision, StoryFrame,
)

root = '/tmp/preprint-af-prompt-parity'
shutil.rmtree(root, ignore_errors=True)
os.makedirs(root + '/paper/sections', exist_ok=True)
os.makedirs(root + '/paper/reviews', exist_ok=True)
os.makedirs(root + '/input', exist_ok=True)
open(root + '/EVIDENCE.md', 'w').write('EVIDENCE')
open(root + '/POSITIONING.md', 'w').write('POSITIONING')
open(root + '/BLUEPRINT.md', 'w').write('BLUEPRINT')
open(root + '/paper/main.tex', 'w').write('PAPER')
open(root + '/paper/refs.bib', 'w').write('@article{smith2024,\n}')
ws = dict(run_id='r', root=root, input_dir=root + '/input', paper_dir=root + '/paper',
    sections_dir=root + '/paper/sections', figures_dir=root + '/paper/figures',
    reviews_dir=root + '/paper/reviews', evidence_path=root + '/EVIDENCE.md',
    positioning_path=root + '/POSITIONING.md', blueprint_path=root + '/BLUEPRINT.md',
    todo_path=root + '/TODO.md', source_folder=root, input_files=[],
    has_existing_draft=False, has_data_files=False)
frame = StoryFrame(name='speed', angle='speed', central_thesis='2x faster',
    title='A Faster Method', mini_abstract='Measured abstract.',
    contribution_order=['latency'], figure_emphasis=['result'], why_it_could_win='measured',
    risk_of_failure='scope', evidence_alignment=0.8, impact_potential=0.7)
judgment = FrameJudgment(frame_name='speed', persona='reader', comprehension=0.8,
    excitement=0.7, credibility=0.9, naturalness=0.6, concerns=['scope'], confident=True)
novelty = NoveltyScan(closest_titles=['Prior'], collision_risks=['collision'],
    positioning_openings=['opening'], confident=True)
out = {}

class Capture:
    async def ai(self, **kw):
        out[self.key + '_system'] = kw['system']
        out[self.key + '_user'] = kw['user']
        schema = kw['schema']
        if schema is FrameSet:
            return FrameSet(frames=[frame], generation_rationale='r', confident=True)
        if schema is FrameJudgment:
            return judgment
        if schema is PositioningDecision:
            return PositioningDecision(winning_frame_name='speed', final_title='Title',
                final_abstract='Abstract', opening_thesis='Opening', contribution_order=['latency'],
                rejected_alternatives=[], selection_rationale='Rationale', score=0.8, confident=True)
        if schema is PersonaReview:
            return PersonaReview(persona='reader', issues=[], acceptance_risk=0.2,
                verdict='ok', confident=True)
        if schema is NarrativeReview:
            return NarrativeReview(transition_issues=[], promise_alignment_issues=[],
                arc_assessment='ok', score=0.8, confident=True)
        if schema is FidelityAudit:
            return FidelityAudit(unsupported_claims=[], number_mismatches=[], citation_issues=[],
                blocking=False, score=1.0, confident=True)
        raise AssertionError(schema)

capture = Capture()
positioning.router.ai = capture.ai
critique.router.ai = capture.ai
positioning.helpers.git_snapshot = lambda *args: None

async def fake_call(name, **kw):
    if name.endswith('positioning_generate_frames'):
        return FrameSet(frames=[frame], generation_rationale='r', confident=True).model_dump()
    if name.endswith('positioning_judge_frame'):
        return judgment.model_dump()
    if name.endswith('positioning_scan_novelty'):
        return novelty.model_dump()
    raise AssertionError(name)

positioning.router.call = fake_call

async def main():
    capture.key = 'frame'
    await positioning.generate_frames(ws, target_venue=None, field_hint=None)
    capture.key = 'judge'
    await positioning.judge_frame(frame.model_dump(), 'DIGEST', 'reader', target_venue=None)
    capture.key = 'meta'
    await positioning.run_positioning(ws, target_venue=None, field_hint=None, allow_web=True)
    out['_positioning_text'] = open(root + '/POSITIONING.md').read()
    capture.key = 'persona'
    await critique.persona_review(ws, 'reader', 2)
    capture.key = 'narrative'
    await critique.narrative_review(ws, 2)
    capture.key = 'fidelity'
    await critique.fidelity_audit(ws, 2)

with contextlib.redirect_stdout(io.StringIO()):
    asyncio.run(main())
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
		t.Fatalf("decode Python prompts: %v\n%s", err, raw)
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
			t.Errorf("missing Go prompt fixture %q", key)
		} else if actual != expected {
			t.Errorf("%s prompt drifted from Python reference\n--- Go ---\n%s\n--- Python ---\n%s", key, actual, expected)
		}
	}
}
