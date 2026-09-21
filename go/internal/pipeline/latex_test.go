package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunLatexmkRejectsPartialPDFOnNonzeroExit(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "latexmk")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch main.pdf\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	paper := t.TempDir()
	ok, pdf, _ := RunLatexmk(paper)
	if ok {
		t.Fatalf("nonzero latexmk exit accepted partial PDF %q", pdf)
	}
}
