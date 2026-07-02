from __future__ import annotations

import asyncio
import os

from agentfield import AgentRouter

from . import helpers
from .models import (
    FrameJudgment,
    FrameSet,
    NoveltyScan,
    PositioningDecision,
    StoryFrame,
    Workspace,
)

router = AgentRouter(prefix="positioning", tags=["positioning"])

# Four senior reader personas used by the framing tournament. Each judges the
# concrete title + mini-abstract of a candidate frame from a distinct stance.
PERSONAS: list[str] = [
    "a domain expert who checks whether the central result is technically "
    "meaningful and correctly interpreted, not just impressive-sounding",
    "a skeptical methods reviewer hunting for overclaims, unsupported "
    "evaluation, and any gap between what the title promises and what the "
    "evidence actually shows",
    "an interdisciplinary editor judging whether the first page signals broad "
    "importance to readers outside the immediate subfield",
    "a busy area chair judging how easily this title and abstract can be placed "
    "in a crowded program and defended against competing submissions",
]

EVIDENCE_CAP = 60000
DIGEST_CAP = 8000
TOP_FRAMES = 4


def _frame_score(frame: StoryFrame) -> float:
    return 0.55 * frame.evidence_alignment + 0.45 * frame.impact_potential


# --------------------------------------------------------------------------- #
# 1. Frame generation
# --------------------------------------------------------------------------- #
@router.reasoner()
async def generate_frames(
    workspace: dict,
    target_venue: str | None = None,
    field_hint: str | None = None,
    model: str | None = None,
) -> FrameSet:
    ws = Workspace(**workspace)
    evidence = helpers.read_text(ws.evidence_path, limit=EVIDENCE_CAP)
    print(f"[positioning] generate_frames: {len(evidence)} chars of evidence, venue={target_venue!r}")

    system = (
        "You are a scientific positioning strategist deciding how to frame a paper "
        "for maximum HONEST impact. The same evidence can be told as several "
        "genuinely different stories, speed, memory, mechanism, reliability, "
        "benchmark, or methodological insight, and your job is to surface the "
        "distinct options a top scientist would actually weigh. Judges score "
        "concrete artifacts, not descriptions, so every frame you propose must "
        "carry a real candidate title and a real mini-abstract written in that "
        "frame. You never invent numbers, datasets, or results, and you refuse to "
        "propose a frame the evidence cannot support."
    )
    user = (
        f"Target venue: {target_venue or 'not specified'}\n"
        f"Field hint: {field_hint or 'not specified'}\n\n"
        "Full evidence ledger (EVIDENCE.md):\n"
        "-----------------------------------\n"
        f"{evidence}\n"
        "-----------------------------------\n\n"
        "Produce 5 to 6 GENUINELY different frames for this paper. Each frame must "
        "commit to a different `angle`, the ONE thing it makes primary, drawn from "
        "options such as speed, memory, mechanism, reliability, benchmark, or "
        "methodological insight, whichever the evidence honestly supports. Frames "
        "must differ in what they make primary, not merely in wording.\n\n"
        "For each frame supply:\n"
        "- name: a short internal label for the frame.\n"
        "- angle: the single dimension it foregrounds.\n"
        "- central_thesis: the one claim the whole paper would defend.\n"
        "- title: a CONCRETE candidate title written in this frame (not a "
        "description of one), reading like a senior human scientist wrote it.\n"
        "- mini_abstract: a CONCRETE abstract of about 120 words written fully in "
        "this frame, carrying problem, precise gap, approach, the main quantitative "
        "result stated with the REAL numbers from the evidence, and the implication.\n"
        "- contribution_order: the ordered list of contributions this frame leads with.\n"
        "- figure_emphasis: which figures or plots this frame would foreground.\n"
        "- why_it_could_win: the honest case for this framing.\n"
        "- risk_of_failure: the honest way this framing could collapse under review.\n"
        "- evidence_alignment and impact_potential: honest scores in [0,1].\n\n"
        "Hard rules: do NOT invent any number, dataset, metric, or result that is "
        "not already in the evidence ledger. Do NOT propose any frame the evidence "
        "cannot support, if there is no speed result, do not write a speed frame. "
        "Set `confident` true only if the evidence is rich enough to position the "
        "paper honestly, and put your reasoning about the spread of frames in "
        "`generation_rationale`."
    )
    try:
        result = await router.ai(
            system=system,
            user=user,
            schema=FrameSet,
            model=helpers.ai_model(model),
        )
        print(f"[positioning] generate_frames: {len(result.frames)} frames (confident={result.confident})")
        return result
    except Exception as exc:  # noqa: BLE001
        print(f"[positioning] generate_frames FAILED: {exc}")
        return helpers.safe_ai_fallback(FrameSet, generation_rationale=f"frame generation failed: {exc}")


