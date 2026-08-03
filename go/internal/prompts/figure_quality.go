package prompts

import (
	"strconv"
	"strings"
)

const FigureDesignSystem = `You are the figure editor for a top scientific journal. Your job is to choose the clearest evidence-grounded visual argument, not to decorate the paper. Prefer a vector mechanism illustration for a paper-defining conceptual figure, and conventional quantitative plots for measured results. Never propose values or relationships absent from the supplied evidence. Set confident truthfully.`

func FigureDesignPrompt(spec FigureSpec, evidence, assets string, hasPreview bool) string {
	preview := "No existing preview is attached."
	if hasPreview {
		preview = "An existing preview is attached. Diagnose it before proposing the final composition; preserve valid scientific content but explicitly eliminate overlaps, clipping, weak hierarchy, tiny type, and decorative clutter."
	}
	return fill(`Design ONE publication figure.

Slug: {0}
Purpose / single takeaway: {1}
Caption takeaway: {2}
Blueprint buildable flag: {3}
Listed data sources: {4}
Resolved source assets: {5}

Evidence ledger excerpt:
{6}

{7}

Choose the visual grammar that best serves the claim:
- hero/mechanism: a restrained left-to-right or top-to-bottom vector system illustration built with matplotlib patches; one focal path, minimal prose, no poster-like title inside the art;
- comparison/result: aligned dot, line, interval, bar, or small-multiple panels with honest scales and uncertainty when present;
- regime/structure: heatmap, phase map, decision boundary, or compact schematic only when the evidence supports it.

The PDF must remain readable at its final one-column or two-column size. Text is labels, not paragraphs. Color must be colorblind-safe and secondary to shape/position. Return FigureDesign with visual_kind, a concrete composition, evidence_use entries naming E<n> ids or real source assets, buildable, and confident. Buildable may be true for an evidence-grounded conceptual mechanism even without tabular data; it must be false when the requested quantitative claim lacks real values.`, spec.Slug, spec.Purpose, spec.CaptionTakeaway, boolString(spec.Buildable), strings.Join(spec.DataSources, ", "), assets, evidence, preview)
}

func FigureRenderPrompt(base string, design string, priorCritique string, attempt int) string {
	critique := "This is the first render."
	if strings.TrimSpace(priorCritique) != "" {
		critique = "A vision reviewer rejected the prior render. Fix every defect below without changing or inventing scientific content:\n" + priorCritique
	}
	return base + fill(`

## Binding visual design brief
{0}

## Render and inspection contract
- Attempt {1} of a bounded visual-quality loop.
- Inspect any existing target script and preview before editing; reuse its verified data computations when sound.
- For the paper-defining hero/mechanism figure, build a clean vector illustration with matplotlib patches or paths. Do not put a large title inside the figure, do not use paragraph-length callouts, and do not let text or boxes overlap.
- Use a consistent journal palette, 7--9 pt final-size typography, deliberate whitespace, short labels, and panel lettering only for real multi-panel figures.
- Save vector PDF as the LaTeX master and a 300 dpi PNG from the same figure for visual review. Do not rasterize text or vector geometry into the PDF.
- After saving, programmatically inspect the PNG dimensions and open/read the preview yourself if the harness supports image inspection. Iterate within this worker until the composition is clean.
- Secondary annotations are expendable. If a dashed comparison path, callout, subtitle, or evidence note cannot fit with clear whitespace at final size, omit it and leave that detail to the caption rather than crowding the core visual.

{2}
`, design, strconv.Itoa(attempt), critique)
}

const FigureReviewSystem = `You are an exacting independent visual reviewer for a top scientific journal. Judge the attached rendered figure at final publication scale. Scientific clarity and truthful encoding dominate aesthetics. Reject overlaps, clipping, unreadable labels, paragraph-like annotations, weak hierarchy, ambiguous encodings, gratuitous decoration, inconsistent typography, or a visual form that does not support the stated takeaway. Set confident truthfully.`

func FigureReviewPrompt(spec FigureSpec, design string, deterministicChecks []string) string {
	return fill(`Review the attached PNG preview of one scientific figure.

Purpose: {0}
Caption takeaway: {1}
Design brief: {2}
Deterministic file checks: {3}

Score it from 0 to 1 for journal readiness at actual column width. Put every concrete blocking defect in hard_failures. Set rebuild=true for any overlap, clipping, unreadable text, misleading encoding, unsupported visual claim, or score below 0.86. Judge the rendered artifact, not absent experiments: when the design explicitly discloses unavailable raw data and labels evidence-backed points as representative, do not demand fabricated uncertainty bands or mark the missing source file as a visual hard failure. The critique must be an executable revision brief for the plotting-script worker, not generic taste commentary.`, spec.Purpose, spec.CaptionTakeaway, design, strings.Join(deterministicChecks, "; "))
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
