from __future__ import annotations

import asyncio
import os

from agentfield import AgentRouter

from . import helpers
from .models import Blueprint, BuildReport, FigureSpec, SectionSpec, WorkerResult

router = AgentRouter(prefix="build", tags=["build"])


# ---------------------------------------------------------------------------
# write_section
# ---------------------------------------------------------------------------

def _section_prompt(
    workspace: dict,
    spec: SectionSpec,
    prev: SectionSpec | None,
    nxt: SectionSpec | None,
) -> str:
    nn = f"{spec.index:02d}"
    relpath = f"paper/sections/{nn}_{spec.slug}.tex"
    beats = "\n".join(f"  {i + 1}. {b}" for i, b in enumerate(spec.beats)) or "  (none specified)"
    evidence = ", ".join(spec.evidence_ids) or "(none pre-listed — use only facts you can trace to an E<n> id in EVIDENCE.md)"
    lo = int(round(spec.target_words * 0.7))
    hi = int(round(spec.target_words * 1.3))

    if prev is None:
        opening_block = (
            "This is the OPENING section. Structure the opening as a funnel: "
            "(1) a truth the field already accepts, "
            "(2) the sharpening tension or unmet need, "
            "(3) the precise gap stated so the reader now wants exactly this paper's answer, "
            "(4) this paper's answer, using the positioning opening thesis from POSITIONING.md. "
            "Do not open with generic context; the first sentence must already carry the paper's stance."
        )
    else:
        opening_block = (
            f"The PREVIOUS section already established: \"{prev.establishes}\". This is context you "
            "inherit. Open by BUILDING ON it (advance the argument), never by re-explaining or "
            "restating it."
        )

    if nxt is not None:
        closing_block = (
            f"Your closing must set up what the NEXT section requires: \"{nxt.requires}\". "
            "The last sentence must raise, in scientific content, the question the next section answers. "
            "Never use 'next, we describe' or any procedural handoff; the thread must be carried "
            "forward through the substance of the argument, not a signpost."
        )
    else:
        closing_block = (
            "This is the FINAL section. Close the paper's argument; do not set up a further section "
            "and do not end with a summary of this section. "
            "Close with consequence, not summary: one paragraph on what this result makes possible "
            "and what the field should now test."
        )

    if spec.figure_slugs:
        fig_lines = "\n".join(
            f"  - figures/{s}.pdf via \\includegraphics, inside a figure environment, with "
            f"\\label{{fig:{s}}} and a caption that states the TAKEAWAY (not what the axes are)"
            for s in spec.figure_slugs
        )
        fig_block = "This section MUST reference these figures (they are built separately):\n" + fig_lines
    else:
        fig_block = "This section references no figures."

    return f"""\
You are writing ONE section of a scientific paper. The workspace root is your working directory
and OpenCode reads AGENTS.md there — obey it in full. Your SINGLE writable file is:
    {relpath}
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
- Heading (verbatim): {spec.heading}
- Ordered beats you must land, IN THIS ORDER:
{beats}
- Establishes (what the reader must believe AFTER this section): {spec.establishes}
- Requires (what the previous section already established — build on it, never re-explain it): {spec.requires or "(nothing — see opening instructions)"}
- Evidence ids you may draw on: {evidence}
- Target length: about {spec.target_words} words (stay within {lo}–{hi} words).

## Transition contract
{opening_block}
{closing_block}

## Figures
{fig_block}

## Content and fidelity rules (hard)
- The file must START with the line `\\section{{{spec.heading}}}` exactly, and nothing before it.
  No \\documentclass, no \\begin{{document}}, no preamble — this file is \\input into main.tex.
- Every number, metric, or quantitative claim MUST trace to an EVIDENCE.md fact id. Put the
  `% E<n>` comment on its OWN line immediately after the sentence that uses the fact. NEVER
  place `%` mid-line with prose after it: LaTeX silently drops everything after `%` on that
  line, destroying paper content. If a number is not backed by a fact id, do not write it.
- Where the material you need is missing, do NOT invent it: write `\\todobox{{...}}` stating what
  is needed and why.
- Cite ONLY keys that already exist in paper/refs.bib. If you need a citation that is absent, do
  not invent a key — write a `\\todobox{{...}}` note describing the citation that is needed.
- Write flowing, scholarly prose per AGENTS.md: no em dashes, no banned phrases, varied sentence
  rhythm, claims-first paragraphs, transitions that carry scientific content rather than signposts.
- Salience placement: provenance details (seeds, N, hardware, hyperparameters) belong ONLY in the
  Methods section, stated once. Limitations and scope boundaries belong ONLY in the designated
  Limitations location, stated once. Never interrupt the narrative to disclaim; write at the strength
  the evidence supports and state it plainly.
- Equations are narrated: say the idea in words first, then the equation as part of a punctuated
  sentence, then one sentence interpreting the term that matters. Never place two displayed equations
  without prose between them.
- Topic sentences carry claims: the first sentence of each paragraph must state the paragraph's claim
  or result, not setup. A reader skimming only first sentences must get this section's argument.
- Never open with "In this section" or any signpost.

## Return value
Return a WorkerResult: name = "section:{spec.slug}", status = "done" if you wrote the section,
files = ["{relpath}"], summary = one line on what the section now establishes.

Do the work now: read, then write {relpath}."""


