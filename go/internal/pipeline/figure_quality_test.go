package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFigureEnvironmentOverridesMatplotlibSettings(t *testing.T) {
	env := figureEnvironment("/tmp/preprint-af-mpl")
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nMPLBACKEND=Agg\n") {
		t.Fatal("MPLBACKEND override missing")
	}
	if !strings.Contains(joined, "\nMPLCONFIGDIR=/tmp/preprint-af-mpl\n") {
		t.Fatal("MPLCONFIGDIR override missing")
	}
}

func TestCanonicalFigureKeyMapsAuthorOrdinals(t *testing.T) {
	cases := map[string]string{
		"fig_mechanism":          "mechanism",
		"fig1_mechanism.py":      "mechanism",
		"figure2-frontier.pdf":   "frontier",
		"fig_embedding_quality":  "embeddingquality",
		"fig6_decision_boundary": "decisionboundary",
	}
	for input, want := range cases {
		if got := canonicalFigureKey(input); got != want {
			t.Errorf("canonicalFigureKey(%q)=%q want %q", input, got, want)
		}
	}
}

func TestStageMatchingFigureAssetsMapsAndRenamesBundle(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	figures := filepath.Join(root, "paper", "figures")
	if err := os.MkdirAll(filepath.Join(input, "figures"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(figures, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(input, "figures", "fig1_mechanism")
	if _, err := WriteText(source+".py", `open("fig1_mechanism.pdf", "wb").write(b"%PDF-1.4")`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source+".pdf", []byte("%PDF-1.4\nasset"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source+".png", make([]byte, 80), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := Workspace{Root: root, InputDir: input, FiguresDir: figures}
	assets, err := stageMatchingFigureAssets(ws, FigureSpec{Slug: "fig_mechanism"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 {
		t.Fatalf("got %d staged assets, want 3: %#v", len(assets), assets)
	}
	script := ReadText(filepath.Join(figures, "fig_mechanism.py"))
	if script == "" || canonicalFigureKey(filepath.Base(filepath.Join(figures, "fig_mechanism.py"))) != "mechanism" {
		t.Fatalf("renamed target script missing: %q", script)
	}
	if script == ReadText(source+".py") {
		t.Fatal("script output basename was not rewritten")
	}
}

func TestStageMatchingFigureAssetsRefusesAmbiguousAlias(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	figures := filepath.Join(root, "paper", "figures")
	for _, dir := range []string{filepath.Join(input, "a"), filepath.Join(input, "b"), figures} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{filepath.Join(input, "a", "fig1_grid.py"), filepath.Join(input, "b", "fig4_grid.py")} {
		if _, err := WriteText(path, "print('x')"); err != nil {
			t.Fatal(err)
		}
	}
	_, err := stageMatchingFigureAssets(Workspace{InputDir: input, FiguresDir: figures}, FigureSpec{Slug: "fig_grid"})
	if err == nil {
		t.Fatal("expected ambiguous alias error")
	}
}

func TestStageMatchingFigureAssetsHonorsExplicitRenamedSource(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	figures := filepath.Join(root, "paper", "figures")
	sourceDir := filepath.Join(input, "figures")
	for _, dir := range []string{sourceDir, figures} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, ext := range []string{".py", ".pdf", ".png"} {
		body := []byte("author asset fig1_mechanism")
		if err := os.WriteFile(filepath.Join(sourceDir, "fig1_mechanism"+ext), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	assets, err := stageMatchingFigureAssets(
		Workspace{InputDir: input, FiguresDir: figures},
		FigureSpec{Slug: "serve_mechanism", DataSources: []string{"input/figures/fig1_mechanism.py"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 3 || !FileExists(filepath.Join(figures, "serve_mechanism.py")) {
		t.Fatalf("explicit source was not staged: %#v", assets)
	}
}

func TestStageMatchingFigureAssetsUsesUniqueScriptVocabulary(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	figures := filepath.Join(root, "paper", "figures")
	sourceDir := filepath.Join(input, "figures")
	for _, dir := range []string{sourceDir, figures} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	scripts := map[string]string{
		"fig1_sota_ablation.py": "matched budget cumulative feature ablation baselines",
		"fig2_frontier.py":      "operating frontier strict budget hit rate gain",
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(sourceDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	assets, err := stageMatchingFigureAssets(
		Workspace{InputDir: input, FiguresDir: figures},
		FigureSpec{Slug: "matched_budget_ablation", Purpose: "Primary matched budget cumulative feature ablation"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0] != "figures/fig1_sota_ablation.py" {
		t.Fatalf("unique semantic source was not resolved: %#v", assets)
	}
}

func TestAuthorSuppliedFigureDesignRequiresCompleteBundle(t *testing.T) {
	spec := FigureSpec{Purpose: "Primary result", DataSources: []string{"input/figures/result.py"}}
	if _, ok := authorSuppliedFigureDesign(spec, []string{"result.py", "result.pdf"}); ok {
		t.Fatal("incomplete author bundle unexpectedly accepted")
	}
	design, ok := authorSuppliedFigureDesign(spec, []string{"result.py", "result.pdf", "result.png"})
	if !ok || !design.Buildable || !design.Confident || design.Composition != spec.Purpose {
		t.Fatalf("complete author bundle rejected: %#v", design)
	}
}

func TestNonBuildableFigureBecomesTODOWithoutIdeation(t *testing.T) {
	root := t.TempDir()
	todo := filepath.Join(root, "TODO.md")
	result, err := (&Service{}).buildFigureWithVisualQA(context.Background(), BuildFigureInput{
		Workspace: Workspace{TODOPath: todo},
		Figure:    FigureSpec{Slug: "future_plot", Purpose: "Future experiment", Buildable: false},
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := result.(WorkerResult)
	if worker.Status != "todo" || !strings.Contains(ReadText(todo, 0), "future_plot") {
		t.Fatalf("non-buildable figure was not preserved as TODO: %#v", worker)
	}
}

func TestBundledExampleResolvesRenamedBlueprintFigures(t *testing.T) {
	input, err := filepath.Abs("../../../examples/serve-paper")
	if err != nil {
		t.Fatal(err)
	}
	ws := Workspace{InputDir: input, FiguresDir: t.TempDir()}
	cases := []struct {
		spec FigureSpec
		want string
	}{
		{FigureSpec{Slug: "serve_mechanism", Purpose: "Mechanism schematic", DataSources: []string{"input/figures/fig1_mechanism.py"}}, "figures/fig1_mechanism.py"},
		{FigureSpec{Slug: "matched_budget_ablation", Purpose: "Primary matched-budget comparison and cumulative feature-ablation figure"}, "figures/fig1_sota_ablation.py"},
		{FigureSpec{Slug: "auc_operating_point", Purpose: "Paired AUC-versus-operating-point evidence"}, "figures/fig2_frontier.py"},
		{FigureSpec{Slug: "recurrence_sweep", Purpose: "Recurrence sensitivity analysis"}, "figures/fig3_recurrence.py"},
		{FigureSpec{Slug: "recurrence_budget_grid", Purpose: "Conditional recurrence and budget regime analysis"}, "figures/fig4_grid.py"},
		{FigureSpec{Slug: "paws_diagnostic", Purpose: "Cross-workload PAWS diagnostic ROC AUC"}, "figures/fig5_paws.py"},
		{FigureSpec{Slug: "synthetic_decision_boundary", Purpose: "Synthetic decision boundary mechanistic intuition"}, "figures/fig6_decision_boundary.py"},
		{FigureSpec{Slug: "budgeted_hit_rate_ablation", Purpose: "Present the primary budgeted utility comparison and the supporting cumulative feature ablation without implying causal attribution."}, "figures/fig1_sota_ablation.py"},
		{FigureSpec{Slug: "operating_point_vs_auc", Purpose: "Contrast budget-specific utility with aggregate discrimination and foreground the paper's evaluation thesis."}, "figures/fig2_frontier.py"},
		{FigureSpec{Slug: "recurrence_supporting_analysis", Purpose: "Provide a bounded supporting analysis of when budgeted reuse gains are largest."}, "figures/fig3_recurrence.py"},
		{FigureSpec{Slug: "recurrence_grid_audit", Purpose: "Use only as a clearly marked audit or TODO figure, not as a settled empirical result."}, "figures/fig4_grid.py"},
		{FigureSpec{Slug: "paws_supporting_result", Purpose: "Show cross-dataset supporting evidence while preserving the narrower QQP/MiniLM positioning."}, "figures/fig5_paws.py"},
		{FigureSpec{Slug: "illustrative_feature_boundary", Purpose: "Avoid presenting the synthetic visualization as empirical evidence; retain only as a clearly labeled illustrative schematic if needed."}, "figures/fig6_decision_boundary.py"},
	}
	for _, tc := range cases {
		assets, resolveErr := stageMatchingFigureAssets(ws, tc.spec)
		if resolveErr != nil {
			t.Errorf("%s: %v", tc.spec.Slug, resolveErr)
			continue
		}
		found := false
		for _, asset := range assets {
			found = found || asset == tc.want
		}
		if !found {
			t.Errorf("%s resolved %#v, want %s", tc.spec.Slug, assets, tc.want)
		}
	}
}