# --------------------------------------------------------------------------- #
# 2. Per-frame judging
# --------------------------------------------------------------------------- #
@router.reasoner()
async def judge_frame(
    frame: dict,
    evidence_digest: str,
    persona: str,
    target_venue: str | None = None,
    model: str | None = None,
) -> FrameJudgment:
    frame_name = str(frame.get("name", "unnamed frame"))
    title = str(frame.get("title", ""))
    thesis = str(frame.get("central_thesis", ""))
    mini_abstract = str(frame.get("mini_abstract", ""))
    angle = str(frame.get("angle", ""))
    print(f"[positioning] judge_frame: {frame_name!r} by {persona[:40]!r}...")

    system = (
        "You are simulating ONE specific senior scientific reader judging a single "
        "candidate framing of a paper. You see the frame's concrete title, its "
        "central thesis, and its ~120-word mini-abstract, plus a digest of the "
        "paper's evidence. Score it honestly on four axes and raise concrete "
        "concerns. Naturalness matters as much as content: a real senior scientist "
        "writes plainly. Penalize colon-stacking (Title: Subtitle: Sub-subtitle), "
        "keyword packing, hype adjectives (novel, powerful, comprehensive, "
        "groundbreaking, seamless), generic 'novel framework' phrasing, and "
        "symmetric slogan rhythm that reads machine-generated."
    )
    user = (
        f"You are {persona}.\n"
        f"Target venue: {target_venue or 'not specified'}\n\n"
        f"Frame under review\n"
        f"------------------\n"
        f"name: {frame_name}\n"
        f"angle: {angle}\n"
        f"title: {title}\n"
        f"central_thesis: {thesis}\n"
        f"mini_abstract: {mini_abstract}\n\n"
        f"Evidence digest\n"
        f"---------------\n"
        f"{evidence_digest}\n\n"
        "Score each axis in [0,1] from your persona's stance:\n"
        "- comprehension: can you grasp the actual contribution from the title and "
        "abstract alone?\n"
        "- excitement: does this make you want to read the paper?\n"
        "- credibility: is every claim in the title and abstract backed by the "
        "evidence digest, with no overclaim?\n"
        "- naturalness: would a senior human scientist actually write this title and "
        "abstract, or does it read as generated?\n\n"
        "List CONCRETE concerns: quote the exact word, phrase, or claim that "
        "triggers each concern. No generic complaints. Set `confident` honestly."
    )
    try:
        result = await router.ai(
            system=system,
            user=user,
            schema=FrameJudgment,
            model=helpers.ai_model(model),
        )
        return result
    except Exception as exc:  # noqa: BLE001
        print(f"[positioning] judge_frame FAILED for {frame_name!r}: {exc}")
        return helpers.safe_ai_fallback(
            FrameJudgment,
            frame_name=frame_name,
            persona=persona,
        )


