from __future__ import annotations

import asyncio
import os
import re

from agentfield import AgentRouter

from . import helpers
from .models import (
    CritiqueBundle,
    FidelityAudit,
    NarrativeReview,
    PersonaReview,
    SlopReport,
)

router = AgentRouter(prefix="critique", tags=["critique"])

PAPER_CAP = 90_000
CONTEXT_CAP = 30_000


# Persona definitions: (file slug, detailed persona brief). PERSONAS exposes only the
# briefs, per the module contract; the slug is used for deterministic report filenames.
_PERSONA_DEFS: list[tuple[str, str]] = [
    (
        "domain-expert",
        "You are an exact-subfield domain expert reviewing a manuscript for a serious venue in "
        "your own specialty. You have published the closest related work and you know the state "
        "of the art cold. Judge the science, not the wording: is the central result genuinely "
        "novel relative to what already exists, or is it a known result in new clothing? Is the "
        "technical depth real, or does the paper stop at the surface where the hard problem "
        "begins? Does the main claim actually matter to the field, or is it a marginal delta on a "
        "solved problem? You are unimpressed by breadth and moved only by a specific, meaningful "
        "advance. Flag places where the contribution is overstated relative to prior art, where a "
        "key technical step is asserted rather than established, and where the result, even if "
        "true, would not change how anyone in the subfield works.",
    ),
    (
        "methods-reviewer",
        "You are a skeptical methods and statistics reviewer whose job is to find the ways this "
        "paper's evidence fails to support its conclusions. Hunt for overclaiming: conclusions "
        "stated with more certainty than the data allows, causal language on correlational "
        "evidence, and generalization beyond the tested regime. Check the baselines: are they "
        "the strong, current ones a critic would demand, or weak strawmen? Check for missing "
        "ablations that would isolate which component actually drives the reported gain. Check "
        "for missing error bars, confidence intervals, seeds, and variance reporting: is a "
        "difference claimed without any evidence it is not noise? Check whether the evaluation "
        "protocol (splits, metrics, tuning) could leak or flatter the proposed method. Every "
        "quantitative claim that lacks the statistical support a careful reviewer would require "
        "is an issue.",
    ),
    (
        "venue-editor",
        "You are an editor at a high-impact venue deciding, on the first page alone, whether this "
        "paper earns a reader's continued attention. Judge engagement and framing. Does the title "
        "pull, or is it generic keyword soup that could sit on a hundred papers? Does the abstract "
        "deliver a complete arc (problem, gap, approach, main quantitative result, implication) or "
        "does it trail off into vague promises? Do the opening paragraphs make the stakes concrete "
        "and specific in the first few sentences, or do they warm up slowly with background nobody "
        "needs? A reader decides fast; anything that makes the first page forgettable, hedged, or "
        "slow is a problem. Flag weak titles, abstracts that hide the result, and openings that "
        "bury the lede or fail to make the reader care.",
    ),
    (
        "careful-reader",
        "You are a careful, literal reader tracking the paper's internal mechanics as you go. Track "
        "every symbol and piece of notation: is each equation, variable, and term defined before "
        "it is used, or does the paper reference notation the reader has not met yet? Track figures "
        "and tables: is each one placed near, and no earlier than, its first textual reference, and "
        "is every figure actually referenced in the prose? Track section-to-section comprehension: "
        "can you follow the argument from one section into the next without a gap, a forward "
        "reference to something not yet established, or an unexplained leap? You are the reader who "
        "gets stuck on the concrete confusion everyone else glosses over. Flag undefined-before-use "
        "notation, misplaced or unreferenced figures, and any point where the reading breaks down.",
    ),
]

PERSONAS: list[str] = [brief for _, brief in _PERSONA_DEFS]

_REVIEW_DIRECTIVE = (
    "Review the FULL manuscript given below. Do NOT rewrite or rephrase the paper; only "
    "diagnose. Produce LOCATED issues: for each issue set `section` to the section file slug "
    "the problem lives in (for example 'results' or 'method'), or 'front_matter' for the "
    "title/abstract, or 'global' for a paper-wide problem. For each issue give a concrete "
    "`issue` (the specific problem, not a vague concern), an actionable `fix_hint` (what to "
    "change), and `severity` = 'major' if it would block acceptance, else 'minor'. Then give "
    "`acceptance_risk` in [0,1] (probability this paper is rejected as-is) and a short "
    "`verdict` paragraph. Set `confident` truthfully."
)


def _persona_slug(persona: str) -> str:
    for slug, brief in _PERSONA_DEFS:
        if brief == persona:
            return slug
    words = re.findall(r"[a-z0-9]+", persona.lower())[:5]
    return "-".join(words) or "persona"