@router.reasoner()
async def write_section(
    workspace: dict,
    section: dict,
    prev_section: dict | None = None,
    next_section: dict | None = None,
    model: str | None = None,
) -> WorkerResult:
    """P3: write one section .tex file via a single OpenCode harness pass.

    Neighbor specs arrive as {} (not None) over the control plane; falsy means "no neighbor".
    """
    spec = SectionSpec(**section)
    prev = SectionSpec(**prev_section) if prev_section else None
    nxt = SectionSpec(**next_section) if next_section else None

    root = workspace["root"]
    sections_dir = workspace["sections_dir"]
    nn = f"{spec.index:02d}"
    filename = f"{nn}_{spec.slug}.tex"
    abs_path = os.path.join(sections_dir, filename)
    relpath = f"paper/sections/{filename}"
    name = f"section:{spec.slug}"

    print(f"[build] writing section {nn} {spec.slug} -> {relpath}")
    result = await router.harness(
        _section_prompt(workspace, spec, prev, nxt),
        provider="opencode",
        model=helpers.opencode_model(model),
        cwd=root,
        project_dir=root,
        schema=WorkerResult,
    )

    # verify the on-disk file, never trust the transcript.
    on_disk = helpers.read_text(abs_path)
    if not os.path.exists(abs_path) or len(on_disk) < 300:
        reason = result.error_message if result.is_error else "section file missing or too small (<300 chars)"
        print(f"[build] section {spec.slug} FAILED: {reason}")
        return WorkerResult(name=name, status="failed", summary=reason)

    summary = ""
    if result.parsed is not None:
        summary = result.parsed.summary
    print(f"[build] section {spec.slug} done ({len(on_disk)} chars)")
    return WorkerResult(name=name, status="done", summary=summary or f"Wrote {relpath}", files=[relpath])


# ---------------------------------------------------------------------------
# build_figure
# ---------------------------------------------------------------------------

def _figure_prompt(workspace: dict, spec: FigureSpec) -> str:
    input_dir = workspace["input_dir"]
    py = helpers.figure_python()
    slug = spec.slug
    if spec.data_sources:
        src_lines = "\n".join(
            f"  - {ds}  (absolute path: {os.path.join(input_dir, ds)})" for ds in spec.data_sources
        )
        data_block = "Load these REAL data files (do not fabricate data):\n" + src_lines
    else:
        data_block = (
            "No data_sources were listed. Load only real values recorded in EVIDENCE.md, and annotate "
            "each with the fact id it came from."
        )

    return f"""\
You are producing ONE publication-quality figure for a scientific paper. The workspace root is
your working directory; OpenCode reads AGENTS.md there — obey it. Your ONLY writable files are:
    paper/figures/{slug}.py
    paper/figures/{slug}.pdf
    paper/figures/{slug}.png
Do not create, edit, or touch anything else.

## Read first
Read EVIDENCE.md (the fact ledger) and AGENTS.md before writing code.

## The figure
- Slug: {slug}
- Purpose / single takeaway it must show: {spec.purpose}
- Caption takeaway: {spec.caption_takeaway or spec.purpose}

{data_block}

## Requirements
- Write `paper/figures/{slug}.py` that loads the real data files above and produces BOTH
  `paper/figures/{slug}.pdf` (vector) AND `paper/figures/{slug}.png`.
- The figure must communicate exactly ONE clear takeaway (the purpose above), readable at column
  width. Publication-quality matplotlib only: NO seaborn, NO styles that need extra dependencies,
  use `plt.tight_layout()`, save the pdf as a true vector, no chartjunk.
- Every hardcoded numeric literal in the script must carry a comment naming the EVIDENCE.md fact
  id it comes from (`# E<n>`). Prefer computing values from the loaded data over hardcoding.
- RUN the script with this exact interpreter and iterate until it exits 0 and the pdf exists:
      {py} paper/figures/{slug}.py
  Fix every error until the run is clean and `paper/figures/{slug}.pdf` is on disk.

## Return value
Return a WorkerResult: name = "figure:{slug}", status = "done" once the pdf exists,
files = ["paper/figures/{slug}.py", "paper/figures/{slug}.pdf", "paper/figures/{slug}.png"].

Do the work now: read EVIDENCE.md, write the script, run it, verify the pdf."""