# --------------------------------------------------------------------------- #
# 3. Novelty / prior-art scan (web via OpenCode harness)
# --------------------------------------------------------------------------- #
@router.reasoner()
async def scan_novelty(
    workspace: dict,
    frame_set: dict,
    model: str | None = None,
) -> NoveltyScan:
    ws = Workspace(**workspace)
    frames = frame_set.get("frames", []) or []
    scan_path = os.path.join(ws.root, "POSITIONING_SCAN.md")

    rendered = "\n".join(
        f"- {str(f.get('title', '')).strip()}  ||  thesis: {str(f.get('central_thesis', '')).strip()}"
        for f in frames
    ) or "- (no frames provided)"
    print(f"[positioning] scan_novelty: web-scanning {len(frames)} frame(s) at {ws.root}")

    prompt = (
        "You are a scientific literature scout with web access. We are choosing how "
        "to position a paper and need to know what already exists nearby.\n\n"
        "Frames under consideration (candidate title || central thesis):\n"
        f"{rendered}\n\n"
        "Task:\n"
        "1. Use web search across arXiv, Semantic Scholar, and Google Scholar to "
        "find the CLOSEST real related work to these frame titles and theses.\n"
        f"2. Write a file named POSITIONING_SCAN.md at the root of this workspace "
        f"({ws.root}) containing:\n"
        "   - Closest real titles found, quoted VERBATIM, each with a working link "
        "(arXiv id, DOI, or URL).\n"
        "   - How each of those papers positions itself (its angle and main claim).\n"
        "   - Collision risks for EACH of our frames: which existing paper would a "
        "reviewer say we overlap with, and how badly.\n"
        "   - Open positioning angles that nearby work leaves unclaimed.\n"
        "3. NEVER fabricate a paper, author, title, or link. If a search returns "
        "nothing usable, or web access fails, say so explicitly in the file rather "
        "than inventing anything.\n\n"
        "Then return the structured NoveltyScan: closest_titles (verbatim), "
        "collision_risks, positioning_openings, and confident."
    )
    try:
        result = await router.harness(
            prompt,
            schema=NoveltyScan,
            provider="opencode",
            model=helpers.opencode_model(model),
            cwd=ws.root,
            project_dir=ws.root,
        )
    except Exception as exc:  # noqa: BLE001
        print(f"[positioning] scan_novelty harness raised: {exc}")
        return helpers.safe_ai_fallback(NoveltyScan)

    if result.is_error or result.parsed is None or not os.path.exists(scan_path):
        print(
            f"[positioning] scan_novelty degraded "
            f"(is_error={result.is_error}, parsed={result.parsed is not None}, "
            f"file_exists={os.path.exists(scan_path)})"
        )
        return helpers.safe_ai_fallback(NoveltyScan)

    print(f"[positioning] scan_novelty: wrote {scan_path}, {len(result.parsed.closest_titles)} titles found")
    return result.parsed