def _parse_bib_keys(paper_dir: str) -> list[str]:
    text = helpers.read_text(os.path.join(paper_dir, "refs.bib"))
    if not text:
        return []
    keys = re.findall(r"@\w+\s*\{\s*([^,\s]+)\s*,", text)
    seen: list[str] = []
    for k in keys:
        if k not in seen:
            seen.append(k)
    return seen


def _invented_cite_keys(paper: str, bib_keys: list[str]) -> list[str]:
    """Deterministically find \\cite keys used in the paper but absent from refs.bib."""
    known = set(bib_keys)
    used: list[str] = []
    for group in re.findall(r"\\[Cc]ite[tp]?\*?(?:\[[^\]]*\])?(?:\[[^\]]*\])?\{([^}]+)\}", paper):
        for key in group.split(","):
            key = key.strip()
            if key and key not in used:
                used.append(key)
    return [k for k in used if k not in known]


@router.reasoner()
async def persona_review(
    workspace: dict, persona: str, round_no: int, model: str | None = None
) -> PersonaReview:
    """One .ai review of the full paper from a single reviewer persona."""
    paper_dir = workspace["paper_dir"]
    paper = helpers.assemble_paper_text(paper_dir)[:PAPER_CAP]
    print(f"[critique] persona_review round={round_no} persona={_persona_slug(persona)}")
    try:
        result = await router.ai(
            system=persona + "\n\n" + _REVIEW_DIRECTIVE,
            user=(
                f"Reviewing round {round_no}. Here is the complete assembled manuscript "
                f"(main.tex + all sections, with '--- <file> ---' markers):\n\n{paper}\n\n"
                "Return your located issues, acceptance_risk, verdict, and confidence."
            ),
            schema=PersonaReview,
            model=helpers.ai_model(model),
        )
        result.persona = persona
        return result
    except Exception as err:  # noqa: BLE001 — degrade, never crash the run.
        print(f"[critique] persona_review crashed persona={_persona_slug(persona)}: {err}")
        return helpers.safe_ai_fallback(
            PersonaReview,
            persona=persona,
            verdict=f"persona review crashed: {err}",
            acceptance_risk=0.5,
        )


@router.reasoner()
async def narrative_review(
    workspace: dict, round_no: int, model: str | None = None
) -> NarrativeReview:
    """One .ai review of the paper's transitions, promise alignment, and story arc."""
    paper_dir = workspace["paper_dir"]
    paper = helpers.assemble_paper_text(paper_dir)[:PAPER_CAP]
    positioning = helpers.read_text(workspace["positioning_path"], limit=CONTEXT_CAP)
    blueprint = helpers.read_text(workspace["blueprint_path"], limit=CONTEXT_CAP)
    print(f"[critique] narrative_review round={round_no}")
    try:
        result = await router.ai(
            system=(
                "You are a senior scientific editor judging whether one continuous story governs "
                "the whole paper. You check three things and nothing else. First, the transition "
                "contract: does each section open from what the previous section established, so "
                "the reader is carried forward rather than restarted? Report breaks as located "
                "transition_issues (section slug, the gap, a fix_hint, severity). Second, promise "
                "alignment: does the body deliver exactly what the title and abstract promise? "
                "List both under-delivery (a promise the body never pays off) and over-delivery "
                "(the body claims more than the front matter set up) in promise_alignment_issues. "
                "Third, the arc: does the sequence of results build belief in the main claim, or "
                "does it wander? Summarize in arc_assessment and give a `score` in [0,1] for "
                "overall narrative coherence. Do not rewrite the paper. Set `confident` truthfully."
            ),
            user=(
                f"Round {round_no}.\n\nPOSITIONING.md (the chosen frame, title, abstract):\n"
                f"{positioning or '(POSITIONING.md unavailable)'}\n\n"
                f"BLUEPRINT.md (section beats and transition contract):\n"
                f"{blueprint or '(BLUEPRINT.md unavailable)'}\n\n"
                f"Complete assembled manuscript:\n\n{paper}\n\n"
                "Return transition_issues, promise_alignment_issues, arc_assessment, score, confident."
            ),
            schema=NarrativeReview,
            model=helpers.ai_model(model),
        )
        return result
    except Exception as err:  # noqa: BLE001
        print(f"[critique] narrative_review crashed: {err}")
        return helpers.safe_ai_fallback(
            NarrativeReview,
            arc_assessment=f"narrative review crashed: {err}",
            score=0.5,
        )


