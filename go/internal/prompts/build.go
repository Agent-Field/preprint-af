package prompts

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

func SectionPrompt(_ Workspace, spec SectionSpec, prev, next *SectionSpec) string {
	nn := fmt.Sprintf("%02d", spec.Index)
	relpath := "paper/sections/" + nn + "_" + spec.Slug + ".tex"
	beats := "  (none specified)"
	if len(spec.Beats) > 0 {
		lines := make([]string, len(spec.Beats))
		for i, beat := range spec.Beats {
			lines[i] = "  " + strconv.Itoa(i+1) + ". " + beat
		}
		beats = strings.Join(lines, "\n")
	}
	evidence := "(none pre-listed — use only facts you can trace to an E<n> id in EVIDENCE.md)"
	if len(spec.EvidenceIDs) > 0 {
		evidence = strings.Join(spec.EvidenceIDs, ", ")
	}
	lo := int(math.RoundToEven(float64(spec.TargetWords) * 0.7))
	hi := int(math.RoundToEven(float64(spec.TargetWords) * 1.3))

	opening := "This is the OPENING section of the paper. Open with the positioning opening thesis (the winning frame's `opening_thesis` recorded in POSITIONING.md). Do not assume any prior section; establish the paper's stance from the first sentence."
	if prev != nil {
		opening = "The PREVIOUS section already established: \"" + prev.Establishes + "\". This is context you inherit. Open by BUILDING ON it (advance the argument), never by re-explaining or restating it."
	}
	closing := "This is the FINAL section. Close the paper's argument; do not set up a further section and do not end with a summary of this section."
	if next != nil {
		closing = "Your closing must set up what the NEXT section requires: \"" + next.Requires + "\". End by handing that thread forward, without summarizing this section."
	}
	figures := "This section references no figures."
	if len(spec.FigureSlugs) > 0 {
		lines := make([]string, len(spec.FigureSlugs))
		for i, slug := range spec.FigureSlugs {
			lines[i] = "  - figures/" + slug + ".pdf via \\includegraphics, inside a figure environment, with \\label{fig:" + slug + "} and a caption that states the TAKEAWAY (not what the axes are)"
		}
		figures = "This section MUST reference these figures (they are built separately):\n" + strings.Join(lines, "\n")
	}
	requires := spec.Requires
	if requires == "" {
		requires = "(nothing — see opening instructions)"
	}
	return fill(`You are writing ONE section of a scientific paper. The workspace root is your working directory
and OpenCode reads AGENTS.md there — obey it in full. Your SINGLE writable file is:
    {0}
Write ONLY that file. Do not create, edit, or touch any other file in the workspace.

## Read first (do not write until you have)
Read these workspace files completely before writing a word:
  - AGENTS.md        (the binding style and fidelity contract)
  - EVIDENCE.md      (the fact ledger — every number you use comes from here, keyed E<n>)
  - POSITIONING.md   (the winning frame, final title/abstract, opening thesis)
  - BLUEPRINT.md     (the full section plan and transition contract)
Then read any file under input/ behind the evidence ids listed below, plus paper/refs.bib to see
which citation keys actually exist.

## The section specification (follow it exactly)
- Heading (verbatim): {1}
- Ordered beats you must land, IN THIS ORDER:
{2}
- Establishes (what the reader must believe AFTER this section): {3}
- Requires (what the previous section already established — build on it, never re-explain it): {4}
- Evidence ids you may draw on: {5}
- Target length: about {6} words (stay within {7}–{8} words).

## Transition contract
{9}
{10}

## Figures
{11}

## Content and fidelity rules (hard)
- The file must START with the line §\section{{1}}§ exactly, and nothing before it.
  No \documentclass, no \begin{document}, no preamble — this file is \input into main.tex.
- Every number, metric, or quantitative claim MUST trace to an EVIDENCE.md fact id. Put the
  §% E<n>§ comment on its OWN line immediately after the sentence that uses the fact. NEVER
  place §%§ mid-line with prose after it: LaTeX silently drops everything after §%§ on that
  line, destroying paper content. If a number is not backed by a fact id, do not write it.
- Where the material you need is missing, do NOT invent it: write §\todobox{...}§ stating what
  is needed and why.
- Cite ONLY keys that already exist in paper/refs.bib. If you need a citation that is absent, do
  not invent a key — write a §\todobox{...}§ note describing the citation that is needed.
- Write flowing, scholarly prose per AGENTS.md: no em dashes, no banned phrases, varied sentence
  rhythm, claims-first paragraphs, transitions that carry scientific content rather than signposts.

## Return value
Return a WorkerResult: name = "section:{12}", status = "done" if you wrote the section,
files = ["{0}"], summary = one line on what the section now establishes.

Do the work now: read, then write {0}.`, relpath, spec.Heading, beats, spec.Establishes, requires, evidence,
		strconv.Itoa(spec.TargetWords), strconv.Itoa(lo), strconv.Itoa(hi), opening, closing, figures, spec.Slug)
}