# --------------------------------------------------------------------------- #
# 4. Orchestrator
# --------------------------------------------------------------------------- #
@router.reasoner()
async def run_positioning(
    workspace: dict,
    target_venue: str | None = None,
    field_hint: str | None = None,
    allow_web: bool = True,
    model: str | None = None,
) -> PositioningDecision:
    ws = Workspace(**workspace)
    print(f"[positioning] run_positioning: venue={target_venue!r}, allow_web={allow_web}")

    # -- frames ------------------------------------------------------------- #
    frame_set = FrameSet(
        **await router.call(
            f"{helpers.node_id()}.positioning_generate_frames",
            workspace=workspace,
            target_venue=target_venue,
            field_hint=field_hint,
            model=model,
        )
    )
    top_frames = sorted(frame_set.frames, key=_frame_score, reverse=True)[:TOP_FRAMES]
    print(f"[positioning] selected top {len(top_frames)} of {len(frame_set.frames)} frames")

    evidence_digest = helpers.read_text(ws.evidence_path, limit=DIGEST_CAP)
    top_frame_set = FrameSet(
        frames=top_frames,
        generation_rationale=frame_set.generation_rationale,
        confident=frame_set.confident,
    )

    # -- concurrent judging + novelty scan ---------------------------------- #
    judge_tasks = [
        router.call(
            f"{helpers.node_id()}.positioning_judge_frame",
            frame=frame.model_dump(),
            evidence_digest=evidence_digest,
            persona=persona,
            target_venue=target_venue,
            model=model,
        )
        for frame in top_frames
        for persona in PERSONAS
    ]

    async def _fallback_novelty() -> dict:
        return helpers.safe_ai_fallback(NoveltyScan).model_dump()

    if allow_web:
        novelty_coro = router.call(
            f"{helpers.node_id()}.positioning_scan_novelty",
            workspace=workspace,
            frame_set=top_frame_set.model_dump(),
            model=model,
        )
    else:
        novelty_coro = _fallback_novelty()

    print(f"[positioning] running {len(judge_tasks)} judgments + novelty scan concurrently")
    gathered = await asyncio.gather(*judge_tasks, novelty_coro)
    judgment_dicts = list(gathered[:-1])
    judgments = [FrameJudgment(**d) for d in judgment_dicts]
    novelty = NoveltyScan(**gathered[-1])

    # -- meta-selection ----------------------------------------------------- #
    system = (
        "You are the meta-editor making the final positioning decision for this "
        "paper. You are given several candidate frames (each with a concrete title "
        "and mini-abstract), the judgments of four senior readers on each frame, "
        "and a novelty scan of nearby published work. Pick exactly ONE winning "
        "frame. Do NOT average the scores: a frame with high excitement but weak "
        "credibility must lose, and a frame that collides with existing work should "
        "be discounted. Then write the paper's real front matter, obeying the hard "
        "constraints below, in the plain voice of a senior scientist."
    )
    user = (
        f"Target venue: {target_venue or 'not specified'}\n"
        f"Field hint: {field_hint or 'not specified'}\n\n"
        f"Candidate frames:\n{[f.model_dump() for f in top_frames]}\n\n"
        f"Reader judgments (four personas per frame):\n{[j.model_dump() for j in judgments]}\n\n"
        f"Novelty scan of nearby work:\n{novelty.model_dump()}\n\n"
        "Decide and produce:\n"
        "- winning_frame_name: the `name` of the single frame that wins.\n"
        "- final_title: obey ALL of these hard rules, at most 14 words, at most one "
        "colon, no question mark, no keyword stuffing, and it must read like a "
        "senior human wrote it.\n"
        "- final_abstract: a FULL abstract of about 180 words (NOT the mini-"
        "abstract), following the arc problem, then the precise gap, then the "
        "approach, then the main quantitative result stated with the REAL numbers "
        "drawn from the frames and evidence, then the implication. Invent NO number "
        "that is not already present in the frames.\n"
        "- opening_thesis: the single sentence the introduction should open on.\n"
        "- contribution_order: the ordered list of contributions the paper will make.\n"
        "- rejected_alternatives: one entry per losing frame, each formatted as "
        "'<frame name>: <one-line reason it lost>'.\n"
        "- selection_rationale: why the winner beats the rest, grounded in "
        "credibility and novelty, not just excitement.\n"
        "- score in [0,1] and confident.\n"
    )
    try:
        decision = await router.ai(
            system=system,
            user=user,
            schema=PositioningDecision,
            model=helpers.ai_model(model),
        )
        print(f"[positioning] meta-selection winner: {decision.winning_frame_name!r}")
    except Exception as exc:  # noqa: BLE001
        print(f"[positioning] meta-selection FAILED, falling back to best-judged frame: {exc}")
        decision = _fallback_decision(top_frames, judgments)

    _write_positioning_md(ws, decision, top_frames, judgments, novelty)
    helpers.git_snapshot(ws.root, "P1 positioning decided")
    print(f"[positioning] wrote {ws.positioning_path}")
    return decision


# --------------------------------------------------------------------------- #
# Helpers (plain functions)
# --------------------------------------------------------------------------- #
def _mean_judgment(judgments: list[FrameJudgment]) -> float:
    if not judgments:
        return 0.0
    per = [
        (j.comprehension + j.excitement + j.credibility + j.naturalness) / 4.0
        for j in judgments
    ]
    return sum(per) / len(per)


