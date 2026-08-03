package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

const evidenceCap = 60000
const digestCap = 8000
const topFrames = 4
const maxPositioningConcurrency = 8

type GenerateFramesInput struct {
	Workspace   Workspace `json:"workspace"`
	TargetVenue *string   `json:"target_venue"`
	FieldHint   *string   `json:"field_hint"`
	Model       *string   `json:"model"`
}
type JudgeFrameInput struct {
	Frame          StoryFrame `json:"frame"`
	EvidenceDigest string     `json:"evidence_digest"`
	Persona        string     `json:"persona"`
	TargetVenue    *string    `json:"target_venue"`
	Model          *string    `json:"model"`
}
type ScanNoveltyInput struct {
	Workspace Workspace `json:"workspace"`
	FrameSet  FrameSet  `json:"frame_set"`
	Model     *string   `json:"model"`
}
type RunPositioningInput struct {
	Workspace   Workspace `json:"workspace"`
	TargetVenue *string   `json:"target_venue"`
	FieldHint   *string   `json:"field_hint"`
	AllowWeb    bool      `json:"allow_web"`
	Model       *string   `json:"model"`
}

func promptWorkspace(ws Workspace) prompts.Workspace {
	return prompts.Workspace{Root: ws.Root, InputDir: ws.InputDir, InputFiles: ws.InputFiles, EvidencePath: ws.EvidencePath, HasExistingDraft: ws.HasExistingDraft}
}
func promptFrame(f StoryFrame) prompts.StoryFrame {
	return prompts.StoryFrame{Name: f.Name, Angle: f.Angle, CentralThesis: f.CentralThesis, Title: f.Title, MiniAbstract: f.MiniAbstract, ContributionOrder: f.ContributionOrder, FigureEmphasis: f.FigureEmphasis, WhyItCouldWin: f.WhyItCouldWin, RiskOfFailure: f.RiskOfFailure, EvidenceAlignment: f.EvidenceAlignment, ImpactPotential: f.ImpactPotential}
}
func promptJudgment(j FrameJudgment) prompts.FrameJudgment {
	return prompts.FrameJudgment{FrameName: j.FrameName, Persona: j.Persona, Comprehension: j.Comprehension, Excitement: j.Excitement, Credibility: j.Credibility, Naturalness: j.Naturalness, Concerns: j.Concerns, Confident: j.Confident}
}

func validateFrameSetContract(fs FrameSet) error {
	if len(fs.Frames) < 5 || len(fs.Frames) > 6 {
		return fmt.Errorf("frame generation produced %d frames (need 5-6)", len(fs.Frames))
	}
	return nil
}

