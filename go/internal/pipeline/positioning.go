package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

const evidenceCap = 60000
const digestCap = 8000
const topFrames = 4

type GenerateFramesInput struct {
	Workspace   Workspace `json:"workspace"`
	TargetVenue *string   `json:"target_venue"`
	FieldHint   *string   `json:"field_hint"`
	Model       *string   `json:"model"`
}
type JudgeFrameInput struct {
	Frame          map[string]any `json:"frame"`
	EvidenceDigest string         `json:"evidence_digest"`
	Persona        string         `json:"persona"`
	TargetVenue    *string        `json:"target_venue"`
	Model          *string        `json:"model"`
}
type ScanNoveltyInput struct {
	Workspace Workspace      `json:"workspace"`
	FrameSet  map[string]any `json:"frame_set"`
	Model     *string        `json:"model"`
}
type RunPositioningInput struct {
	Workspace   Workspace `json:"workspace"`
	TargetVenue *string   `json:"target_venue"`
	FieldHint   *string   `json:"field_hint"`
	AllowWeb    bool      `json:"allow_web"`
	Model       *string   `json:"model"`
}

func (in *RunPositioningInput) UnmarshalJSON(data []byte) error {
	type plain RunPositioningInput
	seeded := plain{AllowWeb: true}
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*in = RunPositioningInput(seeded)
	return nil
}

func promptWorkspace(ws Workspace) prompts.Workspace {
	return prompts.Workspace{Root: ws.Root, InputDir: ws.InputDir, InputFiles: ws.InputFiles, EvidencePath: ws.EvidencePath, HasExistingDraft: ws.HasExistingDraft}
}
func promptFrame(f StoryFrame) prompts.StoryFrame {
	return prompts.StoryFrame{Name: f.Name, Angle: f.Angle, CentralThesis: f.CentralThesis, Title: f.Title, MiniAbstract: f.MiniAbstract, ContributionOrder: f.ContributionOrder, FigureEmphasis: f.FigureEmphasis, WhyItCouldWin: f.WhyItCouldWin, RiskOfFailure: f.RiskOfFailure, EvidenceAlignment: f.EvidenceAlignment, ImpactPotential: f.ImpactPotential}
}

func frameMap(f StoryFrame) map[string]any {
	return map[string]any{
		"name": f.Name, "angle": f.Angle, "central_thesis": f.CentralThesis,
		"title": f.Title, "mini_abstract": f.MiniAbstract,
		"contribution_order": f.ContributionOrder, "figure_emphasis": f.FigureEmphasis,
		"why_it_could_win": f.WhyItCouldWin, "risk_of_failure": f.RiskOfFailure,
		"evidence_alignment": f.EvidenceAlignment, "impact_potential": f.ImpactPotential,
	}
}

// pythonDictString mirrors str(frame.get(key, fallback)) for the scalar values
// consumed by the original Python reasoner.
func pythonDictString(values map[string]any, key, fallback string) string {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	if value == nil {
		return "None"
	}
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "True"
		}
		return "False"
	default:
		return fmt.Sprint(typed)
	}
}
func promptJudgment(j FrameJudgment) prompts.FrameJudgment {
	return prompts.FrameJudgment{FrameName: j.FrameName, Persona: j.Persona, Comprehension: j.Comprehension, Excitement: j.Excitement, Credibility: j.Credibility, Naturalness: j.Naturalness, Concerns: j.Concerns, Confident: j.Confident}
}

func (s *Service) GenerateFrames(ctx context.Context, in GenerateFramesInput) (any, error) {
	evidence := ReadText(in.Workspace.EvidencePath, evidenceCap)
	system, user := prompts.FrameGenerationPrompts(stringValue(in.TargetVenue), stringValue(in.FieldHint), evidence)
	out, err := aiInto[FrameSet](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return FrameSet{Frames: []StoryFrame{}, GenerationRationale: "frame generation failed: " + err.Error()}, nil
	}
	return out, nil
}

func (s *Service) JudgeFrame(ctx context.Context, in JudgeFrameInput) (any, error) {
	frame := prompts.StoryFrame{
		Name:          pythonDictString(in.Frame, "name", "unnamed frame"),
		Angle:         pythonDictString(in.Frame, "angle", ""),
		CentralThesis: pythonDictString(in.Frame, "central_thesis", ""),
		Title:         pythonDictString(in.Frame, "title", ""),
		MiniAbstract:  pythonDictString(in.Frame, "mini_abstract", ""),
	}
	system, user := prompts.JudgeFramePrompts(frame, in.EvidenceDigest, in.Persona, stringValue(in.TargetVenue))
	out, err := aiInto[FrameJudgment](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return FrameJudgment{FrameName: frame.Name, Persona: in.Persona, Concerns: []string{}}, nil
	}
	return out, nil
}

