package pipeline

import "testing"

func TestBuildConfidenceMatchesPythonContract(t *testing.T) {
	sections := []WorkerResult{{Status: "done"}, {Status: "done"}}
	if !buildConfident(sections, WorkerResult{Status: "done"}) {
		t.Fatal("completed sections and bibliography must be confident")
	}
	if buildConfident(append(sections, WorkerResult{Status: "failed"}), WorkerResult{Status: "done"}) {
		t.Fatal("failed section must fail confidence")
	}
	if buildConfident(sections, WorkerResult{Status: "failed"}) {
		t.Fatal("failed bibliography must fail confidence")
	}
}
