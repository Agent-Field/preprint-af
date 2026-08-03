package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

type WriteSectionInput struct {
	Workspace   Workspace    `json:"workspace"`
	Section     SectionSpec  `json:"section"`
	PrevSection *SectionSpec `json:"prev_section"`
	NextSection *SectionSpec `json:"next_section"`
	Model       *string      `json:"model"`
}
type BuildFigureInput struct {
	Workspace Workspace  `json:"workspace"`
	Figure    FigureSpec `json:"figure"`
	Model     *string    `json:"model"`
}
type BuildBibliographyInput struct {
	Workspace     Workspace `json:"workspace"`
	CitationNeeds []string  `json:"citation_needs"`
	AllowWeb      bool      `json:"allow_web"`
	Model         *string   `json:"model"`
}
type RunBuildInput struct {
	Workspace Workspace `json:"workspace"`
	Blueprint Blueprint `json:"blueprint"`
	AllowWeb  bool      `json:"allow_web"`
	Model     *string   `json:"model"`
}

func pSection(v SectionSpec) prompts.SectionSpec {
	return prompts.SectionSpec{Index: v.Index, Slug: v.Slug, Heading: v.Heading, Beats: v.Beats, Establishes: v.Establishes, Requires: v.Requires, EvidenceIDs: v.EvidenceIDs, FigureSlugs: v.FigureSlugs, TargetWords: v.TargetWords}
}
func pFigure(v FigureSpec) prompts.FigureSpec {
	return prompts.FigureSpec{Index: v.Index, Slug: v.Slug, Purpose: v.Purpose, DataSources: v.DataSources, Buildable: v.Buildable, CaptionTakeaway: v.CaptionTakeaway}
}

func (s *Service) WriteSection(ctx context.Context, in WriteSectionInput) (any, error) {
	spec := pSection(in.Section)
	var prev, next *prompts.SectionSpec
	if in.PrevSection != nil {
		v := pSection(*in.PrevSection)
		prev = &v
	}
	if in.NextSection != nil {
		v := pSection(*in.NextSection)
		next = &v
	}
	prompt := prompts.SectionPrompt(promptWorkspace(in.Workspace), spec, prev, next)
	hr, err := harnessArtifact(ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	filename := fmt.Sprintf("%02d_%s.tex", in.Section.Index, in.Section.Slug)
	abs := filepath.Join(in.Workspace.SectionsDir, filename)
	rel := "paper/sections/" + filename
	name := "section:" + in.Section.Slug
	if err != nil || !FileExists(abs) || len(ReadText(abs, 0)) < 300 {
		reason := "section file missing or too small (<300 chars)"
		if err != nil {
			reason = err.Error()
		} else if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		return WorkerResult{Name: name, Status: "failed", Summary: reason, Files: []string{}}, nil
	}
	summary := "Wrote " + rel
	return WorkerResult{Name: name, Status: "done", Summary: summary, Files: []string{rel}}, nil
}

func (s *Service) BuildFigure(ctx context.Context, in BuildFigureInput) (any, error) {
	return s.buildFigureWithVisualQA(ctx, in)
}

func (s *Service) BuildBibliography(ctx context.Context, in BuildBibliographyInput) (any, error) {
	prompt := prompts.BibliographyPrompt(promptWorkspace(in.Workspace), in.CitationNeeds, in.AllowWeb)
	hr, err := harnessArtifact(ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	refs := filepath.Join(in.Workspace.PaperDir, "refs.bib")
	if err != nil || !FileExists(refs) {
		reason := "paper/refs.bib was not created"
		if err != nil {
			reason = err.Error()
		} else if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		_, _ = WriteText(refs, "% refs.bib — bibliography build failed; no entries were written.\n")
		return WorkerResult{Name: "bibliography", Status: "failed", Summary: reason, Files: []string{"paper/refs.bib"}}, nil
	}
	summary := "Bibliography assembled"
	return WorkerResult{Name: "bibliography", Status: "done", Summary: summary, Files: []string{"paper/refs.bib"}}, nil
}

func (s *Service) RunBuild(ctx context.Context, in RunBuildInput) (any, error) {
	sections := append([]SectionSpec(nil), in.Blueprint.Sections...)
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].Index < sections[j].Index })
	figures := in.Blueprint.Figures
	secResults := make([]WorkerResult, len(sections))
	figResults := make([]WorkerResult, len(figures))
	// Figures depend only on the evidence ledger and source assets, so overlap
	// them with bibliography construction. Sections wait for refs.bib because
	// their citation contract depends on the verified key set.
	figGroup, figCtx := errgroup.WithContext(ctx)
	for i, fig := range figures {
		i, fig := i, fig
		figGroup.Go(func() error {
			v, e := callInto[WorkerResult](figCtx, s, "build_build_figure", BuildFigureInput{Workspace: in.Workspace, Figure: fig, Model: in.Model})
			if e != nil {
				v = WorkerResult{Name: "figure:" + fig.Slug, Status: "failed", Summary: e.Error(), Files: []string{}}
			}
			figResults[i] = v
			return nil
		})
	}
	bib, err := callInto[WorkerResult](ctx, s, "build_build_bibliography", BuildBibliographyInput{Workspace: in.Workspace, CitationNeeds: in.Blueprint.CitationNeeds, AllowWeb: in.AllowWeb, Model: in.Model})
	if err != nil {
		bib = WorkerResult{Name: "bibliography", Status: "failed", Summary: err.Error(), Files: []string{}}
	}
	sectionGroup, sectionCtx := errgroup.WithContext(ctx)
	for i, sec := range sections {
		i, sec := i, sec
		var prev, next *SectionSpec
		if i > 0 {
			v := sections[i-1]
			prev = &v
		}
		if i+1 < len(sections) {
			v := sections[i+1]
			next = &v
		}
		sectionGroup.Go(func() error {
			v, e := callInto[WorkerResult](sectionCtx, s, "build_write_section", WriteSectionInput{Workspace: in.Workspace, Section: sec, PrevSection: prev, NextSection: next, Model: in.Model})
			if e != nil {
				v = WorkerResult{Name: "section:" + sec.Slug, Status: "failed", Summary: e.Error(), Files: []string{}}
			}
			secResults[i] = v
			return nil
		})
	}
	_ = sectionGroup.Wait()
	_ = figGroup.Wait()
	confident := bib.Status != "failed"
	for _, x := range secResults {
		confident = confident && x.Status == "done"
	}
	for _, x := range figResults {
		confident = confident && x.Status == "done"
	}
	GitSnapshot(in.Workspace.Root, "P3 build complete")
	return BuildReport{Sections: secResults, Figures: figResults, Bibliography: bib, Confident: confident}, nil
}