def _fallback_decision(
    frames: list[StoryFrame],
    judgments: list[FrameJudgment],
) -> PositioningDecision:
    by_frame: dict[str, list[FrameJudgment]] = {}
    for j in judgments:
        by_frame.setdefault(j.frame_name, []).append(j)

    best_frame = None
    best_mean = -1.0
    for frame in frames:
        mean = _mean_judgment(by_frame.get(frame.name, []))
        if mean > best_mean:
            best_mean = mean
            best_frame = frame

    if best_frame is None:
        # No frames at all: fully degraded fallback.
        return helpers.safe_ai_fallback(
            PositioningDecision,
            winning_frame_name="",
            final_title="Untitled draft",
            final_abstract="",
            opening_thesis="",
            contribution_order=[],
            rejected_alternatives=[],
            selection_rationale="Meta-selection failed and no candidate frames were available.",
            score=0.0,
        )

    rejected = [f.name for f in frames if f.name != best_frame.name]
    return PositioningDecision(
        winning_frame_name=best_frame.name,
        final_title=best_frame.title,
        final_abstract=best_frame.mini_abstract,
        opening_thesis=best_frame.central_thesis,
        contribution_order=list(best_frame.contribution_order),
        rejected_alternatives=rejected,
        selection_rationale=(
            "Meta-selection call failed; fell back to the frame with the highest "
            f"mean reader judgment ({best_mean:.3f})."
        ),
        score=max(0.0, min(1.0, best_mean)),
        confident=False,
    )


def _judgments_table(
    frames: list[StoryFrame],
    judgments: list[FrameJudgment],
) -> str:
    header = (
        "| Frame | Persona | Comprehension | Excitement | Credibility | Naturalness |\n"
        "| --- | --- | --- | --- | --- | --- |\n"
    )
    order = [f.name for f in frames]

    def _key(j: FrameJudgment) -> tuple[int, str]:
        idx = order.index(j.frame_name) if j.frame_name in order else len(order)
        return (idx, j.persona)

    rows = []
    for j in sorted(judgments, key=_key):
        persona_short = j.persona.split(",")[0].strip()
        rows.append(
            f"| {j.frame_name} | {persona_short} | {j.comprehension:.2f} | "
            f"{j.excitement:.2f} | {j.credibility:.2f} | {j.naturalness:.2f} |"
        )
    if not rows:
        rows.append("| (none) | (none) | 0.00 | 0.00 | 0.00 | 0.00 |")
    return header + "\n".join(rows)


def _write_positioning_md(
    ws: Workspace,
    decision: PositioningDecision,
    frames: list[StoryFrame],
    judgments: list[FrameJudgment],
    novelty: NoveltyScan,
) -> None:
    contrib = "\n".join(
        f"{i}. {item}" for i, item in enumerate(decision.contribution_order, start=1)
    ) or "1. (none specified)"

    rejected = "\n".join(f"- {item}" for item in decision.rejected_alternatives) or "- (none)"

    closest = "\n".join(f"- {t}" for t in novelty.closest_titles) or "- (none found)"
    collisions = "\n".join(f"- {c}" for c in novelty.collision_risks) or "- (none identified)"
    openings = "\n".join(f"- {o}" for o in novelty.positioning_openings) or "- (none identified)"

    lines = [
        f"# Title: {decision.final_title}",
        "",
        "## Final abstract",
        "",
        decision.final_abstract,
        "",
        "## Opening thesis",
        "",
        decision.opening_thesis,
        "",
        "## Contribution order",
        "",
        contrib,
        "",
        f"## Winning frame: {decision.winning_frame_name}",
        "",
        decision.selection_rationale,
        "",
        "## Judgments",
        "",
        _judgments_table(frames, judgments),
        "",
        "## Rejected alternatives",
        "",
        rejected,
        "",
        "## Novelty scan",
        "",
        f"Confident: {novelty.confident}",
        "",
        "Closest related titles:",
        closest,
        "",
        "Collision risks:",
        collisions,
        "",
        "Open positioning angles:",
        openings,
        "",
    ]
    helpers.write_text(ws.positioning_path, "\n".join(lines))
