package pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/Agent-Field/preprint-af/go/internal/prompts"
	"golang.org/x/sync/errgroup"
)

type PersonaReviewInput struct {
	Workspace Workspace `json:"workspace"`
	Persona   string    `json:"persona"`
	RoundNo   int       `json:"round_no"`
	Model     *string   `json:"model"`
}

type NarrativeReviewInput struct {
	Workspace Workspace `json:"workspace"`
	RoundNo   int       `json:"round_no"`
	Model     *string   `json:"model"`
}
type FidelityAuditInput = NarrativeReviewInput
type RunCritiqueInput = NarrativeReviewInput

func (s *Service) PersonaReview(ctx context.Context, in PersonaReviewInput) (any, error) {
	paper := truncate(AssemblePaperText(in.Workspace.PaperDir), prompts.PaperCap)
	system, user := prompts.PersonaReviewPrompt(in.Persona, in.RoundNo, paper)
	result, err := aiInto[PersonaReview](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		return PersonaReview{Persona: in.Persona, Issues: []LocatedIssue{}, AcceptanceRisk: .5, Verdict: "persona review crashed: " + err.Error()}, nil
	}
	result.Persona = in.Persona
	return result, nil
}

func (s *Service) NarrativeReview(ctx context.Context, in NarrativeReviewInput) (any, error) {
	paper := truncate(AssemblePaperText(in.Workspace.PaperDir), prompts.PaperCap)
	positioning := ReadText(in.Workspace.PositioningPath, prompts.ContextCap)
	blueprint := ReadText(in.Workspace.BlueprintPath, prompts.ContextCap)
	system, user := prompts.NarrativeReviewPrompt(in.RoundNo, positioning, blueprint, paper)
	result, err := aiInto[NarrativeReview](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		return NarrativeReview{TransitionIssues: []LocatedIssue{}, PromiseAlignmentIssues: []string{}, ArcAssessment: "narrative review crashed: " + err.Error(), Score: .5}, nil
	}
	return result, nil
}

var bibKeyRE = regexp.MustCompile(`@\w+\s*\{\s*([^,\s]+)\s*,`)
var citeRE = regexp.MustCompile(`\\[Cc]ite[tp]?\*?(?:\[[^\]]*\])?(?:\[[^\]]*\])?\{([^}]+)\}`)