func (s *Service) GenerateFrames(ctx context.Context, in GenerateFramesInput) (any, error) {
	evidence := ReadText(in.Workspace.EvidencePath, evidenceCap)
	system, user := prompts.FrameGenerationPrompts(stringValue(in.TargetVenue), stringValue(in.FieldHint), evidence)
	out, err := aiInto[FrameSet](ctx, s, system, user, stringValue(in.Model))
	directContractErr := validateFrameSetContract(out)
	if err != nil || directContractErr != nil {
		fallback, hr, fallbackErr := harnessInto[FrameSet](ctx, s, system+"\n\n"+user, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
		if fallbackErr != nil || hr == nil || hr.IsError {
			return nil, fmt.Errorf("frame generation failed; direct error: %v; direct contract: %v; harness error: %v", err, directContractErr, fallbackErr)
		}
		out = fallback
	}
	if err := validateFrameSetContract(out); err != nil {
		return nil, fmt.Errorf("frame fallback violated contract: %w", err)
	}
	return out, nil
}

func (s *Service) JudgeFrame(ctx context.Context, in JudgeFrameInput) (any, error) {
	system, user := prompts.JudgeFramePrompts(promptFrame(in.Frame), in.EvidenceDigest, in.Persona, stringValue(in.TargetVenue))
	out, err := aiInto[FrameJudgment](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		return FrameJudgment{FrameName: in.Frame.Name, Persona: in.Persona, Concerns: []string{}}, nil
	}
	return out, nil
}

func (s *Service) ScanNovelty(ctx context.Context, in ScanNoveltyInput) (any, error) {
	frames := make([]prompts.StoryFrame, len(in.FrameSet.Frames))
	for i, f := range in.FrameSet.Frames {
		frames[i] = promptFrame(f)
	}
	prompt := prompts.NoveltyScanPrompt(in.Workspace.Root, frames)
	out, hr, err := harnessInto[NoveltyScan](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	scan := filepath.Join(in.Workspace.Root, "POSITIONING_SCAN.md")
	if err != nil || hr == nil || hr.IsError || !FileExists(scan) {
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
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxPositioningConcurrency)
	for fi, f := range frames {
		for pi, p := range prompts.PositioningPersonas {
			idx := fi*len(prompts.PositioningPersonas) + pi
			f, p := f, p
			g.Go(func() error {
				j, e := callInto[FrameJudgment](gctx, s, "positioning_judge_frame", JudgeFrameInput{Frame: f, EvidenceDigest: digest, Persona: p, TargetVenue: in.TargetVenue, Model: in.Model})
				if e != nil {
					j = FrameJudgment{FrameName: f.Name, Persona: p, Concerns: []string{}}
				}
				judgments[idx] = j
				return nil
			})
		}
	}
	if in.AllowWeb {
		g.Go(func() error {
			v, e := callInto[NoveltyScan](gctx, s, "positioning_scan_novelty", ScanNoveltyInput{Workspace: in.Workspace, FrameSet: FrameSet{Frames: frames, GenerationRationale: fs.GenerationRationale, Confident: fs.Confident}, Model: in.Model})
			mu.Lock()
			defer mu.Unlock()
			if e == nil {
				novelty = v
			} else {
				novelty = NewNoveltyScan()
			}
			return nil
		})
	} else {
		novelty = NewNoveltyScan()
	}
	_ = g.Wait()
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
		decision = fallbackDecision(frames, judgments)
	}
	writePositioning(in.Workspace, decision, frames, judgments, novelty)
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

func writePositioning(ws Workspace, d PositioningDecision, frames []StoryFrame, judgments []FrameJudgment, n NoveltyScan) {
	contrib := numbered(d.ContributionOrder, "1. (none specified)")
	rejected := bullets(d.RejectedAlternatives, "- (none)")
	var table strings.Builder
	table.WriteString("| Frame | Persona | Comprehension | Excitement | Credibility | Naturalness |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, f := range frames {
		for _, j := range judgments {
			if j.FrameName == f.Name {
				p := strings.Split(j.Persona, ",")[0]
				fmt.Fprintf(&table, "| %s | %s | %.2f | %.2f | %.2f | %.2f |\n", j.FrameName, strings.TrimSpace(p), j.Comprehension, j.Excitement, j.Credibility, j.Naturalness)
			}
		}
	}
	body := fmt.Sprintf("# Title: %s\n\n## Final abstract\n\n%s\n\n## Opening thesis\n\n%s\n\n## Contribution order\n\n%s\n\n## Winning frame: %s\n\n%s\n\n## Judgments\n\n%s\n## Rejected alternatives\n\n%s\n\n## Novelty scan\n\nConfident: %t\n\nClosest related titles:\n%s\n\nCollision risks:\n%s\n\nOpen positioning angles:\n%s\n", d.FinalTitle, d.FinalAbstract, d.OpeningThesis, contrib, d.WinningFrameName, d.SelectionRationale, table.String(), rejected, n.Confident, bullets(n.ClosestTitles, "- (none found)"), bullets(n.CollisionRisks, "- (none identified)"), bullets(n.PositioningOpenings, "- (none identified)"))
	_, _ = WriteText(ws.PositioningPath, body)
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
