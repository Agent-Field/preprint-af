package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Agent-Field/agentfield/sdk/go/ai"
	"github.com/Agent-Field/agentfield/sdk/go/harness"
)

func TestTextCapsCountUnicodeCodePointsLikePython(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unicode.txt")
	if err := os.WriteFile(path, []byte("é漢🙂tail"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := ReadText(path, 3), "é漢🙂"; got != want {
		t.Fatalf("ReadText unicode cap = %q, want %q", got, want)
	}
	if got, want := truncate("é漢🙂tail", 3), "é漢🙂"; got != want {
		t.Fatalf("truncate unicode cap = %q, want %q", got, want)
	}
}

func TestExpandUserMatchesPythonWorkspaceInputSemantics(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got, want := expandUser("~/papers"), filepath.Join(home, "papers"); got != want {
		t.Fatalf("expandUser = %q, want %q", got, want)
	}
	if got := expandUser("~someone/papers"); got != "~someone/papers" {
		t.Fatalf("named-user path changed to %q", got)
	}
}

func TestEmptyNeighborObjectsMeanNoSection(t *testing.T) {
	var input WriteSectionInput
	if err := json.Unmarshal([]byte(`{"workspace":{},"section":{},"prev_section":{},"next_section":{}}`), &input); err != nil {
		t.Fatal(err)
	}
	prev, err := optionalSection(input.PrevSection)
	if err != nil || prev != nil {
		t.Fatalf("empty prev_section = %#v, %v; want nil, nil", prev, err)
	}
	next, err := optionalSection(input.NextSection)
	if err != nil || next != nil {
		t.Fatalf("empty next_section = %#v, %v; want nil, nil", next, err)
	}
}

func TestPositioningArtifactUsesReferenceOrderingAndBooleans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "POSITIONING.md")
	ws := Workspace{PositioningPath: path}
	frames := []StoryFrame{{Name: "frame-b"}, {Name: "frame-a"}}
	judgments := []FrameJudgment{
		{FrameName: "frame-b", Persona: "zeta, details"},
		{FrameName: "frame-a", Persona: "alpha, details"},
		{FrameName: "frame-b", Persona: "alpha, details"},
	}
	if err := writePositioning(ws, PositioningDecision{}, frames, judgments, NoveltyScan{Confident: true}); err != nil {
		t.Fatal(err)
	}
	text := ReadText(path)
	alphaB := strings.Index(text, "| frame-b | alpha |")
	zetaB := strings.Index(text, "| frame-b | zeta |")
	alphaA := strings.Index(text, "| frame-a | alpha |")
	if alphaB < 0 || zetaB < alphaB || alphaA < zetaB {
		t.Fatalf("judgment rows are not frame-order/persona-order:\n%s", text)
	}
	if !strings.Contains(text, "Confident: True") {
		t.Fatalf("positioning artifact did not use Python boolean spelling:\n%s", text)
	}
}

func TestPositioningArtifactRendersEmptyJudgmentRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "POSITIONING.md")
	if err := writePositioning(Workspace{PositioningPath: path}, PositioningDecision{}, nil, nil, NoveltyScan{}); err != nil {
		t.Fatal(err)
	}
	if text := ReadText(path); !strings.Contains(text, "| (none) | (none) | 0.00 | 0.00 | 0.00 | 0.00 |") {
		t.Fatalf("empty judgment sentinel missing:\n%s", text)
	}
}

func TestRoundArtifactsUsePythonBooleanSpelling(t *testing.T) {
	dir := t.TempDir()
	bundle := CritiqueBundle{
		Round:          1,
		PersonaReviews: []PersonaReview{{Confident: true}},
		Narrative:      NarrativeReview{Confident: true},
		Fidelity:       FidelityAudit{Blocking: true, Confident: false},
	}
	if err := writeRoundArtifacts(Workspace{ReviewsDir: dir}, bundle); err != nil {
		t.Fatal(err)
	}
	persona := ReadText(filepath.Join(dir, "round_1", "domain-expert.md"))
	if !strings.Contains(persona, "**Confident:** True") {
		t.Fatalf("persona boolean spelling drifted:\n%s", persona)
	}
	fidelity := ReadText(filepath.Join(dir, "round_1", "fidelity.md"))
	if !strings.Contains(fidelity, "**Blocking:** True") || !strings.Contains(fidelity, "**Confident:** False") {
		t.Fatalf("fidelity boolean spelling drifted:\n%s", fidelity)
	}
}

