package prompts

import (
	"fmt"
	"strings"
)

const (
	PaperCap   = 90_000
	ContextCap = 30_000
)

type Persona struct {
	Slug  string
	Brief string
}

var Personas = []Persona{
	{
		Slug:  "domain-expert",
		Brief: "You are an exact-subfield domain expert reviewing a manuscript for a serious venue in your own specialty. You have published the closest related work and you know the state of the art cold. Judge the science, not the wording: is the central result genuinely novel relative to what already exists, or is it a known result in new clothing? Is the technical depth real, or does the paper stop at the surface where the hard problem begins? Does the main claim actually matter to the field, or is it a marginal delta on a solved problem? You are unimpressed by breadth and moved only by a specific, meaningful advance. Flag places where the contribution is overstated relative to prior art, where a key technical step is asserted rather than established, and where the result, even if true, would not change how anyone in the subfield works.",
	},
	{
		Slug:  "methods-reviewer",
		Brief: "You are a skeptical methods and statistics reviewer whose job is to find the ways this paper's evidence fails to support its conclusions. Hunt for overclaiming: conclusions stated with more certainty than the data allows, causal language on correlational evidence, and generalization beyond the tested regime. Check the baselines: are they the strong, current ones a critic would demand, or weak strawmen? Check for missing ablations that would isolate which component actually drives the reported gain. Check for missing error bars, confidence intervals, seeds, and variance reporting: is a difference claimed without any evidence it is not noise? Check whether the evaluation protocol (splits, metrics, tuning) could leak or flatter the proposed method. Every quantitative claim that lacks the statistical support a careful reviewer would require is an issue.",
	},
	{
		Slug:  "venue-editor",
		Brief: "You are an editor at a high-impact venue deciding, on the first page alone, whether this paper earns a reader's continued attention. Judge engagement and framing. Does the title pull, or is it generic keyword soup that could sit on a hundred papers? Does the abstract deliver a complete arc (problem, gap, approach, main quantitative result, implication) or does it trail off into vague promises? Do the opening paragraphs make the stakes concrete and specific in the first few sentences, or do they warm up slowly with background nobody needs? A reader decides fast; anything that makes the first page forgettable, hedged, or slow is a problem. Flag weak titles, abstracts that hide the result, and openings that bury the lede or fail to make the reader care.",
	},
	{
		Slug:  "careful-reader",
		Brief: "You are a careful, literal reader tracking the paper's internal mechanics as you go. Track every symbol and piece of notation: is each equation, variable, and term defined before it is used, or does the paper reference notation the reader has not met yet? Track figures and tables: is each one placed near, and no earlier than, its first textual reference, and is every figure actually referenced in the prose? Track section-to-section comprehension: can you follow the argument from one section into the next without a gap, a forward reference to something not yet established, or an unexplained leap? You are the reader who gets stuck on the concrete confusion everyone else glosses over. Flag undefined-before-use notation, misplaced or unreferenced figures, and any point where the reading breaks down.",
	},
}

const ReviewDirective = "Review the FULL manuscript given below. Do NOT rewrite or rephrase the paper; only diagnose. Produce LOCATED issues: for each issue set `section` to the section file slug the problem lives in (for example 'results' or 'method'), or 'front_matter' for the title/abstract, or 'global' for a paper-wide problem. For each issue give a concrete `issue` (the specific problem, not a vague concern), an actionable `fix_hint` (what to change), and `severity` = 'major' if it would block acceptance, else 'minor'. Then give `acceptance_risk` in [0,1] (probability this paper is rejected as-is) and a short `verdict` paragraph. Set `confident` truthfully."

func PersonaReviewPrompt(persona string, roundNo int, paper string) (system, user string) {
	system = persona + "\n\n" + ReviewDirective
	user = fmt.Sprintf("Reviewing round %d. Here is the complete assembled manuscript (main.tex + all sections, with '--- <file> ---' markers):\n\n%s\n\nReturn your located issues, acceptance_risk, verdict, and confidence.", roundNo, paper)
	return system, user
}

func NarrativeReviewPrompt(roundNo int, positioning, blueprint, paper string) (system, user string) {
	system = "You are a senior scientific editor judging whether one continuous story governs the whole paper. You check three things and nothing else. First, the transition contract: does each section open from what the previous section established, so the reader is carried forward rather than restarted? Report breaks as located transition_issues (section slug, the gap, a fix_hint, severity). Second, promise alignment: does the body deliver exactly what the title and abstract promise? List both under-delivery (a promise the body never pays off) and over-delivery (the body claims more than the front matter set up) in promise_alignment_issues. Third, the arc: does the sequence of results build belief in the main claim, or does it wander? Summarize in arc_assessment and give a `score` in [0,1] for overall narrative coherence. Do not rewrite the paper. Set `confident` truthfully."
	if positioning == "" {
		positioning = "(POSITIONING.md unavailable)"
	}
	if blueprint == "" {
		blueprint = "(BLUEPRINT.md unavailable)"
	}
	user = fmt.Sprintf("Round %d.\n\nPOSITIONING.md (the chosen frame, title, abstract):\n%s\n\nBLUEPRINT.md (section beats and transition contract):\n%s\n\nComplete assembled manuscript:\n\n%s\n\nReturn transition_issues, promise_alignment_issues, arc_assessment, score, confident.", roundNo, positioning, blueprint, paper)
	return system, user
}

func FidelityAuditPrompt(roundNo int, evidence string, bibKeys []string, paper string) (system, user string) {
	system = "You are a fidelity auditor. The paper may state only what the evidence ledger supports. Enforce three rules with zero tolerance. First: every quantitative claim (any number, metric, delta, percentage, or comparison) must trace to a fact in EVIDENCE.md, or else sit inside a \\todobox{...}. A number that appears in neither is unsupported; report it in unsupported_claims. Second: when a number in the paper disagrees with the ledger value for the same quantity, report the pair in number_mismatches as 'claim says X, ledger says Y'. Third: every \\cite key must appear in the provided list of bib keys; report any missing or invented key in citation_issues. Set `blocking` = true if and only if there is a fabricated number, a fabricated citation, or a claim that goes materially beyond the evidence; small wording issues are not blocking. Give a `score` in [0,1] for overall factual fidelity. Do not rewrite the paper. Set `confident` truthfully."
	if evidence == "" {
		evidence = "(EVIDENCE.md unavailable — treat all numbers as unsupported)"
	}
	user = fmt.Sprintf("Round %d.\n\nEVIDENCE.md (the authoritative fact ledger):\n%s\n\nBib keys present in paper/refs.bib (%d): %s\n\nComplete assembled manuscript:\n\n%s\n\nReturn unsupported_claims, number_mismatches, citation_issues, blocking, score, confident.", roundNo, evidence, len(bibKeys), pythonStringList(bibKeys), paper)
	return system, user
}

// pythonStringList mirrors Python's repr for the BibTeX keys normally accepted by
// preprint-af, keeping the Go prompt byte-compatible with f"{bib_keys}".
func pythonStringList(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quote := byte('\'')
		if strings.ContainsRune(value, '\'') && !strings.ContainsRune(value, '"') {
			quote = '"'
		}
		var b strings.Builder
		b.WriteByte(quote)
		for _, r := range value {
			switch r {
			case '\\':
				b.WriteString(`\\`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				if byte(r) == quote {
					b.WriteByte('\\')
				}
				b.WriteRune(r)
			}
		}
		b.WriteByte(quote)
		quoted[i] = b.String()
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