@router.reasoner()
async def build_figure(workspace: dict, figure: dict, model: str | None = None) -> WorkerResult:
    """P3: build one figure by writing and running a matplotlib script against real data."""
    spec = FigureSpec(**figure)
    slug = spec.slug
    name = f"figure:{slug}"
    root = workspace["root"]
    figures_dir = workspace["figures_dir"]
    todo_path = workspace["todo_path"]

    if not spec.buildable:
        needs = ", ".join(spec.data_sources) if spec.data_sources else "author data"
        brief = f"Figure {slug}: {spec.purpose} — needs {needs}"
        helpers.append_todos(todo_path, [brief])
        print(f"[build] figure {slug} not buildable -> TODO")
        return WorkerResult(name=name, status="todo", summary=brief)

    abs_pdf = os.path.join(figures_dir, f"{slug}.pdf")
    relpaths = [f"paper/figures/{slug}.py", f"paper/figures/{slug}.pdf", f"paper/figures/{slug}.png"]

    print(f"[build] building figure {slug}")
    result = await router.harness(
        _figure_prompt(workspace, spec),
        provider="opencode",
        model=helpers.opencode_model(model),
        cwd=root,
        project_dir=root,
        schema=WorkerResult,
    )

    # verify the pdf exists on disk.
    if not os.path.exists(abs_pdf):
        reason = result.error_message if result.is_error else f"paper/figures/{slug}.pdf was not produced"
        print(f"[build] figure {slug} FAILED: {reason}")
        return WorkerResult(name=name, status="failed", summary=reason)

    print(f"[build] figure {slug} done")
    return WorkerResult(name=name, status="done", summary=spec.caption_takeaway or spec.purpose, files=relpaths)


# ---------------------------------------------------------------------------
# build_bibliography
# ---------------------------------------------------------------------------

def _bibliography_prompt(workspace: dict, citation_needs: list[str], allow_web: bool) -> str:
    needs = "\n".join(f"  - {n}" for n in citation_needs) or "  (no specific citation needs listed)"
    if allow_web:
        web_block = f"""\
## Step 2 — Resolve citation needs (web search allowed)
For each needed citation below, find the REAL paper via web search (arXiv, Semantic Scholar, or the
publisher's page), then add a correct BibTeX entry to paper/refs.bib with a clean key. Log EVERY
entry you add in paper/CITATIONS_LOG.md, one line each, in exactly this format:
    - <key>: <title> — <URL or DOI> (source: <where verified>)
Needed citations:
{needs}

ABSOLUTE RULE: zero fabricated references. If you cannot verify an entry against a real page, do
NOT add it to refs.bib — record it in TODO.md instead (state what is needed and why)."""
    else:
        web_block = f"""\
## Step 2 — Resolve citation needs (web search DISABLED)
Add only entries you can derive from material already in input/. Do NOT search the web and do NOT
invent entries. Record every one of the needed citations below in TODO.md instead (they cannot be
resolved without web access):
{needs}

ABSOLUTE RULE: zero fabricated references."""

    return f"""\
You are assembling the bibliography for a scientific paper. The workspace root is your working
directory; OpenCode reads AGENTS.md there — obey it. Writable files:
    paper/refs.bib
    paper/CITATIONS_LOG.md
    TODO.md
Do not touch any other file.

## Step 1 — Consolidate what already exists
Search input/ for every bibliographic source: any `.bib` files, and works cited in existing drafts
(.tex/.md/.txt reference lists and inline citations). Consolidate all of them into paper/refs.bib
with clean, consistent citation keys and no duplicates.

{web_block}

## Return value
Return a WorkerResult: name = "bibliography", status = "done", files = the files you wrote,
summary = counts (entries consolidated, entries added, needs deferred to TODO.md).

Do the work now."""


