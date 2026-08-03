package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyPaperPresentationPolicyDefaultsToCleanAndIsReversible(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{PaperDir: filepath.Join(root, "paper")}
	if err := os.MkdirAll(ws.PaperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	main := "\\documentclass{article}\n\\newcommand{\\todobox}[1]{TODO: #1}\n\\begin{document}\n\\todobox{missing evidence}\n\\end{document}\n"
	if _, err := WriteText(filepath.Join(ws.PaperDir, "main.tex"), main); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPaperPresentationPolicy(ws, false); err != nil {
		t.Fatal(err)
	}
	clean := ReadText(filepath.Join(ws.PaperDir, "main.tex"), 0)
	if strings.Count(clean, cleanPaperBegin) != 1 || !strings.Contains(clean, `\renewcommand{\todobox}[1]{}`) {
		t.Fatalf("clean policy was not installed exactly once:\n%s", clean)
	}
	if err := ApplyPaperPresentationPolicy(ws, false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(ReadText(filepath.Join(ws.PaperDir, "main.tex"), 0), cleanPaperBegin) != 1 {
		t.Fatal("clean policy is not idempotent")
	}
	if err := ApplyPaperPresentationPolicy(ws, true); err != nil {
		t.Fatal(err)
	}
	visible := ReadText(filepath.Join(ws.PaperDir, "main.tex"), 0)
	if strings.Contains(visible, cleanPaperBegin) || strings.Contains(visible, `\renewcommand{\todobox}[1]{}`) {
		t.Fatalf("show_todos did not remove clean policy:\n%s", visible)
	}
}
