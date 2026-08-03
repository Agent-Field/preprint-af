package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvidenceLedgerValidFailsClosed(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{EvidencePath: filepath.Join(root, "EVIDENCE.md")}
	_, _ = WriteText(ws.EvidencePath, "# Evidence\n")
	if ok, _ := EvidenceLedgerValid(ws, EvidenceSummary{FactCount: 1, Confident: true}); ok {
		t.Fatal("empty ledger passed")
	}
	_, _ = WriteText(ws.EvidencePath, "### E1\nFact\n### E1\nDuplicate\n")
	if ok, _ := EvidenceLedgerValid(ws, EvidenceSummary{FactCount: 2, Confident: true}); ok {
		t.Fatal("duplicate ledger id passed")
	}
	_, _ = WriteText(ws.EvidencePath, "### E1\nFact\n")
	if ok, _ := EvidenceLedgerValid(ws, EvidenceSummary{FactCount: 1, Confident: false}); ok {
		t.Fatal("unconfident ledger passed")
	}
	if ok, reason := EvidenceLedgerValid(ws, EvidenceSummary{FactCount: 1, Confident: true}); !ok {
		t.Fatalf("valid ledger failed: %s", reason)
	}
}

func TestDeterministicFactualFindingsTraceAndCitation(t *testing.T) {
	evidence := "### E1\nFact\n"
	body := "Accuracy reached 91.2 percent.\n% E1\nUnsupported value is 73.4 percent.\nSee \\cite{invented}.\n% E99\n"
	got := deterministicFactualFindings("results", "paper/sections/03_results.tex", body, evidence, []string{"real"})
	kinds := map[string]bool{}
	for _, finding := range got {
		kinds[finding.Kind] = true
		if !finding.Blocking || finding.Target != "results" {
			t.Fatalf("finding is not exact and blocking: %#v", finding)
		}
	}
	for _, kind := range []string{"missing_evidence_trace", "citation", "invalid_evidence_trace"} {
		if !kinds[kind] {
			t.Fatalf("missing %s finding: %#v", kind, got)
		}
	}
}

func TestFactualScopesAreExactAndStable(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: root, PaperDir: filepath.Join(root, "paper"), SectionsDir: filepath.Join(root, "paper", "sections")}
	if err := os.MkdirAll(ws.SectionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, _ = WriteText(filepath.Join(ws.SectionsDir, "02_results.tex"), "results")
	_, _ = WriteText(filepath.Join(ws.SectionsDir, "01_intro.tex"), "intro")
	got, err := factualScopes(ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Target != "front_matter" || got[1].Target != "intro" || got[2].Target != "results" {
		t.Fatalf("unstable scopes: %#v", got)
	}
}