func TestRoundArtifactsReturnWriteErrors(t *testing.T) {
	parent := t.TempDir()
	blocked := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(blocked, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := writeRoundArtifacts(Workspace{ReviewsDir: blocked}, CritiqueBundle{Round: 1, PersonaReviews: []PersonaReview{{}}})
	if err == nil {
		t.Fatal("writeRoundArtifacts swallowed a filesystem error")
	}
}

func TestRound4MatchesPythonRound(t *testing.T) {
	// Values verified against CPython: round(v, 4).
	for _, tc := range []struct {
		in, want float64
	}{
		{0.00005, 0.0001}, // not a binary tie: the double is just above 5e-05
		{0.03125, 0.0312}, // exact tie (1/32) -> nearest even
		{0.09375, 0.0938}, // exact tie (3/32) -> nearest even
		{0.12345, 0.1235},
		{0.55555, 0.5555},
	} {
		if got := round4(tc.in); got != tc.want {
			t.Errorf("round4(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSaveStateUsesPythonJSONEncoding(t *testing.T) {
	dir := t.TempDir()
	if err := SaveState(dir, map[string]any{"title": "A — B < C"}); err != nil {
		t.Fatal(err)
	}
	text := ReadText(filepath.Join(dir, "STATE.json"))
	if !strings.Contains(text, `"A \u2014 B < C"`) {
		t.Fatalf("STATE.json encoding drifted from json.dumps: %s", text)
	}
}

// failingApp is an App whose harness always fails, the way a crashed or
// unavailable OpenCode subprocess does.
type failingApp struct{}

func (failingApp) AI(context.Context, string, ...ai.Option) (*ai.Response, error) {
	return nil, errors.New("ai unavailable")
}

func (failingApp) Harness(ctx context.Context, _ string, _ map[string]any, _ any, _ harness.Options) (*harness.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("opencode exited 1")
}

func (failingApp) Call(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, errors.New("call unavailable")
}

func (failingApp) CallLocal(context.Context, string, map[string]any) (any, error) {
	return nil, errors.New("call unavailable")
}

func TestScanNoveltyDegradesOnHarnessFailure(t *testing.T) {
	// Python's scan_novelty catches any harness exception and returns
	// safe_ai_fallback(NoveltyScan); only cancellation aborts positioning.
	svc := New(failingApp{}, "preprint-af")
	out, err := svc.ScanNovelty(context.Background(), ScanNoveltyInput{
		Workspace: Workspace{Root: t.TempDir()},
		FrameSet:  map[string]any{"frames": []any{}},
	})
	if err != nil {
		t.Fatalf("harness failure aborted positioning: %v", err)
	}
	scan, ok := out.(NoveltyScan)
	if !ok {
		t.Fatalf("ScanNovelty returned %T, want NoveltyScan", out)
	}
	if scan.Confident {
		t.Fatal("degraded novelty scan must not claim confidence")
	}
}

func TestScanNoveltyPropagatesCancellation(t *testing.T) {
	svc := New(failingApp{}, "preprint-af")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ScanNovelty(ctx, ScanNoveltyInput{
		Workspace: Workspace{Root: t.TempDir()},
		FrameSet:  map[string]any{"frames": []any{}},
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan returned %v, want context.Canceled", err)
	}
}

// scriptedApp answers every direct-AI call with a fixed JSON payload.
type scriptedApp struct{ json string }

func (a scriptedApp) AI(context.Context, string, ...ai.Option) (*ai.Response, error) {
	return &ai.Response{Choices: []ai.Choice{{Message: ai.Message{
		Content: []ai.ContentPart{{Type: "text", Text: a.json}},
	}}}}, nil
}

func (scriptedApp) Harness(context.Context, string, map[string]any, any, harness.Options) (*harness.Result, error) {
	return nil, errors.New("not used")
}

func (scriptedApp) Call(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, errors.New("not used")
}

func (scriptedApp) CallLocal(context.Context, string, map[string]any) (any, error) {
	return nil, errors.New("not used")
}

func TestFidelityOverlayFlagsEveryInventedKey(t *testing.T) {
	// Python snapshots citation_issues before appending, so a finding for
	// "smith2024" cannot suppress the finding for the shorter key "smith".
	root := t.TempDir()
	paper := filepath.Join(root, "paper")
	if err := os.MkdirAll(filepath.Join(paper, "sections"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `\section{R}\cite{smith2024} and \cite{smith}.`
	if err := os.WriteFile(filepath.Join(paper, "sections", "01_intro.tex"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paper, "refs.bib"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New(scriptedApp{json: `{"unsupported_claims":[],"number_mismatches":[],"citation_issues":[],"blocking":false,"score":1,"confident":true}`}, "preprint-af")
	out, err := svc.FidelityAudit(context.Background(), FidelityAuditInput{
		Workspace: Workspace{Root: root, PaperDir: paper},
		RoundNo:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit := out.(FidelityAudit)
	for _, key := range []string{"smith2024", "smith}"} {
		found := false
		for _, issue := range audit.CitationIssues {
			if strings.Contains(issue, key) {
				found = true
			}
		}
		if !found {
			t.Errorf("no deterministic finding for %q: %v", key, audit.CitationIssues)
		}
	}
	if !audit.Blocking || audit.Score > .5 {
		t.Errorf("invented keys must block and cap the score: blocking=%v score=%v", audit.Blocking, audit.Score)
	}
}

func TestAppendTODOsIsSafeUnderConcurrentFigureWorkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TODO.md")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := AppendTODOs(path, []string{fmt.Sprintf("figure %d is not buildable", i)}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	text := ReadText(path)
	for i := 0; i < 16; i++ {
		if !strings.Contains(text, fmt.Sprintf("figure %d is not buildable", i)) {
			t.Fatalf("concurrent append lost figure %d:\n%s", i, text)
		}
	}
}

func TestTitleFallbackStripsTitlePrefixCaseInsensitively(t *testing.T) {
	// Python lowercases before comparing: `if title.lower().startswith("title:")`.
	for _, heading := range []string{"## TITLE: Example", "## Title: Example", "## title: Example"} {
		if got, _ := parseFrontMatter(heading); got != "Example" {
			t.Errorf("parseFrontMatter(%q) title = %q, want %q", heading, got, "Example")
		}
	}
}
