package pipeline

import (
	"testing"
	"time"
)

func TestStructuredTokenBudgetsFavorPlanningArtifacts(t *testing.T) {
	if got := structuredTokenBudget[FrameSet](); got != 8192 {
		t.Fatalf("FrameSet budget=%d", got)
	}
	if got := structuredTokenBudget[Blueprint](); got != 8192 {
		t.Fatalf("Blueprint budget=%d", got)
	}
	if got := structuredTokenBudget[FrameJudgment](); got != 3072 {
		t.Fatalf("atomic judgment budget=%d", got)
	}
	if got := structuredAttemptTimeout[Blueprint](); got != 4*time.Minute {
		t.Fatalf("Blueprint timeout=%s", got)
	}
	if got := structuredAttemptTimeout[FrameJudgment](); got != 2*time.Minute {
		t.Fatalf("atomic judgment timeout=%s", got)
	}
}
