package pipeline

import (
	"context"
	"fmt"

	"github.com/Agent-Field/agentfield/sdk/go/harness"
	"github.com/Agent-Field/preprint-af/go/internal/prompts"
)

const maxLatexAttempts = 3

type CompilePaperInput struct {
	Workspace Workspace `json:"workspace"`
	Model     *string   `json:"model"`
}

func (s *Service) CompilePaper(ctx context.Context, in CompilePaperInput) (any, error) {
	last := ""
	for attempt := 1; attempt <= maxLatexAttempts; attempt++ {
		ok, pdf, excerpt := RunLatexmk(in.Workspace.PaperDir)
		if ok {
			GitSnapshot(in.Workspace.Root, fmt.Sprintf("P4 compile success (attempt %d)", attempt))
			return CompileReport{Success: true, Attempts: attempt, PDFPath: pdf}, nil
		}
		last = excerpt
		if attempt == maxLatexAttempts {
			break
		}
		if err := s.acquireHarness(ctx); err != nil {
			return nil, err
		}
		hr, err := s.App.Harness(ctx, prompts.LatexRepairPrompt(excerpt), nil, nil, harness.Options{Provider: "opencode", Model: OpenCodeModel(stringValue(in.Model)), Cwd: in.Workspace.PaperDir, ProjectDir: in.Workspace.Root})
		s.releaseHarness()
		if err != nil {
			fmt.Printf("[latex] repair pass errored: %v\n", err)
		} else if hr != nil && hr.IsError {
			fmt.Printf("[latex] repair pass errored: %s\n", hr.ErrorMessage)
		}
	}
	return CompileReport{Success: false, Attempts: maxLatexAttempts, ErrorExcerpt: last}, nil
}