@router.reasoner()
async def build_bibliography(
    workspace: dict,
    citation_needs: list[str],
    allow_web: bool = True,
    model: str | None = None,
) -> WorkerResult:
    """P3: consolidate and resolve the paper's references into paper/refs.bib."""
    name = "bibliography"
    root = workspace["root"]
    paper_dir = workspace["paper_dir"]
    refs_path = os.path.join(paper_dir, "refs.bib")

    print(f"[build] building bibliography (needs={len(citation_needs)}, allow_web={allow_web})")
    result = await router.harness(
        _bibliography_prompt(workspace, citation_needs, allow_web),
        provider="opencode",
        model=helpers.opencode_model(model),
        cwd=root,
        project_dir=root,
        schema=WorkerResult,
    )

    # verify refs.bib exists; create a stub if the agent failed to.
    if not os.path.exists(refs_path):
        reason = result.error_message if result.is_error else "paper/refs.bib was not created"
        helpers.write_text(refs_path, "% refs.bib — bibliography build failed; no entries were written.\n")
        print(f"[build] bibliography FAILED: {reason}")
        return WorkerResult(name=name, status="failed", summary=reason, files=["paper/refs.bib"])

    summary = ""
    if result.parsed is not None:
        summary = result.parsed.summary
    print("[build] bibliography done")
    return WorkerResult(name=name, status="done", summary=summary or "Bibliography assembled", files=["paper/refs.bib"])


# ---------------------------------------------------------------------------
# run_build (orchestrator)
# ---------------------------------------------------------------------------

def _to_result(raw: object, name: str) -> WorkerResult:
    if isinstance(raw, Exception):
        return WorkerResult(name=name, status="failed", summary=str(raw))
    return WorkerResult(**raw)  # router.call returns a plain dict


@router.reasoner()
async def run_build(
    workspace: dict,
    blueprint: dict,
    allow_web: bool = True,
    model: str | None = None,
) -> BuildReport:
    """P3 orchestrator: build all sections, figures, and the bibliography concurrently."""
    bp = Blueprint(**blueprint)
    section_specs = sorted(bp.sections, key=lambda s: s.index)
    figure_specs = list(bp.figures)
    nid = helpers.node_id()

    print(f"[build] run_build: {len(section_specs)} sections, {len(figure_specs)} figures, 1 bibliography")

    # Bibliography FIRST: section writers may cite only keys present in refs.bib,
    # so it must exist before any section is written.
    try:
        bib_raw = await router.call(
            f"{nid}.build_build_bibliography",
            workspace=workspace,
            citation_needs=bp.citation_needs,
            allow_web=allow_web,
            model=model,
        )
        bib_result = _to_result(bib_raw, "bibliography")
    except Exception as exc:  # noqa: BLE001 — a failed bib must not block the build
        bib_result = WorkerResult(name="bibliography", status="failed", summary=str(exc))
    print(f"[build] bibliography phase done: {bib_result.status}")

    tasks = []
    section_names: list[str] = []
    for i, sec in enumerate(section_specs):
        prev = section_specs[i - 1] if i > 0 else None
        nxt = section_specs[i + 1] if i < len(section_specs) - 1 else None
        section_names.append(f"section:{sec.slug}")
        tasks.append(
            router.call(
                f"{nid}.build_write_section",
                workspace=workspace,
                section=sec.model_dump(),
                # The control plane 422s on None for dict-typed params; {} means "no neighbor".
                prev_section=prev.model_dump() if prev else {},
                next_section=nxt.model_dump() if nxt else {},
                model=model,
            )
        )

    figure_names: list[str] = []
    for fig in figure_specs:
        figure_names.append(f"figure:{fig.slug}")
        tasks.append(
            router.call(
                f"{nid}.build_build_figure",
                workspace=workspace,
                figure=fig.model_dump(),
                model=model,
            )
        )

    # ONE gather for sections + figures — the OpenCode semaphore (10) throttles concurrency.
    raw = await asyncio.gather(*tasks, return_exceptions=True)

    n_sec = len(section_specs)
    n_fig = len(figure_specs)
    section_results = [_to_result(raw[i], section_names[i]) for i in range(n_sec)]
    figure_results = [_to_result(raw[n_sec + j], figure_names[j]) for j in range(n_fig)]

    confident = all(s.status == "done" for s in section_results) and bib_result.status != "failed"

    helpers.git_snapshot(workspace["root"], "P3 build complete")
    print(
        f"[build] run_build done: sections "
        f"{sum(1 for s in section_results if s.status == 'done')}/{n_sec} done, "
        f"figures {sum(1 for f in figure_results if f.status == 'done')}/{n_fig} done, "
        f"bib={bib_result.status}, confident={confident}"
    )
    return BuildReport(
        sections=section_results,
        figures=figure_results,
        bibliography=bib_result,
        confident=confident,
    )
