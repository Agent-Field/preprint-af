package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreMissingFigureBlocksPreservesValidatedBuildFigures(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{
		PaperDir:    filepath.Join(root, "paper"),
		SectionsDir: filepath.Join(root, "paper", "sections"),
	}
	if err := os.MkdirAll(filepath.Join(ws.PaperDir, "figures"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ws.SectionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.PaperDir, "figures", "result.pdf"), []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	section := filepath.Join(ws.SectionsDir, "05_results.tex")
	block := "\\begin{figure}[t]\n\\centering\n\\includegraphics[width=\\columnwidth]{figures/result.pdf}\n\\caption{Result.}\n\\end{figure}"
	if _, err := WriteText(section, "\\section{Results}\n\n"+block+"\n"); err != nil {
		t.Fatal(err)
	}
	preserved := snapshotFigureBlocks(ws)
	if len(preserved) != 1 {
		t.Fatalf("preserved %d blocks, want 1", len(preserved))
	}
	if _, err := WriteText(section, "\\section{Results}\n\nRepaired prose without the figure.\n"); err != nil {
		t.Fatal(err)
	}
	if err := restoreMissingFigureBlocks(ws, preserved); err != nil {
		t.Fatal(err)
	}
	got := ReadText(section, 0)
	if !strings.Contains(got, "{figures/result.pdf}") || !strings.Contains(got, "\\caption{Result.}") {
		t.Fatalf("validated figure block was not restored:\n%s", got)
	}
	if err := restoreMissingFigureBlocks(ws, preserved); err != nil {
		t.Fatal(err)
	}
	if strings.Count(ReadText(section, 0), "{figures/result.pdf}") != 1 {
		t.Fatal("restoration duplicated an existing figure")
	}
}