@router.reasoner()
async def fidelity_audit(
    workspace: dict, round_no: int, model: str | None = None
) -> FidelityAudit:
    """One .ai audit tracing every quantitative claim and citation to evidence. Fails CLOSED."""
    paper_dir = workspace["paper_dir"]
    paper = helpers.assemble_paper_text(paper_dir)[:PAPER_CAP]
    evidence = helpers.read_text(workspace["evidence_path"])  # FULL ledger, uncapped.
    bib_keys = _parse_bib_keys(paper_dir)
    print(f"[critique] fidelity_audit round={round_no} bib_keys={len(bib_keys)}")
    try:
        result = await router.ai(
            system=(
                "You are a fidelity auditor. The paper may state only what the evidence ledger "
                "supports. Enforce three rules with zero tolerance. First: every quantitative "
                "claim (any number, metric, delta, percentage, or comparison) must trace to a "
                "fact in EVIDENCE.md, or else sit inside a \\todobox{...}. A number that appears "
                "in neither is unsupported; report it in unsupported_claims. Second: when a number "
                "in the paper disagrees with the ledger value for the same quantity, report the "
                "pair in number_mismatches as 'claim says X, ledger says Y'. Third: every \\cite "
                "key must appear in the provided list of bib keys; report any missing or invented "
                "key in citation_issues. Set `blocking` = true if and only if there is a "
                "fabricated number, a fabricated citation, or a claim that goes materially beyond "
                "the evidence; small wording issues are not blocking. Give a `score` in [0,1] for "
                "overall factual fidelity. Do not rewrite the paper. Set `confident` truthfully."
            ),
            user=(
                f"Round {round_no}.\n\nEVIDENCE.md (the authoritative fact ledger):\n"
                f"{evidence or '(EVIDENCE.md unavailable — treat all numbers as unsupported)'}\n\n"
                f"Bib keys present in paper/refs.bib ({len(bib_keys)}): {bib_keys}\n\n"
                f"Complete assembled manuscript:\n\n{paper}\n\n"
                "Return unsupported_claims, number_mismatches, citation_issues, blocking, score, confident."
            ),
            schema=FidelityAudit,
            model=helpers.ai_model(model),
        )
        # Deterministic overlay: invented citation keys are mechanically checkable and
        # ALWAYS blocking, regardless of how lenient the model's judgment was.
        invented = _invented_cite_keys(paper, bib_keys)
        if invented:
            flagged = {c for c in result.citation_issues}
            for key in invented:
                finding = f"\\cite{{{key}}} is used in the paper but absent from refs.bib (deterministic check)"
                if not any(key in c for c in flagged):
                    result.citation_issues.append(finding)
            result.blocking = True
            result.score = min(result.score, 0.5)
            print(f"[critique] fidelity: {len(invented)} invented cite keys -> blocking=True")
        return result
    except Exception as err:  # noqa: BLE001 — fail CLOSED: an unauditable paper must not converge.
        print(f"[critique] fidelity_audit crashed (failing closed): {err}")
        return helpers.safe_ai_fallback(
            FidelityAudit,
            blocking=True,
            score=0.0,
            unsupported_claims=["fidelity audit crashed: " + str(err)],
        )


@router.reasoner()
async def run_critique(
    workspace: dict, round_no: int, model: str | None = None
) -> CritiqueBundle:
    """Orchestrator: all persona/narrative/fidelity reviewers in ONE gather + deterministic slop."""
    nid = helpers.node_id()
    paper_dir = workspace["paper_dir"]
    print(f"[critique] run_critique round={round_no} personas={len(PERSONAS)}")

    tasks = [
        router.call(
            f"{nid}.critique_persona_review",
            workspace=workspace,
            persona=persona,
            round_no=round_no,
            model=model,
        )
        for persona in PERSONAS
    ]
    tasks.append(
        router.call(
            f"{nid}.critique_narrative_review",
            workspace=workspace,
            round_no=round_no,
            model=model,
        )
    )
    tasks.append(
        router.call(
            f"{nid}.critique_fidelity_audit",
            workspace=workspace,
            round_no=round_no,
            model=model,
        )
    )

    results = await asyncio.gather(*tasks, return_exceptions=True)
    had_exception = False

    persona_reviews: list[PersonaReview] = []
    for persona, res in zip(PERSONAS, results[: len(PERSONAS)]):
        if isinstance(res, Exception):
            had_exception = True
            print(f"[critique] persona gather error {_persona_slug(persona)}: {res}")
            persona_reviews.append(
                helpers.safe_ai_fallback(
                    PersonaReview,
                    persona=persona,
                    verdict=f"persona review errored in gather: {res}",
                    acceptance_risk=0.5,
                )
            )
        else:
            pr = PersonaReview(**res)
            pr.persona = persona
            persona_reviews.append(pr)

    narr_res = results[len(PERSONAS)]
    if isinstance(narr_res, Exception):
        had_exception = True
        print(f"[critique] narrative gather error: {narr_res}")
        narrative = helpers.safe_ai_fallback(
            NarrativeReview,
            arc_assessment=f"narrative review errored in gather: {narr_res}",
            score=0.5,
        )
    else:
        narrative = NarrativeReview(**narr_res)

    fid_res = results[len(PERSONAS) + 1]
    if isinstance(fid_res, Exception):
        had_exception = True
        print(f"[critique] fidelity gather error (failing closed): {fid_res}")
        fidelity = helpers.safe_ai_fallback(
            FidelityAudit,
            blocking=True,
            score=0.0,
            unsupported_claims=["fidelity audit errored in gather: " + str(fid_res)],
        )
    else:
        fidelity = FidelityAudit(**fid_res)

    # Deterministic slop lint — direct helper call, NOT a reasoner, NOT in the gather.
    slop: SlopReport = helpers.slop_lint(paper_dir)

    confident = (
        (not had_exception)
        and all(p.confident for p in persona_reviews)
        and narrative.confident
        and fidelity.confident
    )

    bundle = CritiqueBundle(
        round=round_no,
        persona_reviews=persona_reviews,
        narrative=narrative,
        fidelity=fidelity,
        slop=slop,
        confident=confident,
    )

    _write_round_artifacts(workspace, round_no, persona_reviews, narrative, fidelity, slop, bundle)
    print(
        f"[critique] round={round_no} done confident={confident} "
        f"fidelity_blocking={fidelity.blocking} slop_score={slop.score:.3f} "
        f"mean_risk={sum(p.acceptance_risk for p in persona_reviews) / max(len(persona_reviews), 1):.3f}"
    )
    return bundle


