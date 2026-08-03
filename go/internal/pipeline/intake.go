package pipeline

import (
	"context"
	"fmt"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
)

type PrepareWorkspaceInput struct {
	FolderPath string  `json:"folder_path"`
	Model      *string `json:"model"`
}
type BuildEvidenceInput struct {
	Workspace Workspace `json:"workspace"`
	Model     *string   `json:"model"`
}

func (s *Service) PrepareWorkspace(_ context.Context, in PrepareWorkspaceInput) (any, error) {
	ws, err := CreateWorkspace(in.FolderPath)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *Service) BuildEvidenceLedger(ctx context.Context, in BuildEvidenceInput) (any, error) {
	prompt := prompts.EvidencePrompt(promptWorkspace(in.Workspace), FigurePython())
	summary, hr, err := harnessInto[EvidenceSummary](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil || hr == nil || hr.IsError || !FileExists(in.Workspace.EvidencePath) || len(ReadText(in.Workspace.EvidencePath, 0)) < 500 {
		reason := "EVIDENCE.md missing, too small, or unparsed"
		if err != nil {
			reason = err.Error()
		} else if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		fallback := NewEvidenceSummary()
		fallback.DraftAssessment = "Evidence intake failed: " + reason
		return fallback, nil
	}
	if len(summary.Gaps) > 0 {
		AppendTODOs(in.Workspace.TODOPath, summary.Gaps)
	}
	fmt.Printf("[intake] evidence ledger done facts=%d figure_candidates=%d gaps=%d\n", summary.FactCount, len(summary.FigureCandidates), len(summary.Gaps))
	return summary, nil
}