func FigurePrompt(workspace Workspace, spec FigureSpec, figurePython string) string {
	caption := spec.CaptionTakeaway
	if caption == "" {
		caption = spec.Purpose
	}
	data := "No data_sources were listed. Load only real values recorded in EVIDENCE.md, and annotate each with the fact id it came from."
	if len(spec.DataSources) > 0 {
		lines := make([]string, len(spec.DataSources))
		for i, source := range spec.DataSources {
			lines[i] = "  - " + source + "  (absolute path: " + pythonPathJoin(workspace.InputDir, source) + ")"
		}
		data = "Load these REAL data files (do not fabricate data):\n" + strings.Join(lines, "\n")
	}
	return fill(`You are producing ONE publication-quality figure for a scientific paper. The workspace root is
your working directory; OpenCode reads AGENTS.md there — obey it. Your ONLY writable files are:
    paper/figures/{0}.py
    paper/figures/{0}.pdf
    paper/figures/{0}.png
Do not create, edit, or touch anything else.

## Read first
Read EVIDENCE.md (the fact ledger) and AGENTS.md before writing code.

## The figure
- Slug: {0}
- Purpose / single takeaway it must show: {1}
- Caption takeaway: {2}

{3}

## Requirements
- Write §paper/figures/{0}.py§ that loads the real data files above and produces BOTH
  §paper/figures/{0}.pdf§ (vector) AND §paper/figures/{0}.png§.
- The figure must communicate exactly ONE clear takeaway (the purpose above), readable at column
  width. Publication-quality matplotlib only: NO seaborn, NO styles that need extra dependencies,
  use §plt.tight_layout()§, save the pdf as a true vector, no chartjunk.
- Every hardcoded numeric literal in the script must carry a comment naming the EVIDENCE.md fact
  id it comes from (§# E<n>§). Prefer computing values from the loaded data over hardcoding.
- RUN the script with this exact interpreter and iterate until it exits 0 and the pdf exists:
      {4} paper/figures/{0}.py
  Fix every error until the run is clean and §paper/figures/{0}.pdf§ is on disk.

## Return value
Return a WorkerResult: name = "figure:{0}", status = "done" once the pdf exists,
files = ["paper/figures/{0}.py", "paper/figures/{0}.pdf", "paper/figures/{0}.png"].

Do the work now: read EVIDENCE.md, write the script, run it, verify the pdf.`, spec.Slug, spec.Purpose, caption, data, figurePython)
}

func pythonPathJoin(base, child string) string {
	if filepath.IsAbs(child) {
		return child
	}
	return strings.TrimSuffix(base, string(filepath.Separator)) + string(filepath.Separator) + child
}

func BibliographyPrompt(_ Workspace, citationNeeds []string, allowWeb bool) string {
	needs := "  (no specific citation needs listed)"
	if len(citationNeeds) > 0 {
		lines := make([]string, len(citationNeeds))
		for i, need := range citationNeeds {
			lines[i] = "  - " + need
		}
		needs = strings.Join(lines, "\n")
	}
	web := fill(`## Step 2 — Resolve citation needs (web search DISABLED)
Add only entries you can derive from material already in input/. Do NOT search the web and do NOT
invent entries. Record every one of the needed citations below in TODO.md instead (they cannot be
resolved without web access):
{0}

ABSOLUTE RULE: zero fabricated references.`, needs)
	if allowWeb {
		web = fill(`## Step 2 — Resolve citation needs (web search allowed)
For each needed citation below, find the REAL paper via web search (arXiv, Semantic Scholar, or the
publisher's page), then add a correct BibTeX entry to paper/refs.bib with a clean key. Log EVERY
entry you add in paper/CITATIONS_LOG.md, one line each, in exactly this format:
    - <key>: <title> — <URL or DOI> (source: <where verified>)
Needed citations:
{0}

ABSOLUTE RULE: zero fabricated references. If you cannot verify an entry against a real page, do
NOT add it to refs.bib — record it in TODO.md instead (state what is needed and why).`, needs)
	}
	return fill(`You are assembling the bibliography for a scientific paper. The workspace root is your working
directory; OpenCode reads AGENTS.md there — obey it. Writable files:
    paper/refs.bib
    paper/CITATIONS_LOG.md
    TODO.md
Do not touch any other file.

## Step 1 — Consolidate what already exists
Search input/ for every bibliographic source: any §.bib§ files, and works cited in existing drafts
(.tex/.md/.txt reference lists and inline citations). Consolidate all of them into paper/refs.bib
with clean, consistent citation keys and no duplicates.

{0}

## Return value
Return a WorkerResult: name = "bibliography", status = "done", files = the files you wrote,
summary = counts (entries consolidated, entries added, needs deferred to TODO.md).

Do the work now.`, web)
}