func parseBibKeys(paperDir string) []string {
	text := ReadText(filepath.Join(paperDir, "refs.bib"), 0)
	seen := map[string]bool{}
	var out []string
	for _, m := range bibKeyRE.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func inventedCiteKeys(paper string, bib []string) []string {
	known := map[string]bool{}
	for _, k := range bib {
		known[k] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range citeRE.FindAllStringSubmatch(paper, -1) {
		for _, raw := range strings.Split(m[1], ",") {
			k := strings.TrimSpace(raw)
			if k != "" && !known[k] && !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}

func (s *Service) FidelityAudit(ctx context.Context, in FidelityAuditInput) (any, error) {
	paper := truncate(AssemblePaperText(in.Workspace.PaperDir), prompts.PaperCap)
	evidence := ReadText(in.Workspace.EvidencePath, 0)
	bib := parseBibKeys(in.Workspace.PaperDir)
	system, user := prompts.FidelityAuditPrompt(in.RoundNo, evidence, bib, paper)
	result, err := aiInto[FidelityAudit](ctx, s, system, user, stringValue(in.Model))
	if err != nil {
		return FidelityAudit{UnsupportedClaims: []string{"fidelity audit crashed: " + err.Error()}, NumberMismatches: []string{}, CitationIssues: []string{}, Blocking: true, Score: 0}, nil
	}
	for _, key := range inventedCiteKeys(paper, bib) {
		found := false
		for _, issue := range result.CitationIssues {
			if strings.Contains(issue, key) {
				found = true
				break
			}
		}
		if !found {
			result.CitationIssues = append(result.CitationIssues, fmt.Sprintf("\\cite{%s} is used in the paper but absent from refs.bib (deterministic check)", key))
		}
		result.Blocking = true
		if result.Score > .5 {
			result.Score = .5
		}
	}
	return result, nil
}

func (s *Service) RunCritique(ctx context.Context, in RunCritiqueInput) (any, error) {
	reviews := make([]PersonaReview, len(prompts.Personas))
	var narrative NarrativeReview
	var fidelity FidelityAudit
	var hadErr bool
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, persona := range prompts.Personas {
		i, persona := i, persona
		g.Go(func() error {
			v, err := callInto[PersonaReview](gctx, s, "critique_persona_review", PersonaReviewInput{Workspace: in.Workspace, Persona: persona.Brief, RoundNo: in.RoundNo, Model: in.Model})
			if err != nil {
				mu.Lock()
				hadErr = true
				mu.Unlock()
				v = PersonaReview{Persona: persona.Brief, Issues: []LocatedIssue{}, AcceptanceRisk: .5, Verdict: "persona review errored in gather: " + err.Error()}
			}
			v.Persona = persona.Brief
			reviews[i] = v
			return nil
		})
	}
	g.Go(func() error {
		v, err := callInto[NarrativeReview](gctx, s, "critique_narrative_review", in)
		if err != nil {
			mu.Lock()
			hadErr = true
			mu.Unlock()
			v = NarrativeReview{TransitionIssues: []LocatedIssue{}, PromiseAlignmentIssues: []string{}, Score: .5, ArcAssessment: "narrative review errored in gather: " + err.Error()}
		}
		narrative = v
		return nil
	})
	g.Go(func() error {
		v, err := callInto[FidelityAudit](gctx, s, "critique_fidelity_audit", in)
		if err != nil {
			mu.Lock()
			hadErr = true
			mu.Unlock()
			v = FidelityAudit{UnsupportedClaims: []string{"fidelity audit errored in gather: " + err.Error()}, NumberMismatches: []string{}, CitationIssues: []string{}, Blocking: true}
		}
		fidelity = v
		return nil
	})
	_ = g.Wait()
	slop := SlopLint(in.Workspace.PaperDir)
	confident := !hadErr && narrative.Confident && fidelity.Confident
	for _, r := range reviews {
		confident = confident && r.Confident
	}
	bundle := CritiqueBundle{Round: in.RoundNo, PersonaReviews: reviews, Narrative: narrative, Fidelity: fidelity, Slop: slop, Confident: confident}
	writeRoundArtifacts(in.Workspace, bundle)
	return bundle, nil
}

func writeRoundArtifacts(ws Workspace, b CritiqueBundle) {
	dir := filepath.Join(ws.ReviewsDir, fmt.Sprintf("round_%d", b.Round))
	for i, r := range b.PersonaReviews {
		slug := prompts.Personas[i].Slug
		body := fmt.Sprintf("# Persona review — %s (round %d)\n\n**Acceptance risk:** %.3f  \n**Confident:** %t\n\n## Verdict\n\n%s\n\n## Issues\n\n%s\n", slug, b.Round, r.AcceptanceRisk, r.Confident, defaultText(r.Verdict, "_none_"), issueTable(r.Issues))
		_, _ = WriteText(filepath.Join(dir, slug+".md"), body)
	}
	promise := bullets(b.Narrative.PromiseAlignmentIssues, "_None._")
	narr := fmt.Sprintf("# Narrative review (round %d)\n\n**Score:** %.3f  \n**Confident:** %t\n\n## Arc assessment\n\n%s\n\n## Promise alignment issues\n\n%s\n\n## Transition issues\n\n%s\n", b.Round, b.Narrative.Score, b.Narrative.Confident, defaultText(b.Narrative.ArcAssessment, "_none_"), promise, issueTable(b.Narrative.TransitionIssues))
	_, _ = WriteText(filepath.Join(dir, "narrative.md"), narr)
	fid := fmt.Sprintf("# Fidelity audit (round %d)\n\n**Blocking:** %t  \n**Score:** %.3f  \n**Confident:** %t\n\n## Unsupported claims\n\n%s\n\n## Number mismatches\n\n%s\n\n## Citation issues\n\n%s\n", b.Round, b.Fidelity.Blocking, b.Fidelity.Score, b.Fidelity.Confident, bullets(b.Fidelity.UnsupportedClaims, "_None._"), bullets(b.Fidelity.NumberMismatches, "_None._"), bullets(b.Fidelity.CitationIssues, "_None._"))
	_, _ = WriteText(filepath.Join(dir, "fidelity.md"), fid)
	var slop strings.Builder
	fmt.Fprintf(&slop, "# Slop lint (round %d)\n\n**Score:** %.3f  \n\n**Total violations:** %d\n", b.Round, b.Slop.Score, len(b.Slop.Violations))
	for _, v := range b.Slop.Violations {
		fmt.Fprintf(&slop, "\n- `%s:%d` [%s] — %s", v.File, v.Line, v.Rule, v.Excerpt)
	}
	slop.WriteByte('\n')
	_, _ = WriteText(filepath.Join(dir, "slop.md"), slop.String())
	_, _ = WriteText(filepath.Join(dir, "bundle.json"), prettyJSON(b))
}

func issueTable(issues []LocatedIssue) string {
	if len(issues) == 0 {
		return "_No issues reported._\n"
	}
	var b strings.Builder
	b.WriteString("| Section | Severity | Issue | Fix hint |\n|---|---|---|---|\n")
	for _, x := range issues {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", mdCell(x.Section), mdCell(x.Severity), mdCell(x.Issue), mdCell(x.FixHint))
	}
	return b.String()
}
func mdCell(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " "))
}
func bullets(v []string, empty string) string {
	if len(v) == 0 {
		return empty
	}
	var b strings.Builder
	for i, x := range v {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(x)
	}
	return b.String()
}
func defaultText(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
func truncate(v string, n int) string {
	if n > 0 && len(v) > n {
		return v[:n]
	}
	return v
}
func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