func (s *Service) ScanNovelty(ctx context.Context, in ScanNoveltyInput) (any, error) {
	rawFrames, _ := in.FrameSet["frames"].([]any)
	frames := make([]prompts.StoryFrame, 0, len(rawFrames))
	for _, raw := range rawFrames {
		f, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		frames = append(frames, prompts.StoryFrame{
			Title:         strings.TrimSpace(pythonDictString(f, "title", "")),
			CentralThesis: strings.TrimSpace(pythonDictString(f, "central_thesis", "")),
		})
	}
	prompt := prompts.NoveltyScanPrompt(in.Workspace.Root, frames)
	out, hr, err := harnessInto[NoveltyScan](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Python wraps this harness call in try/except and degrades to a
		// non-confident empty scan; a scout failure must not abort positioning.
		return NewNoveltyScan(), nil
	}
	scan := filepath.Join(in.Workspace.Root, "POSITIONING_SCAN.md")
	if hr == nil || hr.IsError || hr.Parsed == nil || !FileExists(scan) {
		return NewNoveltyScan(), nil
	}
	return out, nil
}

func frameScore(f StoryFrame) float64 { return .55*f.EvidenceAlignment + .45*f.ImpactPotential }

func (s *Service) RunPositioning(ctx context.Context, in RunPositioningInput) (any, error) {
	fs, err := callInto[FrameSet](ctx, s, "positioning_generate_frames", GenerateFramesInput{Workspace: in.Workspace, TargetVenue: in.TargetVenue, FieldHint: in.FieldHint, Model: in.Model})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(fs.Frames, func(i, j int) bool { return frameScore(fs.Frames[i]) > frameScore(fs.Frames[j]) })
	frames := fs.Frames
	if len(frames) > topFrames {
		frames = frames[:topFrames]
	}
	digest := ReadText(in.Workspace.EvidencePath, digestCap)
	judgments := make([]FrameJudgment, len(frames)*len(prompts.PositioningPersonas))
	var novelty NoveltyScan
	var g errgroup.Group
	for fi, f := range frames {
		for pi, p := range prompts.PositioningPersonas {
			idx := fi*len(prompts.PositioningPersonas) + pi
			f, p := f, p
			g.Go(func() error {
				j, e := callInto[FrameJudgment](ctx, s, "positioning_judge_frame", JudgeFrameInput{Frame: frameMap(f), EvidenceDigest: digest, Persona: p, TargetVenue: in.TargetVenue, Model: in.Model})
				if e != nil {
					return e
				}
				judgments[idx] = j
				return nil
			})
		}
	}
	if in.AllowWeb {
		g.Go(func() error {
			rawFrames := make([]any, len(frames))
			for i, f := range frames {
				rawFrames[i] = frameMap(f)
			}
			frameSet := map[string]any{"frames": rawFrames, "generation_rationale": fs.GenerationRationale, "confident": fs.Confident}
			v, e := callInto[NoveltyScan](ctx, s, "positioning_scan_novelty", ScanNoveltyInput{Workspace: in.Workspace, FrameSet: frameSet, Model: in.Model})
			if e != nil {
				return e
			}
			novelty = v
			return nil
		})
	} else {
		novelty = NewNoveltyScan()
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	pf := make([]prompts.StoryFrame, len(frames))
	for i, f := range frames {
		pf[i] = promptFrame(f)
	}
	pj := make([]prompts.FrameJudgment, len(judgments))
	for i, j := range judgments {
		pj[i] = promptJudgment(j)
	}
	pn := prompts.NoveltyScan{ClosestTitles: novelty.ClosestTitles, CollisionRisks: novelty.CollisionRisks, PositioningOpenings: novelty.PositioningOpenings, Confident: novelty.Confident}
	system, user := prompts.MetaSelectionPrompts(stringValue(in.TargetVenue), stringValue(in.FieldHint), pf, pj, pn)
	decision, e := aiInto[PositioningDecision](ctx, s, system, user, stringValue(in.Model))
	if e != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		decision = fallbackDecision(frames, judgments)
	}
	if err := writePositioning(in.Workspace, decision, frames, judgments, novelty); err != nil {
		return nil, err
	}
	GitSnapshot(in.Workspace.Root, "P1 positioning decided")
	return decision, nil
}

