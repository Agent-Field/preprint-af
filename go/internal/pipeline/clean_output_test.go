package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTODOBoxesNestedMultilineEscapedAndCommented(t *testing.T) {
	tex := "% \\todobox{ignored}\n\\todobox{need {nested}\nwork and \\{literal\\}} text \\todobox { second }"
	got, err := ExtractTODOBoxes(tex)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.Contains(got[0], "{nested}") || got[1] != "second" {
		t.Fatalf("unexpected TODO boxes: %#v", got)
	}
}

func TestSyncManuscriptTODOsPreservesManualAndRemovesResolvedManagedEntries(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: root, PaperDir: filepath.Join(root, "paper"), SectionsDir: filepath.Join(root, "paper", "sections"), TODOPath: filepath.Join(root, "TODO.md")}
	if err := os.MkdirAll(ws.SectionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _ = WriteText(ws.TODOPath, "# TODO\n\n- manual item\n")
	_, _ = WriteText(filepath.Join(ws.PaperDir, "main.tex"), "\\newcommand{\\todobox}[1]{}\n\\begin{document}\n\\todobox{front}\n")
	section := filepath.Join(ws.SectionsDir, "01_results.tex")
	_, _ = WriteText(section, "\\todobox{result gap}\n")
	if n, err := SyncManuscriptTODOs(ws); err != nil || n != 2 {
		t.Fatalf("first sync n=%d err=%v", n, err)
	}
	_, _ = WriteText(section, "resolved\n")
	if n, err := SyncManuscriptTODOs(ws); err != nil || n != 1 {
		t.Fatalf("second sync n=%d err=%v", n, err)
	}
	got := ReadText(ws.TODOPath, 0)
	if !strings.Contains(got, "manual item") || !strings.Contains(got, "front") || strings.Contains(got, "result gap") || strings.Count(got, manuscriptTODOBegin) != 1 {
		t.Fatalf("bad managed TODO content:\n%s", got)
	}
}

func TestSyncManuscriptTODOsRejectsMalformedManagedMarkers(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: root, PaperDir: filepath.Join(root, "paper"), SectionsDir: filepath.Join(root, "paper", "sections"), TODOPath: filepath.Join(root, "TODO.md")}
	_ = os.MkdirAll(ws.SectionsDir, 0o755)
	_, _ = WriteText(ws.TODOPath, manuscriptTODOBegin+"\n")
	if _, err := SyncManuscriptTODOs(ws); err == nil {
		t.Fatal("expected malformed marker error")
	}
}
