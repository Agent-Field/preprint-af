package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

type WriteSectionInput struct {
	Workspace   Workspace      `json:"workspace"`
	Section     SectionSpec    `json:"section"`
	PrevSection map[string]any `json:"prev_section"`
	NextSection map[string]any `json:"next_section"`
	Model       *string        `json:"model"`
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

func (in *BuildBibliographyInput) UnmarshalJSON(data []byte) error {
	type plain BuildBibliographyInput
	seeded := plain{AllowWeb: true}
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*in = BuildBibliographyInput(seeded)
	return nil
}

func (in *RunBuildInput) UnmarshalJSON(data []byte) error {
	type plain RunBuildInput
	seeded := plain{AllowWeb: true}
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*in = RunBuildInput(seeded)
	return nil
}

func pSection(v SectionSpec) prompts.SectionSpec {
	return prompts.SectionSpec{Index: v.Index, Slug: v.Slug, Heading: v.Heading, Beats: v.Beats, Establishes: v.Establishes, Requires: v.Requires, EvidenceIDs: v.EvidenceIDs, FigureSlugs: v.FigureSlugs, TargetWords: v.TargetWords}
}
func pFigure(v FigureSpec) prompts.FigureSpec {
	return prompts.FigureSpec{Index: v.Index, Slug: v.Slug, Purpose: v.Purpose, DataSources: v.DataSources, Buildable: v.Buildable, CaptionTakeaway: v.CaptionTakeaway}
}

func sectionMap(v SectionSpec) map[string]any {
	return map[string]any{
		"index": v.Index, "slug": v.Slug, "heading": v.Heading, "beats": v.Beats,
		"establishes": v.Establishes, "requires": v.Requires, "evidence_ids": v.EvidenceIDs,
		"figure_slugs": v.FigureSlugs, "target_words": v.TargetWords,
	}
}

func optionalSection(raw map[string]any) (*prompts.SectionSpec, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	for _, name := range []string{"index", "slug", "heading", "beats", "establishes"} {
		if _, ok := raw[name]; !ok {
			return nil, fmt.Errorf("invalid neighboring section: %s is required", name)
		}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var section SectionSpec
	if err := json.Unmarshal(data, &section); err != nil {
		return nil, err
	}
	value := pSection(section)
	return &value, nil
}

func (s *Service) WriteSection(ctx context.Context, in WriteSectionInput) (any, error) {
	spec := pSection(in.Section)
	prev, err := optionalSection(in.PrevSection)
	if err != nil {
		return nil, err
	}
	next, err := optionalSection(in.NextSection)
	if err != nil {
		return nil, err
	}
	prompt := prompts.SectionPrompt(promptWorkspace(in.Workspace), spec, prev, next)
	out, hr, err := harnessInto[WorkerResult](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil {
		return nil, err
	}
	filename := fmt.Sprintf("%02d_%s.tex", in.Section.Index, in.Section.Slug)
	abs := filepath.Join(in.Workspace.SectionsDir, filename)
	rel := "paper/sections/" + filename
	name := "section:" + in.Section.Slug
	if !FileExists(abs) || textLen(ReadText(abs, 0)) < 300 {
		reason := "section file missing or too small (<300 chars)"
		if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		return WorkerResult{Name: name, Status: "failed", Summary: reason, Files: []string{}}, nil
	}
	summary := out.Summary
	if summary == "" {
		summary = "Wrote " + rel
	}
	return WorkerResult{Name: name, Status: "done", Summary: summary, Files: []string{rel}}, nil
}

func (s *Service) BuildFigure(ctx context.Context, in BuildFigureInput) (any, error) {
	f := in.Figure
	name := "figure:" + f.Slug
	if !f.Buildable {
		needs := "author data"
		if len(f.DataSources) > 0 {
			needs = strings.Join(f.DataSources, ", ")
		}
		brief := fmt.Sprintf("Figure %s: %s — needs %s", f.Slug, f.Purpose, needs)
		if err := AppendTODOs(in.Workspace.TODOPath, []string{brief}); err != nil {
			return nil, err
		}
		return WorkerResult{Name: name, Status: "todo", Summary: brief, Files: []string{}}, nil
	}
	prompt := prompts.FigurePrompt(promptWorkspace(in.Workspace), pFigure(f), FigurePython())
	_, hr, err := harnessInto[WorkerResult](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil {
		return nil, err
	}
	pdf := filepath.Join(in.Workspace.FiguresDir, f.Slug+".pdf")
	if !FileExists(pdf) {
		reason := "paper/figures/" + f.Slug + ".pdf was not produced"
		if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		return WorkerResult{Name: name, Status: "failed", Summary: reason, Files: []string{}}, nil
	}
	summary := f.CaptionTakeaway
	if summary == "" {
		summary = f.Purpose
	}
	return WorkerResult{Name: name, Status: "done", Summary: summary, Files: []string{"paper/figures/" + f.Slug + ".py", "paper/figures/" + f.Slug + ".pdf", "paper/figures/" + f.Slug + ".png"}}, nil
}

func (s *Service) BuildBibliography(ctx context.Context, in BuildBibliographyInput) (any, error) {
	prompt := prompts.BibliographyPrompt(promptWorkspace(in.Workspace), in.CitationNeeds, in.AllowWeb)
	out, hr, err := harnessInto[WorkerResult](ctx, s, prompt, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
	if err != nil {
		return nil, err
	}
	refs := filepath.Join(in.Workspace.PaperDir, "refs.bib")
	if !FileExists(refs) {
		reason := "paper/refs.bib was not created"
		if hr != nil && hr.IsError {
			reason = hr.ErrorMessage
		}
		if _, writeErr := WriteText(refs, "% refs.bib — bibliography build failed; no entries were written.\n"); writeErr != nil {
			return nil, writeErr
		}
		return WorkerResult{Name: "bibliography", Status: "failed", Summary: reason, Files: []string{"paper/refs.bib"}}, nil
	}
	summary := out.Summary
	if summary == "" {
		summary = "Bibliography assembled"
	}
	return WorkerResult{Name: "bibliography", Status: "done", Summary: summary, Files: []string{"paper/refs.bib"}}, nil
}

func (s *Service) RunBuild(ctx context.Context, in RunBuildInput) (any, error) {
	sections := append([]SectionSpec(nil), in.Blueprint.Sections...)
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].Index < sections[j].Index })
	figures := in.Blueprint.Figures
	bib, err := callInto[WorkerResult](ctx, s, "build_build_bibliography", BuildBibliographyInput{Workspace: in.Workspace, CitationNeeds: in.Blueprint.CitationNeeds, AllowWeb: in.AllowWeb, Model: in.Model})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		bib = WorkerResult{Name: "bibliography", Status: "failed", Summary: err.Error(), Files: []string{}}
	}
	secResults := make([]WorkerResult, len(sections))
	figResults := make([]WorkerResult, len(figures))
	g, gctx := errgroup.WithContext(ctx)
	for i, sec := range sections {
		i, sec := i, sec
		prev, next := map[string]any{}, map[string]any{}
		if i > 0 {
			prev = sectionMap(sections[i-1])
		}
		if i+1 < len(sections) {
			next = sectionMap(sections[i+1])
		}
		g.Go(func() error {
			v, e := callInto[WorkerResult](gctx, s, "build_write_section", WriteSectionInput{Workspace: in.Workspace, Section: sec, PrevSection: prev, NextSection: next, Model: in.Model})
			if e != nil {
				if gctx.Err() != nil {
					return gctx.Err()
				}
				v = WorkerResult{Name: "section:" + sec.Slug, Status: "failed", Summary: e.Error(), Files: []string{}}
			}
			secResults[i] = v
			return nil
		})
	}
	for i, fig := range figures {
		i, fig := i, fig
		g.Go(func() error {
			v, e := callInto[WorkerResult](gctx, s, "build_build_figure", BuildFigureInput{Workspace: in.Workspace, Figure: fig, Model: in.Model})
			if e != nil {
				if gctx.Err() != nil {
					return gctx.Err()
				}
				v = WorkerResult{Name: "figure:" + fig.Slug, Status: "failed", Summary: e.Error(), Files: []string{}}
			}
			figResults[i] = v
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	confident := bib.Status != "failed"
	for _, x := range secResults {
		confident = confident && x.Status == "done"
	}
	GitSnapshot(in.Workspace.Root, "P3 build complete")
	return BuildReport{Sections: secResults, Figures: figResults, Bibliography: bib, Confident: confident}, nil
}
