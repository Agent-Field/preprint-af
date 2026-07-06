from __future__ import annotations

import os
import re

from agentfield import AgentRouter

from . import helpers
from .models import Blueprint

router = AgentRouter(prefix="blueprint", tags=["blueprint"])

TEXT_CAP = 50_000


def _slugify(raw: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "_", (raw or "").strip().lower()).strip("_")
    return slug or "section"


def _parse_front_matter(positioning_md: str) -> tuple[str, str]:
    """Extract final title and abstract from POSITIONING.md.

    Contract (positioning.py): a line `# Title: <title>` and a `## Final abstract` section.
    Coded defensively: if markers are absent, fall back to the first heading for the title and
    an empty abstract.
    """
    title = ""
    abstract = ""
    lines = positioning_md.splitlines()

    for line in lines:
        m = re.match(r"^#\s*Title:\s*(.+?)\s*$", line, re.IGNORECASE)
        if m:
            title = m.group(1).strip()
            break

    if not title:
        for line in lines:
            s = line.strip()
            if s.startswith("#"):
                title = s.lstrip("#").strip()
                if title.lower().startswith("title:"):
                    title = title[len("title:"):].strip()
                if title:
                    break

    # Abstract: everything under the '## Final abstract' heading until the next heading.
    collecting = False
    collected: list[str] = []
    for line in lines:
        if re.match(r"^#{1,6}\s*Final abstract\s*$", line.strip(), re.IGNORECASE):
            collecting = True
            continue
        if collecting:
            if line.strip().startswith("#"):
                break
            collected.append(line)
    abstract = "\n".join(collected).strip()

    return title, abstract


def _blueprint_system() -> str:
    return (
        "You are the lead architect of a scientific paper. You design the section-by-section "
        "blueprint that writers will fill in. You do not write prose. You produce a precise, "
        "evidence-grounded plan: what each section must accomplish, in what order, and how the "
        "argument hands off from one section to the next.\n\n"
        "Hard requirements:\n"
        "- 6 to 9 sections, adapted to the evidence and venue (typical arc: introduction, related "
        "work, method, experimental setup, results, discussion, conclusion — but adapt honestly).\n"
        "- Sections are indexed from 1 in reading order. Each slug is a short lowercase snake_case "
        "file slug (e.g. 'intro', 'related_work', 'method', 'results').\n"
        "- Each section has ordered `beats` (the narrative points it must land) and `evidence_ids` "
        "(the EVIDENCE.md fact ids, like E3, E7, it is allowed to use — use only ids that exist).\n"
        "- The transition contract MUST be consistent: each section's `establishes` states what the "
        "reader believes after it; the NEXT section's `requires` must be a subset of the PREVIOUS "
        "section's `establishes`. Build one clean chain with no gaps and no forward references. The "
        "first section's `requires` is empty.\n"
        "- Figures: propose `FigureSpec`s. Mark `buildable=True` ONLY when the needed data files "
        "actually exist in the provided input file list, and set `data_sources` to those real "
        "input/ paths. If the data is not present, set `buildable=False` (it becomes a TODO brief) "
        "and leave data_sources empty or aspirational. Never invent an input path that is not in "
        "the list. Attach figure slugs to the sections that reference them via `figure_slugs`.\n"
        "- `citation_needs`: concrete works or claims that will need a citation.\n"
        "- `venue_notes`: how the target venue shapes structure, length, and emphasis.\n"
        "Ground everything in the provided EVIDENCE.md and POSITIONING.md. Do not invent results.\n\n"
        "Limitations placement: designate exactly ONE location for limitations — a short Limitations "
        "paragraph inside the discussion/conclusion section, or a clause in Methods. No other section "
        "may carry limitation prose. Assign provenance details (seeds, N, hardware) to the Methods "
        "section beats only.\n\n"
        "Intro funnel: the opening section's beats must form a funnel: accepted field truth -> "
        "sharpening tension -> precise gap -> this paper's answer."
    )


def _mode_block(mode: str) -> str:
    if mode == "position":
        return (
            "POSITION MODE (default): the evidence set is CLOSED. You may NOT plan figures with "
            "buildable=false in the main text and may not plan sections that depend on missing data. "
            "If a story beat lacks evidence, restructure the story around what exists, or convert "
            "the gap into one clause in the Limitations location. The paper must be complete and "
            "submission-ready from existing evidence alone."
        )
    # propose mode (or any other future mode)
    return (
        "PROPOSE MODE: buildable=false figures are allowed as TODOs, and sections may reference "
        "planned data that does not yet exist. Mark such figures clearly with buildable=False and "
        "use todobox placeholders in beats where data is pending."
    )


def _blueprint_user(evidence_md: str, positioning_md: str, input_files: list[str],
                    target_venue: str | None, mode: str = "position") -> str:
    listing = "\n".join(f"  - input/{p}" for p in input_files) or "  (no input files listed)"
    return (
        f"TARGET VENUE: {target_venue or 'not specified'}\n\n"
        f"=== MODE ===\n{_mode_block(mode)}\n\n"
        "=== EVIDENCE.md (fact ledger; use these fact ids and figure candidates) ===\n"
        f"{evidence_md or '(EVIDENCE.md is empty or missing)'}\n\n"
        "=== POSITIONING.md (the winning frame, final title/abstract, contribution order) ===\n"
        f"{positioning_md or '(POSITIONING.md is empty or missing)'}\n\n"
        "=== INPUT FILES AVAILABLE (a figure may set buildable=True ONLY if its data_sources are "
        "among these paths) ===\n"
        f"{listing}\n\n"
        "Design the blueprint now. The story must follow the winning positioning frame: the "
        "section order and beats must deliver the promise made by the final title and abstract, in "
        "the contribution order chosen in POSITIONING.md. Produce a consistent establishes/requires "
        "chain across all sections. Return a Blueprint with sections, figures, citation_needs, "
        "venue_notes, and confident."
    )


