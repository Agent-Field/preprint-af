package pipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
	_, hr, err := harnessInto[EvidenceSummary](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	// Some tool-capable models occasionally answer the harness in prose without
	// writing its required artifact. Give that transport failure one bounded,
	// explicit verification retry; the original authored prompt remains the
	// primary attempt and the workflow still fails closed if the file is absent.
	if err == nil && hr != nil && !hr.IsError && (!FileExists(in.Workspace.EvidencePath) || len(ReadText(in.Workspace.EvidencePath, 0)) < 500) {
		retry := prompt + "\n\nVERIFICATION RETRY: Your previous attempt did not create a substantive EVIDENCE.md at the exact path above. Use your filesystem tools now, inspect input/, write that file with the required structure, verify it exists and exceeds 500 bytes, then return the EvidenceSummary. Do not merely describe the work."
		_, hr, err = harnessInto[EvidenceSummary](ctx, s, retry, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	}
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
	summary := parseEvidenceSummary(ReadText(in.Workspace.EvidencePath, 0))
	if len(summary.Gaps) > 0 {
		AppendTODOs(in.Workspace.TODOPath, summary.Gaps)
	}
	fmt.Printf("[intake] evidence ledger done facts=%d figure_candidates=%d gaps=%d\n", summary.FactCount, len(summary.FigureCandidates), len(summary.Gaps))
	return summary, nil
}

var evidenceStatementRE = regexp.MustCompile(`(?m)^- \*\*Statement:\*\*\s*(.+)$`)

func parseEvidenceSummary(body string) EvidenceSummary {
	summary := NewEvidenceSummary()
	summary.FactCount = len(evidenceHeadingRE.FindAllStringSubmatch(body, -1))
	summary.FigureCandidates = markdownBullets(markdownSection(body, "Figure candidates"))
	summary.Gaps = markdownBullets(markdownSection(body, "Gaps"))
	summary.ExistingCitations = markdownBullets(markdownSection(body, "Citation inventory"))
	summary.DraftAssessment = strings.TrimSpace(markdownSection(body, "Existing draft"))
	if statements := evidenceStatementRE.FindAllStringSubmatch(body, -1); len(statements) > 0 {
		summary.StrongestFactualThesis = strings.TrimSpace(statements[0][1])
	}
	summary.Confident = summary.FactCount > 0 && len(body) >= 500
	return summary
}

func markdownSection(body, heading string) string {
	marker := "## " + heading
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	section := body[start+len(marker):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return strings.TrimSpace(section)
}

func markdownBullets(section string) []string {
	out := []string{}
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		}
	}
	return out
}