func fallbackDecision(frames []StoryFrame, judgments []FrameJudgment) PositioningDecision {
	if len(frames) == 0 {
		return PositioningDecision{FinalTitle: "Untitled draft", ContributionOrder: []string{}, RejectedAlternatives: []string{}, SelectionRationale: "Meta-selection failed and no candidate frames were available."}
	}
	best := frames[0]
	bestMean := -1.0
	for _, f := range frames {
		sum := 0.0
		n := 0
		for _, j := range judgments {
			if j.FrameName == f.Name {
				sum += (j.Comprehension + j.Excitement + j.Credibility + j.Naturalness) / 4
				n++
			}
		}
		mean := 0.0
		if n > 0 {
			mean = sum / float64(n)
		}
		if mean > bestMean {
			bestMean = mean
			best = f
		}
	}
	rej := []string{}
	for _, f := range frames {
		if f.Name != best.Name {
			rej = append(rej, f.Name)
		}
	}
	return PositioningDecision{WinningFrameName: best.Name, FinalTitle: best.Title, FinalAbstract: best.MiniAbstract, OpeningThesis: best.CentralThesis, ContributionOrder: best.ContributionOrder, RejectedAlternatives: rej, SelectionRationale: fmt.Sprintf("Meta-selection call failed; fell back to the frame with the highest mean reader judgment (%.3f).", bestMean), Score: clamp(bestMean)}
}

func writePositioning(ws Workspace, d PositioningDecision, frames []StoryFrame, judgments []FrameJudgment, n NoveltyScan) error {
	contrib := numbered(d.ContributionOrder, "1. (none specified)")
	rejected := bullets(d.RejectedAlternatives, "- (none)")
	var table strings.Builder
	table.WriteString("| Frame | Persona | Comprehension | Excitement | Credibility | Naturalness |\n| --- | --- | --- | --- | --- | --- |\n")
	order := make(map[string]int, len(frames))
	for i, frame := range frames {
		order[frame.Name] = i
	}
	rows := append([]FrameJudgment(nil), judgments...)
	sort.SliceStable(rows, func(i, j int) bool {
		left, ok := order[rows[i].FrameName]
		if !ok {
			left = len(order)
		}
		right, ok := order[rows[j].FrameName]
		if !ok {
			right = len(order)
		}
		if left != right {
			return left < right
		}
		return rows[i].Persona < rows[j].Persona
	})
	for _, judgment := range rows {
		persona := strings.Split(judgment.Persona, ",")[0]
		fmt.Fprintf(&table, "| %s | %s | %.2f | %.2f | %.2f | %.2f |\n", judgment.FrameName, strings.TrimSpace(persona), judgment.Comprehension, judgment.Excitement, judgment.Credibility, judgment.Naturalness)
	}
	if len(rows) == 0 {
		table.WriteString("| (none) | (none) | 0.00 | 0.00 | 0.00 | 0.00 |\n")
	}
	body := fmt.Sprintf("# Title: %s\n\n## Final abstract\n\n%s\n\n## Opening thesis\n\n%s\n\n## Contribution order\n\n%s\n\n## Winning frame: %s\n\n%s\n\n## Judgments\n\n%s\n## Rejected alternatives\n\n%s\n\n## Novelty scan\n\nConfident: %s\n\nClosest related titles:\n%s\n\nCollision risks:\n%s\n\nOpen positioning angles:\n%s\n", d.FinalTitle, d.FinalAbstract, d.OpeningThesis, contrib, d.WinningFrameName, d.SelectionRationale, table.String(), rejected, pythonBool(n.Confident), bullets(n.ClosestTitles, "- (none found)"), bullets(n.CollisionRisks, "- (none identified)"), bullets(n.PositioningOpenings, "- (none identified)"))
	_, err := WriteText(ws.PositioningPath, body)
	return err
}
func clamp(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
func numbered(v []string, empty string) string {
	if len(v) == 0 {
		return empty
	}
	a := make([]string, len(v))
	for i, x := range v {
		a[i] = fmt.Sprintf("%d. %s", i+1, x)
	}
	return strings.Join(a, "\n")
}