# --- report rendering -------------------------------------------------------

def _md_cell(s: str) -> str:
    return str(s).replace("|", "\\|").replace("\n", " ").strip()


def _issue_table(issues) -> str:
    if not issues:
        return "_No issues reported._\n"
    rows = ["| Section | Severity | Issue | Fix hint |", "|---|---|---|---|"]
    for it in issues:
        rows.append(
            f"| {_md_cell(it.section)} | {_md_cell(it.severity)} | "
            f"{_md_cell(it.issue)} | {_md_cell(it.fix_hint)} |"
        )
    return "\n".join(rows) + "\n"


def _write_round_artifacts(
    workspace, round_no, persona_reviews, narrative, fidelity, slop, bundle
) -> None:
    round_dir = os.path.join(workspace["reviews_dir"], f"round_{round_no}")

    for persona, review in zip(PERSONAS, persona_reviews):
        slug = _persona_slug(persona)
        md = (
            f"# Persona review — {slug} (round {round_no})\n\n"
            f"**Acceptance risk:** {review.acceptance_risk:.3f}  \n"
            f"**Confident:** {review.confident}\n\n"
            f"## Verdict\n\n{review.verdict or '_none_'}\n\n"
            f"## Issues\n\n{_issue_table(review.issues)}\n"
        )
        helpers.write_text(os.path.join(round_dir, f"{slug}.md"), md)

    promise = "\n".join(f"- {p}" for p in narrative.promise_alignment_issues) or "_None._"
    narr_md = (
        f"# Narrative review (round {round_no})\n\n"
        f"**Score:** {narrative.score:.3f}  \n**Confident:** {narrative.confident}\n\n"
        f"## Arc assessment\n\n{narrative.arc_assessment or '_none_'}\n\n"
        f"## Promise alignment issues\n\n{promise}\n\n"
        f"## Transition issues\n\n{_issue_table(narrative.transition_issues)}\n"
    )
    helpers.write_text(os.path.join(round_dir, "narrative.md"), narr_md)

    def _bullets(items):
        return "\n".join(f"- {i}" for i in items) or "_None._"

    fid_md = (
        f"# Fidelity audit (round {round_no})\n\n"
        f"**Blocking:** {fidelity.blocking}  \n**Score:** {fidelity.score:.3f}  \n"
        f"**Confident:** {fidelity.confident}\n\n"
        f"## Unsupported claims\n\n{_bullets(fidelity.unsupported_claims)}\n\n"
        f"## Number mismatches\n\n{_bullets(fidelity.number_mismatches)}\n\n"
        f"## Citation issues\n\n{_bullets(fidelity.citation_issues)}\n"
    )
    helpers.write_text(os.path.join(round_dir, "fidelity.md"), fid_md)

    grouped: dict[str, list] = {}
    for v in slop.violations:
        grouped.setdefault(v.rule, []).append(v)
    parts = [f"# Slop lint (round {round_no})\n", f"**Score:** {slop.score:.3f}  \n",
             f"**Total violations:** {len(slop.violations)}\n"]
    for rule in sorted(grouped):
        parts.append(f"\n## {rule} ({len(grouped[rule])})\n")
        for v in grouped[rule]:
            parts.append(f"- `{v.file}:{v.line}` — {_md_cell(v.excerpt)}")
    helpers.write_text(os.path.join(round_dir, "slop.md"), "\n".join(parts) + "\n")

    helpers.write_text(
        os.path.join(round_dir, "bundle.json"), bundle.model_dump_json(indent=2)
    )
