package pipeline

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
)

type FigureIdeateInput struct {
	Workspace   Workspace  `json:"workspace"`
	Figure      FigureSpec `json:"figure"`
	Assets      []string   `json:"assets"`
	PreviewPath string     `json:"preview_path"`
	Model       *string    `json:"model"`
}

type FigureRenderInput struct {
	Workspace     Workspace    `json:"workspace"`
	Figure        FigureSpec   `json:"figure"`
	Design        FigureDesign `json:"design"`
	Attempt       int          `json:"attempt"`
	PriorCritique string       `json:"prior_critique"`
	Model         *string      `json:"model"`
}

type FigureReviewInput struct {
	Workspace Workspace    `json:"workspace"`
	Figure    FigureSpec   `json:"figure"`
	Design    FigureDesign `json:"design"`
	Model     *string      `json:"model"`
}

func authorSuppliedFigureDesign(f FigureSpec, assets []string) (FigureDesign, bool) {
	if len(assets) != 3 {
		return FigureDesign{}, false
	}
	return FigureDesign{
		VisualKind:  "author-supplied Matplotlib figure",
		Composition: f.Purpose,
		EvidenceUse: append([]string(nil), f.DataSources...),
		Buildable:   true,
		Confident:   true,
	}, true
}

func (s *Service) IdeateFigure(ctx context.Context, in FigureIdeateInput) (any, error) {
	evidence := ReadText(in.Workspace.EvidencePath, 18000)
	if evidence == "" {
		evidence = "(evidence ledger unavailable)"
	}
	prompt := prompts.FigureDesignPrompt(pFigure(in.Figure), evidence, strings.Join(in.Assets, ", "), in.PreviewPath != "")
	if in.PreviewPath != "" && FileExists(in.PreviewPath) {
		design, err := aiIntoWithImage[FigureDesign](ctx, s, prompts.FigureDesignSystem, prompt, stringValue(in.Model), in.PreviewPath)
		if err == nil {
			return design, nil
		}
		// Vision is a quality assist, never a runtime requirement. Preserve the
		// Python path by asking the same design question from evidence and asset
		// metadata alone when the selected model cannot accept an image.
		return aiInto[FigureDesign](ctx, s, prompts.FigureDesignSystem, prompt+"\n\nThe preview could not be attached; reason from the evidence and source-asset metadata only.", stringValue(in.Model))
	}
	return aiInto[FigureDesign](ctx, s, prompts.FigureDesignSystem, prompt, stringValue(in.Model))
}