@router.reasoner()
async def design_blueprint(workspace: dict, target_venue: str | None = None,
                           mode: str = "position",
                           model: str | None = None) -> Blueprint:
    """P2: design the paper blueprint and write BLUEPRINT.md + paper/main.tex.

    mode="position" (default): evidence set is closed; no unbuildable figures in main text.
    mode="propose": unbuildable figures allowed as TODOs for future work.

    This is the only reasoner allowed to raise: a paper cannot proceed without a blueprint.
    """
    root = workspace["root"]
    evidence_path = workspace["evidence_path"]
    positioning_path = workspace["positioning_path"]
    blueprint_path = workspace["blueprint_path"]
    paper_dir = workspace["paper_dir"]
    input_files = workspace.get("input_files") or []

    evidence_md = helpers.read_text(evidence_path, limit=TEXT_CAP)
    positioning_md = helpers.read_text(positioning_path, limit=TEXT_CAP)

    print(f"[blueprint] designing blueprint (mode={mode} evidence={len(evidence_md)}c "
          f"positioning={len(positioning_md)}c input_files={len(input_files)})")

    try:
        blueprint: Blueprint = await router.ai(
            system=_blueprint_system(),
            user=_blueprint_user(evidence_md, positioning_md, input_files, target_venue, mode),
            schema=Blueprint,
            model=helpers.ai_model(model),
        )
    except Exception as exc:  # noqa: BLE001 — blueprint failure aborts the run
        print(f"[blueprint] FATAL: blueprint .ai call failed: {exc}")
        raise ValueError(f"Blueprint design failed; a paper cannot proceed without a blueprint: {exc}") from exc

    # In position mode, drop any FigureSpec the LLM returned with buildable=False.
    # Record them in venue_notes so they are not silently lost.
    if mode == "position":
        excluded = [f for f in blueprint.figures if not f.buildable]
        if excluded:
            slugs = ", ".join(f.slug for f in excluded)
            print(f"[blueprint] position mode: excluding {len(excluded)} unbuildable figure(s): {slugs}")
            note = f"excluded in position mode: {slugs}"
            blueprint.venue_notes = (blueprint.venue_notes or "") + f"\n\n<!-- {note} -->"
            blueprint.figures = [f for f in blueprint.figures if f.buildable]

    sections = list(blueprint.sections)
    if len(sections) < 4:
        raise ValueError(
            f"Blueprint produced only {len(sections)} sections (need >= 4); aborting run."
        )

    # Normalize + deduplicate slugs; keep index ordering stable.
    seen: set[str] = set()
    for i, spec in enumerate(sections, start=1):
        spec.index = i
        base = _slugify(spec.slug or spec.heading)
        slug = base
        n = 2
        while slug in seen:
            slug = f"{base}_{n}"
            n += 1
        seen.add(slug)
        spec.slug = slug
    blueprint.sections = sections

    _write_blueprint_md(blueprint_path, blueprint)

    title, abstract = _parse_front_matter(positioning_md)
    if not title:
        print("[blueprint] WARNING: no title parsed from POSITIONING.md; using placeholder")
        title = "Untitled Draft"

    section_files = [f"{spec.index:02d}_{spec.slug}" for spec in sections]
    main_tex = helpers.main_tex_skeleton(title, abstract, section_files)
    helpers.write_text(os.path.join(paper_dir, "main.tex"), main_tex)

    print(f"[blueprint] done sections={len(sections)} figures={len(blueprint.figures)} "
          f"buildable={sum(1 for f in blueprint.figures if f.buildable)} "
          f"citation_needs={len(blueprint.citation_needs)} title={title!r}")
    return blueprint


def _write_blueprint_md(path: str, blueprint: Blueprint) -> None:
    lines: list[str] = ["# Blueprint", ""]

    lines.append("## Sections")
    lines.append("")
    lines.append("| # | slug | heading | beats | establishes | requires | evidence | figures |")
    lines.append("|---|------|---------|-------|-------------|----------|----------|---------|")
    for s in blueprint.sections:
        beats = "; ".join(s.beats) if s.beats else ""
        ev = ", ".join(s.evidence_ids) if s.evidence_ids else ""
        figs = ", ".join(s.figure_slugs) if s.figure_slugs else ""
        cells = [str(s.index), s.slug, s.heading, beats, s.establishes, s.requires, ev, figs]
        lines.append("| " + " | ".join(_cell(c) for c in cells) + " |")
    lines.append("")

    lines.append("## Figures")
    lines.append("")
    lines.append("| # | slug | purpose | data sources | buildable | caption takeaway |")
    lines.append("|---|------|---------|--------------|-----------|------------------|")
    for f in blueprint.figures:
        srcs = ", ".join(f.data_sources) if f.data_sources else ""
        cells = [str(f.index), f.slug, f.purpose, srcs, "yes" if f.buildable else "no (TODO)", f.caption_takeaway]
        lines.append("| " + " | ".join(_cell(c) for c in cells) + " |")
    lines.append("")

    lines.append("## Citation needs")
    lines.append("")
    if blueprint.citation_needs:
        for c in blueprint.citation_needs:
            lines.append(f"- {c}")
    else:
        lines.append("- (none)")
    lines.append("")

    lines.append("## Venue notes")
    lines.append("")
    lines.append(blueprint.venue_notes or "(none)")
    lines.append("")

    helpers.write_text(path, "\n".join(lines))


def _cell(value: str) -> str:
    return (value or "").replace("\n", " ").replace("|", "\\|").strip()
