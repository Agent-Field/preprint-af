package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
)

type DesignBlueprintInput struct {
	Workspace   Workspace `json:"workspace"`
	TargetVenue *string   `json:"target_venue"`
	Model       *string   `json:"model"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func validateBlueprintContract(bp Blueprint) error {
	if len(bp.Sections) < 6 || len(bp.Sections) > 9 {
		return fmt.Errorf("blueprint produced %d sections (need 6-9)", len(bp.Sections))
	}
	return nil
}

func slugify(v string) string {
	v = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "_"), "_")
	if v == "" {
		return "section"
	}
	return v
}

func parseFrontMatter(md string) (string, string) {
	var title string
	lines := strings.Split(md, "\n")
	titleRE := regexp.MustCompile(`(?i)^#\s*Title:\s*(.+?)\s*$`)
	for _, line := range lines {
		if m := titleRE.FindStringSubmatch(line); m != nil {
			title = strings.TrimSpace(m[1])
			break
		}
	}
	if title == "" {
		for _, line := range lines {
			v := strings.TrimSpace(line)
			if strings.HasPrefix(v, "#") {
				title = strings.TrimSpace(strings.TrimLeft(v, "#"))
				title = strings.TrimSpace(strings.TrimPrefix(title, "Title:"))
				if title != "" {
					break
				}
			}
		}
	}
	collect := false
	var abs []string
	for _, line := range lines {
		if regexp.MustCompile(`(?i)^#{1,6}\s*Final abstract\s*$`).MatchString(strings.TrimSpace(line)) {
			collect = true
			continue
		}
		if collect && strings.HasPrefix(strings.TrimSpace(line), "#") {
			break
		}
		if collect {
			abs = append(abs, line)
		}
	}
	return title, strings.TrimSpace(strings.Join(abs, "\n"))
}

func (s *Service) DesignBlueprint(ctx context.Context, in DesignBlueprintInput) (any, error) {
	evidence := ReadText(in.Workspace.EvidencePath, 50000)
	positioning := ReadText(in.Workspace.PositioningPath, 50000)
	system := prompts.BlueprintSystemPrompt()
	user := prompts.BlueprintUserPrompt(evidence, positioning, in.Workspace.InputFiles, stringValue(in.TargetVenue))
	bp, err := aiInto[Blueprint](ctx, s, system, user, stringValue(in.Model))
	directContractErr := validateBlueprintContract(bp)
	if err != nil || directContractErr != nil {
		// Large nested blueprints exceed the reliable structured-output path of
		// some OpenRouter models. Some providers also report success with a
		// semantically empty object. Escalate only that failed call to the
		// existing file-backed harness, retaining the exact authored system/user
		// text.
		var hrErr error
		var hrResult any
		fallback, hr, fallbackErr := harnessInto[Blueprint](ctx, s, system+"\n\n"+user, stringValue(in.Model), in.Workspace.Root, in.Workspace.Root)
		if hr != nil && hr.IsError {
			hrErr = fmt.Errorf("%s", hr.ErrorMessage)
			hrResult = hr.FailureType
		}
		if fallbackErr != nil || hrErr != nil {
			return nil, fmt.Errorf("blueprint design failed; direct error: %v; direct contract: %v; harness error: %v %v %v", err, directContractErr, fallbackErr, hrErr, hrResult)
		}
		bp = fallback
	}
	if err := validateBlueprintContract(bp); err != nil {
		return nil, fmt.Errorf("blueprint fallback violated contract: %w", err)
	}
	seen := map[string]bool{}
	for i := range bp.Sections {
		bp.Sections[i].Index = i + 1
		base := slugify(firstNonempty(bp.Sections[i].Slug, bp.Sections[i].Heading))
		slug := base
		for n := 2; seen[slug]; n++ {
			slug = fmt.Sprintf("%s_%d", base, n)
		}
		seen[slug] = true
		bp.Sections[i].Slug = slug
		if bp.Sections[i].TargetWords == 0 {
			bp.Sections[i].TargetWords = 400
		}
	}
	writeBlueprint(in.Workspace.BlueprintPath, bp)
	title, abstract := parseFrontMatter(positioning)
	if title == "" {
		title = "Untitled Draft"
	}
	files := make([]string, len(bp.Sections))
	for i, x := range bp.Sections {
		files[i] = fmt.Sprintf("%02d_%s", x.Index, x.Slug)
	}
	_, err = WriteText(filepath.Join(in.Workspace.PaperDir, "main.tex"), MainTexSkeleton(title, abstract, files))
	if err != nil {
		return nil, err
	}
	return bp, nil
}

func writeBlueprint(path string, b Blueprint) {
	var out strings.Builder
	out.WriteString("# Blueprint\n\n## Sections\n\n| # | slug | heading | beats | establishes | requires | evidence | figures |\n|---|------|---------|-------|-------------|----------|----------|---------|\n")
	for _, s := range b.Sections {
		fmt.Fprintf(&out, "| %d | %s | %s | %s | %s | %s | %s | %s |\n", s.Index, cell(s.Slug), cell(s.Heading), cell(strings.Join(s.Beats, "; ")), cell(s.Establishes), cell(s.Requires), cell(strings.Join(s.EvidenceIDs, ", ")), cell(strings.Join(s.FigureSlugs, ", ")))
	}
	out.WriteString("\n## Figures\n\n| # | slug | purpose | data sources | buildable | caption takeaway |\n|---|------|---------|--------------|-----------|------------------|\n")
	for _, f := range b.Figures {
		buildable := "no (TODO)"
		if f.Buildable {
			buildable = "yes"
		}
		fmt.Fprintf(&out, "| %d | %s | %s | %s | %s | %s |\n", f.Index, cell(f.Slug), cell(f.Purpose), cell(strings.Join(f.DataSources, ", ")), buildable, cell(f.CaptionTakeaway))
	}
	out.WriteString("\n## Citation needs\n\n")
	out.WriteString(bullets(b.CitationNeeds, "- (none)"))
	out.WriteString("\n\n## Venue notes\n\n")
	out.WriteString(defaultText(b.VenueNotes, "(none)"))
	out.WriteByte('\n')
	_, _ = WriteText(path, out.String())
}
func cell(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "\n", " "), "|", "\\|"))
}
func firstNonempty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