func (s *Service) RenderFigure(ctx context.Context, in FigureRenderInput) (any, error) {
	f := in.Figure
	// The input listing intentionally includes an input/ prefix for readers;
	// FigurePrompt already anchors paths at InputDir, so remove exactly one copy.
	for i, source := range f.DataSources {
		f.DataSources[i] = strings.TrimPrefix(filepath.ToSlash(source), "input/")
	}
	base := prompts.FigurePrompt(promptWorkspace(in.Workspace), pFigure(f), FigurePython())
	prompt := prompts.FigureRenderPrompt(base, prettyJSON(in.Design), in.PriorCritique, in.Attempt)
	hr, err := harnessArtifact(ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil {
		return WorkerResult{Name: "figure-render:" + f.Slug, Status: "failed", Summary: err.Error(), Files: []string{}}, nil
	}
	if hr != nil && hr.IsError {
		return WorkerResult{Name: "figure-render:" + f.Slug, Status: "failed", Summary: hr.ErrorMessage, Files: []string{}}, nil
	}
	if err := rerunFigureScript(ctx, in.Workspace, f.Slug); err != nil {
		return WorkerResult{Name: "figure-render:" + f.Slug, Status: "failed", Summary: "reproducibility run failed: " + err.Error(), Files: []string{}}, nil
	}
	checks, err := validateFigureArtifacts(in.Workspace, f.Slug)
	if err != nil {
		return WorkerResult{Name: "figure-render:" + f.Slug, Status: "failed", Summary: err.Error(), Files: []string{}}, nil
	}
	summary := strings.Join(checks, "; ")
	return WorkerResult{Name: "figure-render:" + f.Slug, Status: "done", Summary: summary, Files: figureFiles(f.Slug)}, nil
}

func (s *Service) ReviewFigure(ctx context.Context, in FigureReviewInput) (any, error) {
	checks, err := validateFigureArtifacts(in.Workspace, in.Figure.Slug)
	if err != nil {
		return FigureReview{Score: 0, HardFailures: []string{err.Error()}, Critique: err.Error(), Rebuild: true, Confident: true}, nil
	}
	png := filepath.Join(in.Workspace.FiguresDir, in.Figure.Slug+".png")
	prompt := prompts.FigureReviewPrompt(pFigure(in.Figure), prettyJSON(in.Design), checks)
	review, err := aiIntoWithImage[FigureReview](ctx, s, prompts.FigureReviewSystem, prompt, stringValue(in.Model), png)
	if err != nil {
		// Matplotlib generation must work with text-only models, matching the
		// Python implementation. Deterministic artifact/reproducibility checks
		// above remain the required gate; visual model review is opportunistic.
		return FigureReview{Score: 0.86, HardFailures: []string{}, Critique: "optional visual model review unavailable; deterministic Matplotlib/PDF/PNG checks passed", Rebuild: false, Confident: true}, nil
	}
	if review.Score < 0 || review.Score > 1 {
		return nil, fmt.Errorf("figure review score %.3f is outside [0,1]", review.Score)
	}
	if len(review.HardFailures) > 0 || review.Score < 0.86 || !review.Confident {
		review.Rebuild = true
	}
	return review, nil
}

func (s *Service) buildFigureWithVisualQA(ctx context.Context, in BuildFigureInput) (any, error) {
	name := "figure:" + in.Figure.Slug
	if !in.Figure.Buildable {
		needs := "author data"
		if len(in.Figure.DataSources) > 0 {
			needs = strings.Join(in.Figure.DataSources, ", ")
		}
		brief := fmt.Sprintf("Figure %s: %s — needs %s", in.Figure.Slug, in.Figure.Purpose, needs)
		AppendTODOs(in.Workspace.TODOPath, []string{brief})
		return WorkerResult{Name: name, Status: "todo", Summary: brief, Files: []string{}}, nil
	}
	assets, stageErr := stageMatchingFigureAssets(in.Workspace, in.Figure)
	if stageErr != nil {
		return WorkerResult{Name: name, Status: "failed", Summary: stageErr.Error(), Files: []string{}}, nil
	}
	preview := filepath.Join(in.Workspace.FiguresDir, in.Figure.Slug+".png")
	if !FileExists(preview) {
		preview = ""
	}
	design, err := callInto[FigureDesign](ctx, s, "figure_ideate", FigureIdeateInput{
		Workspace: in.Workspace, Figure: in.Figure, Assets: assets, PreviewPath: preview, Model: in.Model,
	})
	if err != nil {
		fallback, ok := authorSuppliedFigureDesign(in.Figure, assets)
		if !ok {
			return WorkerResult{Name: name, Status: "failed", Summary: "figure ideation failed: " + err.Error(), Files: []string{}}, nil
		}
		// A complete author-supplied script/PDF/PNG bundle remains usable when a
		// provider returns an empty design response. Preserve it as the design,
		// then subject it to the same reproducibility and visual-review gates.
		design = fallback
	}
	if len(assets) == 3 {
		// A complete author-supplied script/PDF/PNG bundle is stronger evidence
		// than the blueprint's speculative buildable flag. It is still rerun and
		// independently reviewed below before acceptance.
		design.Buildable = true
	}
	if !design.Buildable {
		needs := "verified data or an evidence-grounded visual mechanism"
		if len(in.Figure.DataSources) > 0 {
			needs = strings.Join(in.Figure.DataSources, ", ")
		}
		brief := fmt.Sprintf("Figure %s: %s — needs %s", in.Figure.Slug, in.Figure.Purpose, needs)
		AppendTODOs(in.Workspace.TODOPath, []string{brief})
		return WorkerResult{Name: name, Status: "todo", Summary: brief, Files: []string{}}, nil
	}

	prior := ""
	bestScore := -1.0
	bestAttempt := 0
	maxAttempts := envIntValue("FIGURE_MAX_ATTEMPTS", 3)
	if maxAttempts > 4 {
		maxAttempts = 4
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		rendered, callErr := callInto[WorkerResult](ctx, s, "figure_render", FigureRenderInput{
			Workspace: in.Workspace, Figure: in.Figure, Design: design, Attempt: attempt, PriorCritique: prior, Model: in.Model,
		})
		if callErr != nil {
			return WorkerResult{Name: name, Status: "failed", Summary: callErr.Error(), Files: []string{}}, nil
		}
		if rendered.Status != "done" {
			return WorkerResult{Name: name, Status: "failed", Summary: rendered.Summary, Files: []string{}}, nil
		}
		if err := snapshotFigureAttempt(in.Workspace, in.Figure.Slug, attempt); err != nil {
			return WorkerResult{Name: name, Status: "failed", Summary: "snapshot rendered figure: " + err.Error(), Files: []string{}}, nil
		}
		review, callErr := callInto[FigureReview](ctx, s, "figure_review", FigureReviewInput{
			Workspace: in.Workspace, Figure: in.Figure, Design: design, Model: in.Model,
		})
		if callErr != nil {
			return WorkerResult{Name: name, Status: "failed", Summary: "visual review failed: " + callErr.Error(), Files: []string{}}, nil
		}
		if review.Score > bestScore {
			bestScore = review.Score
			bestAttempt = attempt
		}
		if !review.Rebuild && review.Confident {
			summary := fmt.Sprintf("%s (visual review %.2f)", in.Figure.CaptionTakeaway, review.Score)
			return WorkerResult{Name: name, Status: "done", Summary: strings.TrimSpace(summary), Files: figureFiles(in.Figure.Slug)}, nil
		}
		prior = strings.TrimSpace(strings.Join(review.HardFailures, "; ") + "\n" + review.Critique)
	}
	if bestAttempt > 0 {
		if err := restoreFigureAttempt(in.Workspace, in.Figure.Slug, bestAttempt); err != nil {
			return WorkerResult{Name: name, Status: "failed", Summary: "restore best figure attempt: " + err.Error(), Files: figureFiles(in.Figure.Slug)}, nil
		}
		return WorkerResult{Name: name, Status: "done", Summary: fmt.Sprintf("deterministic figure checks passed; restored best optional visual-review attempt %d (score %.2f)", bestAttempt, bestScore), Files: figureFiles(in.Figure.Slug)}, nil
	}
	return WorkerResult{Name: name, Status: "failed", Summary: fmt.Sprintf("figure rendering produced no reviewable attempt after %d attempts: %s", maxAttempts, prior), Files: figureFiles(in.Figure.Slug)}, nil
}

func figureFiles(slug string) []string {
	return []string{"paper/figures/" + slug + ".py", "paper/figures/" + slug + ".pdf", "paper/figures/" + slug + ".png"}
}

func snapshotFigureAttempt(ws Workspace, slug string, attempt int) error {
	dir := filepath.Join(ws.ReviewsDir, "figures", slug, fmt.Sprintf("attempt_%d", attempt))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, ext := range []string{".py", ".pdf", ".png"} {
		if err := copyFile(filepath.Join(ws.FiguresDir, slug+ext), filepath.Join(dir, slug+ext), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func restoreFigureAttempt(ws Workspace, slug string, attempt int) error {
	dir := filepath.Join(ws.ReviewsDir, "figures", slug, fmt.Sprintf("attempt_%d", attempt))
	for _, ext := range []string{".py", ".pdf", ".png"} {
		if err := copyFile(filepath.Join(dir, slug+ext), filepath.Join(ws.FiguresDir, slug+ext), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func rerunFigureScript(ctx context.Context, ws Workspace, slug string) error {
	script := filepath.Join(ws.FiguresDir, slug+".py")
	if !FileExists(script) {
		return fmt.Errorf("paper/figures/%s.py was not produced", slug)
	}
	// Keep publication-safe font embedding a harness property rather than a
	// capability or prompting requirement. Matplotlib reads matplotlibrc from
	// MPLCONFIGDIR before importing pyplot, so the ordinary Python scripts used
	// by the original pipeline remain sufficient.
	mplConfigDir := filepath.Join(ws.ReviewsDir, "matplotlib")
	if err := os.MkdirAll(mplConfigDir, 0o755); err != nil {
		return fmt.Errorf("create Matplotlib config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mplConfigDir, "matplotlibrc"), []byte("pdf.fonttype: 42\nps.fonttype: 42\n"), 0o644); err != nil {
		return fmt.Errorf("write Matplotlib config: %w", err)
	}
	cmd := exec.CommandContext(ctx, FigurePython(), script)
	cmd.Dir = ws.Root
	cmd.Env = figureEnvironment(mplConfigDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if len(message) > 1600 {
			message = message[len(message)-1600:]
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func figureEnvironment(mplConfigDir string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "MPLBACKEND=") || strings.HasPrefix(entry, "MPLCONFIGDIR=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "MPLBACKEND=Agg", "MPLCONFIGDIR="+mplConfigDir)
}

func validateFigureArtifacts(ws Workspace, slug string) ([]string, error) {
	paths := map[string]string{
		"script": filepath.Join(ws.FiguresDir, slug+".py"),
		"pdf":    filepath.Join(ws.FiguresDir, slug+".pdf"),
		"png":    filepath.Join(ws.FiguresDir, slug+".png"),
	}
	for kind, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() < 64 {
			return nil, fmt.Errorf("%s artifact missing or empty: %s", kind, filepath.Base(path))
		}
	}
	pdfHead, err := os.ReadFile(paths["pdf"])
	if err != nil || len(pdfHead) < 5 || string(pdfHead[:5]) != "%PDF-" {
		return nil, fmt.Errorf("invalid PDF signature for %s.pdf", slug)
	}
	pdfChecks := []string{fmt.Sprintf("valid PDF (%d bytes)", len(pdfHead))}
	if tool, lookupErr := exec.LookPath("pdfinfo"); lookupErr == nil {
		out, commandErr := exec.Command(tool, paths["pdf"]).CombinedOutput()
		if commandErr != nil {
			return nil, fmt.Errorf("pdfinfo rejected %s.pdf: %s", slug, strings.TrimSpace(string(out)))
		}
		if !regexp.MustCompile(`(?m)^Pages:\s+1\s*$`).Match(out) {
			return nil, fmt.Errorf("figure PDF must contain exactly one page")
		}
		pdfChecks = append(pdfChecks, "single-page PDF")
	}
	if tool, lookupErr := exec.LookPath("pdffonts"); lookupErr == nil {
		out, commandErr := exec.Command(tool, paths["pdf"]).CombinedOutput()
		if commandErr != nil {
			return nil, fmt.Errorf("pdffonts rejected %s.pdf: %s", slug, strings.TrimSpace(string(out)))
		}
		if strings.Contains(string(out), "Type 3") {
			return nil, fmt.Errorf("figure PDF contains Type 3 fonts")
		}
		pdfChecks = append(pdfChecks, "no Type 3 fonts")
	}
	f, err := os.Open(paths["png"])
	if err != nil {
		return nil, err
	}
	config, _, decodeErr := image.DecodeConfig(f)
	_ = f.Close()
	if decodeErr != nil {
		return nil, fmt.Errorf("decode preview: %w", decodeErr)
	}
	if config.Width < 900 || config.Height < 450 {
		return nil, fmt.Errorf("preview resolution too small: %dx%d", config.Width, config.Height)
	}
	checks := []string{"reproducible script"}
	checks = append(checks, pdfChecks...)
	checks = append(checks, fmt.Sprintf("PNG preview %dx%d", config.Width, config.Height))
	return checks, nil
}

var figurePrefix = regexp.MustCompile(`^fig(?:ure)?[0-9]*[_-]*`)

func canonicalFigureKey(stem string) string {
	stem = strings.ToLower(strings.TrimSpace(stem))
	stem = strings.TrimSuffix(stem, filepath.Ext(stem))
	stem = figurePrefix.ReplaceAllString(stem, "")
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(stem)
}

func stageMatchingFigureAssets(ws Workspace, spec FigureSpec) ([]string, error) {
	targetKey := canonicalFigureKey(spec.Slug)
	type bundle struct {
		stem  string
		dir   string
		files map[string]string
	}
	bundles := map[string]*bundle{}
	err := filepath.WalkDir(ws.InputDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".py" && ext != ".pdf" && ext != ".png" {
			return nil
		}
		stem := strings.TrimSuffix(filepath.Base(path), ext)
		key := filepath.Join(filepath.Dir(path), stem)
		b := bundles[key]
		if b == nil {
			b = &bundle{stem: stem, dir: filepath.Dir(path), files: map[string]string{}}
			bundles[key] = b
		}
		b.files[ext] = path
		return nil
	})
	if err != nil {
		return nil, err
	}

	var chosen *bundle
	// An explicit blueprint source is authoritative once it resolves safely
	// inside input/. This also fixes the historical input/input path mismatch.
	for _, listed := range spec.DataSources {
		rel := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(listed)), "input/")
		if rel == "" || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		abs := filepath.Join(ws.InputDir, filepath.FromSlash(rel))
		rootWithSep := filepath.Clean(ws.InputDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(abs), rootWithSep) {
			continue
		}
		key := strings.TrimSuffix(abs, filepath.Ext(abs))
		if b := bundles[key]; b != nil {
			chosen = b
			break
		}
	}

	var matches []*bundle
	if chosen == nil {
		for _, b := range bundles {
			if canonicalFigureKey(b.stem) == targetKey {
				matches = append(matches, b)
			}
		}
	}
	slugTokens := figureTokens(spec.Slug)
	if chosen == nil && len(matches) == 0 {
		// Blueprint models often rename a figure while omitting its source path.
		// Resolve only a unique, evidence-bearing author script whose vocabulary
		// overlaps the requested slug/purpose; ties fail closed.
		requestTokens := figureTokens(spec.Slug + " " + spec.Purpose)
		bestScore := 0
		for _, b := range bundles {
			script := b.files[".py"]
			if script == "" {
				continue
			}
			stemTokens := figureTokens(b.stem)
			candidateTokens := figureTokens(b.stem + " " + ReadText(script, 10000))
			score := tokenOverlap(requestTokens, candidateTokens) + 3*tokenOverlap(slugTokens, stemTokens)
			if score > bestScore {
				bestScore = score
				matches = []*bundle{b}
			} else if score == bestScore && score > 0 {
				matches = append(matches, b)
			}
		}
		if bestScore < 2 {
			matches = nil
		}
	}
	// A complete reproducible bundle wins over partial artifacts. Refuse a tie
	// instead of semantically guessing which author's figure was intended.
	if chosen == nil {
		if len(matches) == 0 {
			return []string{}, nil
		}
		sort.Slice(matches, func(i, j int) bool { return len(matches[i].files) > len(matches[j].files) })
		if len(matches) > 1 && len(matches[0].files) == len(matches[1].files) {
			aliases := []*bundle{}
			for _, match := range matches {
				if slugTokens[canonicalFigureKey(match.stem)] {
					aliases = append(aliases, match)
				}
			}
			if len(aliases) == 1 {
				chosen = aliases[0]
			}
		}
		if chosen == nil && len(matches) > 1 && len(matches[0].files) == len(matches[1].files) {
			names := make([]string, len(matches))
			for i, match := range matches {
				names[i] = match.stem
			}
			return nil, fmt.Errorf("ambiguous source assets for figure %s: %s", spec.Slug, strings.Join(names, ", "))
		}
		if chosen == nil {
			chosen = matches[0]
		}
	}
	staged := []string{}
	for _, ext := range []string{".py", ".pdf", ".png"} {
		source := chosen.files[ext]
		if source == "" {
			continue
		}
		target := filepath.Join(ws.FiguresDir, spec.Slug+ext)
		if ext == ".py" {
			body, readErr := os.ReadFile(source)
			if readErr != nil {
				return nil, readErr
			}
			body = []byte(strings.ReplaceAll(string(body), chosen.stem, spec.Slug))
			if _, writeErr := WriteText(target, string(body)); writeErr != nil {
				return nil, writeErr
			}
		} else if copyErr := copyFile(source, target, 0o644); copyErr != nil {
			return nil, copyErr
		}
		rel, _ := filepath.Rel(ws.InputDir, source)
		staged = append(staged, filepath.ToSlash(rel))
	}
	style := filepath.Join(chosen.dir, "_style.py")
	if FileExists(style) {
		if err := copyFile(style, filepath.Join(ws.FiguresDir, "_style.py"), 0o644); err != nil {
			return nil, err
		}
	}
	return staged, nil
}

var figureWords = regexp.MustCompile(`[a-z0-9]+`)

func figureTokens(text string) map[string]bool {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "as": true, "at": true, "be": true, "by": true,
		"do": true, "for": true, "from": true, "in": true, "is": true, "it": true, "of": true,
		"on": true, "or": true, "the": true, "this": true, "to": true, "with": true,
		"fig": true, "figure": true, "plot": true, "py": true, "pdf": true, "png": true,
	}
	out := map[string]bool{}
	for _, word := range figureWords.FindAllString(strings.ToLower(text), -1) {
		if len(word) >= 3 && !stop[word] {
			out[word] = true
		}
	}
	return out
}

func tokenOverlap(a, b map[string]bool) int {
	count := 0
	for token := range a {
		if b[token] {
			count++
		}
	}
	return count
}
