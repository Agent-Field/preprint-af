from __future__ import annotations

import copy
import os
from statistics import mean

from agentfield import AgentRouter

from . import convergence, helpers, repair
from .critique import _invented_cite_keys, _parse_bib_keys
from .models import (
    Blueprint,
    BuildReport,
    CompileReport,
    CritiqueBundle,
    EvidenceSummary,
    LedgerIssue,
    PairwiseVerdict,
    PositioningDecision,
    RepairPlan,
    RoundRecord,
    VerificationReport,
    Workspace,
    WriteRequest,
    WriteResult,
)

router = AgentRouter(prefix="", tags=["workflow"])


def _round_table(records: list[RoundRecord]) -> str:
    lines = [
        "| Round | Total | Persona | Narrative | Fidelity | Slop | Compile | Repairs | Stop |",
        "| --- | --- | --- | --- | --- | --- | --- | --- | --- |",
    ]
    for r in records:
        lines.append(
            f"| {r.round} | {r.total_score:.4f} | {r.persona_score:.4f} | "
            f"{r.narrative_score:.4f} | {r.fidelity_score:.4f} | {r.slop_score:.4f} | "
            f"{'yes' if r.compile_ok else 'no'} | {r.repairs_applied} | "
            f"{r.stop_reason if r.stop else '-'} |"
        )
    return "\n".join(lines)


def _md_cell(s: object) -> str:
    return str(s).replace("|", "\\|").replace("\n", " ").strip()


def _ledger_section(
    ledger: list[LedgerIssue] | None,
    records: list[RoundRecord],
    pairwise_verdicts: list[tuple[int, str]] | None,
) -> str:
    """Render the Ledger section: totals, open-issue table, resolution rate, pairwise verdicts."""
    ledger = ledger or []
    parts: list[str] = ["\n## Issue ledger\n"]

    def _count(status: str, severity: str) -> int:
        return sum(1 for i in ledger if i.status == status and i.severity == severity)

    opened = len(ledger)
    resolved = sum(1 for i in ledger if i.status == "resolved")
    open_n = sum(1 for i in ledger if i.status == "open")
    parts.append(
        f"Totals: opened {opened} (major {sum(1 for i in ledger if i.severity == 'major')}, "
        f"minor {sum(1 for i in ledger if i.severity == 'minor')}), resolved {resolved}, "
        f"open {open_n} (major {_count('open', 'major')}, minor {_count('open', 'minor')}).\n"
    )

    open_issues = [i for i in ledger if i.status == "open"]
    if open_issues:
        rows = ["\n| id | severity | target | description |", "|---|---|---|---|"]
        for i in open_issues:
            rows.append(
                f"| {_md_cell(i.id)} | {_md_cell(i.severity)} | {_md_cell(i.target)} | "
                f"{_md_cell(i.description)[:200]} |"
            )
        parts.append("\n".join(rows) + "\n")
    else:
        parts.append("\nNo open issues.\n")

    # Resolution rate per round: resolved-this-round / opened-this-round.
    rounds = sorted({i.round_opened for i in ledger} | {r.round for r in records})
    if rounds:
        rate_rows = ["\n| Round | Opened | Resolved (opened this round) |", "|---|---|---|"]
        for r in rounds:
            opened_r = sum(1 for i in ledger if i.round_opened == r)
            resolved_r = sum(
                1 for i in ledger if i.round_opened == r and i.status == "resolved"
            )
            rate_rows.append(f"| {r} | {opened_r} | {resolved_r} |")
        parts.append("\n".join(rate_rows) + "\n")

    pairwise_verdicts = pairwise_verdicts or []
    if pairwise_verdicts:
        pv_rows = ["\n## Pairwise verdicts per round\n", "| Round | Winner |", "|---|---|"]
        for r, w in pairwise_verdicts:
            pv_rows.append(f"| {r} | {_md_cell(w)} |")
        parts.append("\n".join(pv_rows) + "\n")

    return "".join(parts)


def _compose_review(
    bundle: CritiqueBundle | None,
    records: list[RoundRecord],
    stop_reason: str,
    todo_digest: str,
    mode: str = "position",
    polish_outcome: str = "skipped",
    polish_detail: str = "",
    ledger: list[LedgerIssue] | None = None,
    pairwise_verdicts: list[tuple[int, str]] | None = None,
    coverage_line: str = "",
) -> str:
    parts: list[str] = ["# REVIEW — unresolved problems\n"]
    parts.append(f"Mode: **{mode}**\n")
    parts.append(f"Stop reason: **{stop_reason}**\n")
    if coverage_line:
        parts.append(f"{coverage_line}\n")
    polish_line = f"Voice-unification polish: **{polish_outcome}**"
    if polish_detail:
        polish_line += f" ({polish_detail})"
    parts.append(polish_line + "\n")

    persona_majors: list[str] = []
    fidelity_findings: list[str] = []
    narrative_issues: list[str] = []
    slop_residue = 0
    if bundle is not None:
        for pr in bundle.persona_reviews:
            for issue in pr.issues:
                if issue.severity == "major":
                    persona_majors.append(f"[{pr.persona} / {issue.section}] {issue.issue} — fix: {issue.fix_hint}")
        fidelity_findings.extend(bundle.fidelity.unsupported_claims)
        fidelity_findings.extend(bundle.fidelity.number_mismatches)
        fidelity_findings.extend(bundle.fidelity.citation_issues)
        for ti in bundle.narrative.transition_issues:
            narrative_issues.append(f"[{ti.section}] {ti.issue}")
        narrative_issues.extend(bundle.narrative.promise_alignment_issues)
        slop_residue = len(bundle.slop.violations)

    parts.append("## Unresolved major persona issues\n")
    parts.append("\n".join(f"- {m}" for m in persona_majors) if persona_majors else "- none")
    parts.append("\n## Fidelity findings\n")
    parts.append("\n".join(f"- {m}" for m in fidelity_findings) if fidelity_findings else "- none")
    parts.append("\n## Narrative issues\n")
    parts.append("\n".join(f"- {m}" for m in narrative_issues) if narrative_issues else "- none")
    parts.append(f"\n## Slop residue\n\n{slop_residue} remaining slop violation(s).\n")
    parts.append(_ledger_section(ledger, records, pairwise_verdicts))
    parts.append("\n## Per-round telemetry scores\n")
    parts.append(_round_table(records) if records else "(no critique rounds ran)")
    parts.append("\n## TODO digest\n")
    parts.append(todo_digest.strip() or "(TODO.md is empty)")
    return "\n".join(parts) + "\n"


def _telemetry_score(bundle: CritiqueBundle) -> tuple[float, float, float, float, float]:
    """Compute the (total, persona, narrative, fidelity, slop) telemetry scores. Weights
    are unchanged from the previous score-driven loop; the total is telemetry only now."""
    if bundle.persona_reviews:
        persona_score = 1 - mean(pr.acceptance_risk for pr in bundle.persona_reviews)
    else:
        persona_score = 0.5
    narrative_score = bundle.narrative.score
    fidelity_score = bundle.fidelity.score
    slop_score = bundle.slop.score
    if bundle.skim is not None:
        total = round(
            0.30 * persona_score
            + 0.20 * narrative_score
            + 0.25 * fidelity_score
            + 0.10 * slop_score
            + 0.15 * bundle.skim.score,
            4,
        )
    else:
        total = round(
            0.35 * persona_score
            + 0.25 * narrative_score
            + 0.25 * fidelity_score
            + 0.15 * slop_score,
            4,
        )
    return total, persona_score, narrative_score, fidelity_score, slop_score


def _candidates_from_bundle(bundle: CritiqueBundle) -> list[dict]:
    """Turn a CritiqueBundle into normalized ledger candidate findings.

    persona majors+minors; narrative transition + promise issues (major); fidelity findings
    (major, persona 'fidelity'); skim issues (major if not sells else minor, persona 'skim').
    """
    cands: list[dict] = []
    for pr in bundle.persona_reviews:
        for issue in pr.issues:
            cands.append(
                convergence.make_candidate(
                    persona=pr.persona,
                    target=issue.section,
                    severity=issue.severity,
                    description=issue.issue,
                    fix_hint=issue.fix_hint,
                )
            )
    for ti in bundle.narrative.transition_issues:
        cands.append(
            convergence.make_candidate(
                persona="narrative",
                target=ti.section,
                severity="major",
                description=ti.issue,
                fix_hint=ti.fix_hint,
            )
        )
    for pa in bundle.narrative.promise_alignment_issues:
        cands.append(
            convergence.make_candidate(
                persona="narrative", target="global", severity="major", description=pa
            )
        )
    for finding in (
        list(bundle.fidelity.unsupported_claims)
        + list(bundle.fidelity.number_mismatches)
        + list(bundle.fidelity.citation_issues)
    ):
        cands.append(
            convergence.make_candidate(
                persona="fidelity", target="global", severity="major", description=finding
            )
        )
    if bundle.skim is not None:
        sev = "minor" if bundle.skim.sells else "major"
        for issue in bundle.skim.issues:
            cands.append(
                convergence.make_candidate(
                    persona="skim", target="global", severity=sev, description=issue
                )
            )
    return cands


async def _bootstrap_round0(node: str, ws: Workspace, req: WriteRequest) -> None:
    """Round 0 (bootstrap): deterministically fix mechanical failures (slop violations,
    invented citations) BEFORE the critique loop, with a synthesized plan (no personas, no
    .ai() planning) and a recompile. Silent no-op when nothing mechanical is broken."""
    slop = helpers.slop_lint(ws.paper_dir)
    bib_keys = _parse_bib_keys(ws.paper_dir)
    paper_text = helpers.assemble_paper_text(ws.paper_dir)
    invented = _invented_cite_keys(paper_text, bib_keys)

    if not slop.violations and not invented:
        print("[round 0 (bootstrap)] nothing mechanical to fix; skipping")
        return

    print(
        f"[round 0 (bootstrap)] slop_violations={len(slop.violations)} "
        f"invented_citations={len(invented)}"
    )
    # Synthesize a compact critique holding only the mechanical findings, then reuse the
    # deterministic planner (no .ai()) to route them into repair tasks.
    compact = {
        "personas": [],
        "narrative": {"transition_issues": [], "promise_alignment_issues": [], "arc_assessment": ""},
        "fidelity": {
            "blocking": bool(invented),
            "unsupported_claims": [],
            "number_mismatches": [],
            "citation_issues": [
                f"\\cite{{{k}}} used but absent from refs.bib (deterministic bootstrap check)"
                for k in invented
            ],
        },
        "slop": [f"{v.file}:{v.line} {v.rule} — {v.excerpt}" for v in slop.violations],
    }
    plan = repair._synthesize_plan(compact, "round 0 bootstrap: mechanical fixes")
    if not plan.tasks:
        return
    applied = await router.call(
        f"{node}.repair_apply_repairs",
        workspace=ws.model_dump(),
        plan=plan.model_dump(),
        model=req.model,
    )
    print(f"[round 0 (bootstrap)] applied {int(applied)} mechanical repair task(s)")
    await router.call(
        f"{node}.latex_compile_paper",
        workspace=ws.model_dump(),
        model=req.model,
    )


@router.reasoner(tags=["entry"])
async def write_paper(
    folder_path: str,
    target_venue: str | None = None,
    field_hint: str | None = None,
    max_rounds: int = 6,
    allow_web: bool = True,
    dry_run: bool = False,
    quality_threshold: float = 0.90,  # TELEMETRY-ONLY: no longer gates convergence.
    plateau_delta: float = 0.01,  # TELEMETRY-ONLY: replaced by ledger/pairwise mechanisms.
    slop_tolerance: int = 3,
    mode: str = "position",
    model: str | None = None,
) -> dict:
    req = WriteRequest(
        folder_path=folder_path,
        target_venue=target_venue,
        field_hint=field_hint,
        max_rounds=max_rounds,
        allow_web=allow_web,
        dry_run=dry_run,
        quality_threshold=quality_threshold,
        plateau_delta=plateau_delta,
        slop_tolerance=slop_tolerance,
        mode=mode,
        model=model,
    )
    node = helpers.node_id()
    print(f"[workflow] write_paper mode={req.mode!r}")

    # ---- P0: workspace + evidence ledger ---------------------------------
    print(f"[P0] preparing workspace from {req.folder_path}")
    ws = Workspace(
        **await router.call(
            f"{node}.intake_prepare_workspace",
            folder_path=req.folder_path,
            model=req.model,
        )
    )
    print(f"[P0] workspace ready run_id={ws.run_id} root={ws.root}")
    evidence = EvidenceSummary(
        **await router.call(
            f"{node}.intake_build_evidence_ledger",
            workspace=ws.model_dump(),
            mode=req.mode,
            model=req.model,
        )
    )
    print(f"[P0] evidence ledger built: {evidence.fact_count} facts, confident={evidence.confident}")
    helpers.save_state(
        ws.root,
        {"phase": "P0", "mode": req.mode, "fact_count": evidence.fact_count, "confident": evidence.confident},
    )
    helpers.git_snapshot(ws.root, "P0 evidence")

    # ---- P1: positioning (once) ------------------------------------------
    print("[P1] running positioning tournament")
    decision = PositioningDecision(
        **await router.call(
            f"{node}.positioning_run_positioning",
            workspace=ws.model_dump(),
            target_venue=req.target_venue,
            field_hint=req.field_hint,
            allow_web=req.allow_web,
            mode=req.mode,
            model=req.model,
        )
    )
    print(f"[P1] winning frame='{decision.winning_frame_name}' title='{decision.final_title}'")
    helpers.save_state(ws.root, {"phase": "P1", "title": decision.final_title, "frame": decision.winning_frame_name})
    helpers.git_snapshot(ws.root, "P1 positioning")

    # ---- P2: blueprint (may raise; let it propagate) ---------------------
    print("[P2] designing blueprint")
    bp = Blueprint(
        **await router.call(
            f"{node}.blueprint_design_blueprint",
            workspace=ws.model_dump(),
            target_venue=req.target_venue,
            mode=req.mode,
            model=req.model,
        )
    )
    print(f"[P2] blueprint: {len(bp.sections)} sections, {len(bp.figures)} figures")
    # Fact ids the blueprint deliberately excluded from the story. Threaded into the
    # round loop (to skip coverage candidates for them) and into finalization (to
    # supply the recorded reason in UNUSED_EVIDENCE.md).
    bp_unused_reasons: dict[str, str] = {u.fact_id: u.reason for u in bp.unused_evidence}
    bp_unused_ids: set[str] = set(bp_unused_reasons)
    helpers.save_state(ws.root, {"phase": "P2", "sections": len(bp.sections), "figures": len(bp.figures)})
    helpers.git_snapshot(ws.root, "P2 blueprint")

    if req.dry_run:
        print("[dry_run] stopping after blueprint")
        return WriteResult(
            status="planned",
            run_id=ws.run_id,
            workspace=ws.root,
            title=decision.final_title,
            positioning_path=ws.positioning_path,
            todo_path=ws.todo_path,
            stop_reason="dry_run",
        ).model_dump()

    # ---- P3: build --------------------------------------------------------
    print("[P3] building sections, figures, bibliography")
    build = BuildReport(
        **await router.call(
            f"{node}.build_run_build",
            workspace=ws.model_dump(),
            blueprint=bp.model_dump(),
            allow_web=req.allow_web,
            model=req.model,
        )
    )
    print(
        f"[P3] build done: {len(build.sections)} sections, {len(build.figures)} figures, "
        f"confident={build.confident}"
    )
    helpers.save_state(ws.root, {"phase": "P3", "confident": build.confident})
    helpers.git_snapshot(ws.root, "P3 build")

    # ---- P4: first compile gate ------------------------------------------
    print("[P4] compiling paper")
    compile_report = CompileReport(
        **await router.call(
            f"{node}.latex_compile_paper",
            workspace=ws.model_dump(),
            model=req.model,
        )
    )
    print(f"[P4] compile success={compile_report.success} attempts={compile_report.attempts}")
    helpers.save_state(ws.root, {"phase": "P4", "compile_ok": compile_report.success})
    helpers.git_snapshot(ws.root, "P4 compile")

    # ---- Round 0 bootstrap: deterministic mechanical fixes ---------------
    await _bootstrap_round0(node, ws, req)
    # Re-read compile state after the bootstrap may have edited + recompiled.
    compile_report = CompileReport(
        **await router.call(f"{node}.latex_compile_paper", workspace=ws.model_dump(), model=req.model)
    )
    helpers.git_snapshot(ws.root, "round 0 (bootstrap)")

    # ---- Convergence loop (countable/binary control; scores are telemetry) --
    records: list[RoundRecord] = []
    best: tuple[float, int | None] = (-1.0, None)
    stop_reason = "safety_cap_reached"
    bundle: CritiqueBundle | None = None
    ledger: list[LedgerIssue] = convergence.load_ledger(ws.model_dump())
    no_new_majors_streak = 0
    stall = 0
    pairwise_verdicts: list[tuple[int, str]] = []

    for round_no in range(1, req.max_rounds + 1):
        # 1. Snapshot the round-start state so a regression can be reverted cleanly.
        helpers.git_snapshot(ws.root, f"round {round_no} start")
        head = helpers._git(ws.root, ["rev-parse", "HEAD"], timeout=30)
        round_start_commit = head.stdout.strip() if head is not None and head.returncode == 0 else ""
        prev_draft_text = helpers.assemble_paper_text(ws.paper_dir)
        # Deep copy of the ledger as of round start, so a confident-regression revert
        # can restore issue statuses exactly (not just re-open this round's resolves).
        round_start_ledger = copy.deepcopy(ledger)

        # 2. Critique, anchored on the current ledger (persona + narrative only).
        print(f"[round {round_no}] running critique")
        bundle = CritiqueBundle(
            **await router.call(
                f"{node}.critique_run_critique",
                workspace=ws.model_dump(),
                round_no=round_no,
                open_issues=convergence.open_issue_dicts(ledger),
                model=req.model,
            )
        )

        # 3. Merge findings into the ledger: deterministic fingerprint pass, then one dedup call.
        candidates = _candidates_from_bundle(bundle)

        # 3a. Evidence-coverage candidates: any fact unused in the paper that the blueprint
        # did NOT deliberately exclude becomes a minor global finding pushing the writer to
        # weave it in. Same make_candidate mechanism -> stable fingerprint (target "global"
        # + fixed description head "Evidence fact E<n> is unused ...") makes it idempotent
        # across rounds via the pre_fps dedup below. Severity "minor" keeps it out of the
        # zero-open-majors convergence gate.
        cov = helpers.evidence_coverage(ws.root)
        for fid in cov["unused"]:
            if fid in bp_unused_ids:
                continue
            excerpt = helpers.evidence_fact_excerpt(ws.root, fid)
            candidates.append(
                convergence.make_candidate(
                    persona="coverage",
                    target="global",
                    severity="minor",
                    description=(
                        f"Evidence fact {fid} is unused in the paper: {excerpt}. "
                        "Weave it into the most natural section (Methods and SI count), "
                        "or it will be reported as unused evidence."
                    ),
                )
            )

        pre_fps = {i.fingerprint for i in ledger}
        survivors = {
            str(idx): c for idx, c in enumerate(candidates) if c["fingerprint"] not in pre_fps
        }
        duplicates: dict[str, str] = {}
        if survivors:
            try:
                dd = await router.call(
                    f"{node}.convergence_dedup_issues",
                    workspace=ws.model_dump(),
                    ledger=convergence.open_issue_dicts(ledger),
                    candidates=survivors,
                    model=req.model,
                )
                duplicates = dict(dd.get("duplicates", {}) or {})
            except Exception as err:  # noqa: BLE001 — degrade: treat all candidates as new.
                print(f"[workflow] convergence call failed (dedup_issues): {err}")
                duplicates = {}
        ledger, new_majors, reopened_ids = convergence.merge_candidates(
            ledger, candidates, round_no, duplicates
        )
        convergence.save_ledger(ws.model_dump(), ledger)
        open_major_issues = convergence.open_majors(ledger)
        print(
            f"[round {round_no}] ledger: {len(ledger)} total, {len(open_major_issues)} open majors, "
            f"{new_majors} new major(s) this round"
        )

        # 4. Telemetry score (unchanged weights) — logged, does NOT control the loop.
        total, persona_score, narrative_score, fidelity_score, slop_score = _telemetry_score(bundle)
        compile_ok = compile_report.success
        slop_count = len(bundle.slop.violations)

        record = RoundRecord(
            round=round_no,
            total_score=total,
            persona_score=round(persona_score, 4),
            narrative_score=round(narrative_score, 4),
            fidelity_score=round(fidelity_score, 4),
            slop_score=round(slop_score, 4),
            compile_ok=compile_ok,
            repairs_applied=0,
            stop=False,
            stop_reason="continue",
        )
        records.append(record)
        if total > best[0]:
            best = (total, round_no)
        helpers.git_snapshot(ws.root, f"round {round_no} critique score={total}")
        print(
            f"[round {round_no}] telemetry total={total} persona={persona_score:.4f} "
            f"narrative={narrative_score:.4f} fidelity={fidelity_score:.4f} "
            f"slop={slop_score:.4f} compile_ok={compile_ok} blocking={bundle.fidelity.blocking}"
        )

        def _persist() -> None:
            helpers.save_state(
                ws.root,
                {
                    "phase": "critique",
                    "round": round_no,
                    "total_score": total,
                    "persona_score": record.persona_score,
                    "narrative_score": record.narrative_score,
                    "fidelity_score": record.fidelity_score,
                    "slop_score": record.slop_score,
                    "compile_ok": compile_ok,
                    "open_majors": len(open_major_issues),
                    "evidence_coverage": helpers.evidence_coverage(ws.root)["ratio"],
                    "repairs_applied": record.repairs_applied,
                    "stop": record.stop,
                    "stop_reason": record.stop_reason,
                },
            )

        # 5. CONVERGENCE CHECK (all binary/countable).
        skim_sells = bundle.skim is None or bundle.skim.sells
        converged = (
            compile_ok
            and not bundle.fidelity.blocking
            and len(open_major_issues) == 0
            and slop_count <= req.slop_tolerance
            and skim_sells
        )
        if converged:
            record.stop = True
            record.stop_reason = stop_reason = "converged"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break

        # 6. DRY CHECK: two consecutive rounds with zero new unique majors and no open majors.
        if new_majors == 0:
            no_new_majors_streak += 1
        else:
            no_new_majors_streak = 0
        if no_new_majors_streak >= 2 and len(open_major_issues) == 0:
            record.stop = True
            record.stop_reason = stop_reason = "no_new_majors"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break

        # 7. Plan repairs (ledger-driven), apply, recompile.
        plan = RepairPlan(
            **await router.call(
                f"{node}.repair_plan_repairs",
                workspace=ws.model_dump(),
                critique=bundle.model_dump(),
                ledger_open_majors=convergence.open_issue_dicts(open_major_issues),
                model=req.model,
            )
        )
        if not plan.tasks:
            record.stop = True
            record.stop_reason = stop_reason = "no_repairs_needed"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break

        applied = await router.call(
            f"{node}.repair_apply_repairs",
            workspace=ws.model_dump(),
            plan=plan.model_dump(),
            model=req.model,
        )
        record.repairs_applied = int(applied)
        print(f"[round {round_no}] applied {record.repairs_applied} repair task(s)")

        compile_report = CompileReport(
            **await router.call(
                f"{node}.latex_compile_paper",
                workspace=ws.model_dump(),
                model=req.model,
            )
        )
        print(f"[round {round_no}] post-repair compile success={compile_report.success}")

        # 8. Verify: which ledger ids did the applied tasks address?
        addressed_ids: list[str] = []
        for t in plan.tasks:
            addressed_ids.extend(t.addresses or [])
        addressed_ids = list(dict.fromkeys(addressed_ids))
        if addressed_ids:
            verify_targets = [
                i for i in open_major_issues if i.id in set(addressed_ids)
            ]
        else:
            # Fallback: verify open majors whose target matches a repaired target.
            repaired_targets = {t.target for t in plan.tasks}
            verify_targets = [i for i in open_major_issues if i.target in repaired_targets]
        if verify_targets:
            try:
                report = VerificationReport(
                    **await router.call(
                        f"{node}.convergence_verify_resolutions",
                        workspace=ws.model_dump(),
                        issues=[
                            {
                                "id": i.id,
                                "description": i.description,
                                "target": i.target,
                                "fix_hint": i.fix_hint,
                            }
                            for i in verify_targets
                        ],
                        model=req.model,
                    )
                )
            except Exception as err:  # noqa: BLE001 — degrade: issues stay open, no attempts increment.
                print(f"[workflow] convergence call failed (verify_resolutions): {err}")
                report = None
            if report is not None:
                ledger = convergence.apply_verifications(
                    ledger, report, round_no, skip_attempt_ids=reopened_ids
                )
                convergence.save_ledger(ws.model_dump(), ledger)
                n_res = sum(1 for v in report.verifications if v.resolved)
                print(f"[round {round_no}] verify: {n_res}/{len(verify_targets)} resolved")

        # 9. Pairwise gate: blind judge of previous vs current draft.
        current_draft_text = helpers.assemble_paper_text(ws.paper_dir)
        try:
            verdict = PairwiseVerdict(
                **await router.call(
                    f"{node}.convergence_pairwise_compare",
                    workspace=ws.model_dump(),
                    previous_text=prev_draft_text,
                    current_text=current_draft_text,
                    model=req.model,
                )
            )
        except Exception as err:  # noqa: BLE001 — degrade: treat as a tie / no-op.
            print(f"[workflow] convergence call failed (pairwise_compare): {err}")
            verdict = PairwiseVerdict(winner="tie", rationale=f"pairwise call failed: {err}", confident=False)
        pairwise_verdicts.append((round_no, verdict.winner))
        print(f"[round {round_no}] pairwise winner={verdict.winner} confident={verdict.confident}")

        if verdict.winner == "previous" and verdict.confident:
            # Regression: revert the whole round back to its start state (working tree
            # AND ledger). reset --hard + clean -fd also removes files created this round
            # (e.g. new section files from repair harnesses); checkout -f would leave them.
            if round_start_commit:
                helpers._git(ws.root, ["reset", "--hard", round_start_commit], timeout=60)
                helpers._git(ws.root, ["clean", "-fd", ws.root], timeout=60)
                helpers.git_snapshot(ws.root, f"round {round_no} regression reverted")
            # Restore the ledger to its round-start statuses. save_ledger re-writes
            # ledger.json + ISSUES.md so they match the reverted text (the reset removed
            # or reverted the on-disk ledger; this makes it consistent again).
            ledger = round_start_ledger
            convergence.save_ledger(ws.model_dump(), ledger)
            open_major_issues = convergence.open_majors(ledger)
            compile_report = CompileReport(
                **await router.call(
                    f"{node}.latex_compile_paper", workspace=ws.model_dump(), model=req.model
                )
            )
            record.stop = True
            record.stop_reason = stop_reason = "regression_reverted"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break
        if verdict.winner == "current":
            stall = 0
        else:
            # "previous" (not confident) or "tie" -> a stall; two in a row ends the loop.
            stall += 1
            if stall >= 2:
                record.stop = True
                record.stop_reason = stop_reason = "pairwise_plateau"
                print(f"[round {round_no}] STOP {stop_reason}")
                _persist()
                break

        _persist()
    else:
        stop_reason = "safety_cap_reached"
        if records:
            records[-1].stop = True
            records[-1].stop_reason = stop_reason
        print(f"[loop] STOP {stop_reason}")

    # ---- Voice-unification polish pass (once, after convergence) ---------
    # Skip when the loop ended in a bad state: a blocking fidelity audit or a broken
    # compile means the paper is not in shape to hand to a single-voice rewrite.
    fidelity_blocking = bool(bundle is not None and bundle.fidelity.blocking)
    if fidelity_blocking or not compile_report.success:
        polish_outcome = "skipped"
        polish_detail = "loop ended with fidelity blocking" if fidelity_blocking else "loop ended with compile failure"
        print(f"[polish] skipped: {polish_detail}")
    else:
        print("[polish] running voice-unification pass")
        polish_report = await router.call(
            f"{node}.prose_polish_voice",
            workspace=ws.model_dump(),
            model=req.model,
        )
        polish_outcome = str(polish_report.get("outcome", "skipped"))
        polish_detail = str(polish_report.get("detail", ""))
        print(f"[polish] outcome={polish_outcome} detail={polish_detail!r}")
        # A reverted or applied pass may have moved the workspace; re-read the compile
        # state from disk is not needed here since the pass reverts to a compiling tree.
    helpers.save_state(ws.root, {"phase": "polish", "outcome": polish_outcome, "detail": polish_detail})

    # ---- Evidence-coverage finalization ----------------------------------
    # No fact silently disappears: every unused fact is recorded, with a reason, in
    # UNUSED_EVIDENCE.md. If all facts are used, no file is written (and any stale one
    # from an earlier round of this run is removed).
    final_cov = helpers.evidence_coverage(ws.root)
    unused_path = f"{ws.root}/UNUSED_EVIDENCE.md"
    total, used, unused = final_cov["total"], final_cov["used"], final_cov["unused"]
    if unused:
        parts = [
            "# Unused evidence",
            "",
            "Facts from EVIDENCE.md that did not fit the paper's story. Nothing was silently "
            "dropped; each entry records why.",
            "",
        ]
        for fid in unused:
            excerpt = helpers.evidence_fact_excerpt(ws.root, fid)
            reason = bp_unused_reasons.get(
                fid, "could not be integrated into the narrative during writing"
            )
            parts.append(f"## {fid}")
            parts.append("")
            parts.append(f"- Excerpt: {excerpt or '(no statement found)'}")
            parts.append(f"- Reason: {reason}")
            parts.append("")
        helpers.write_text(unused_path, "\n".join(parts))
        coverage_line = (
            f"Evidence coverage: {len(used)}/{total} facts used "
            f"({len(unused)} unused, see UNUSED_EVIDENCE.md)"
        )
    else:
        # Remove a stale UNUSED_EVIDENCE.md left by an earlier round of this same run.
        try:
            os.remove(unused_path)
        except OSError:
            pass
        coverage_line = f"Evidence coverage: {total}/{total} facts used"
    print(f"[coverage] {coverage_line}")
    helpers.save_state(
        ws.root,
        {"phase": "coverage", "evidence_coverage": final_cov["ratio"], "unused": unused},
    )

    # ---- REVIEW.md + final result ----------------------------------------
    todo_digest = helpers.read_text(ws.todo_path, limit=6000)
    review_body = _compose_review(
        bundle,
        records,
        stop_reason,
        todo_digest,
        req.mode,
        polish_outcome,
        polish_detail,
        ledger=ledger,
        pairwise_verdicts=pairwise_verdicts,
        coverage_line=coverage_line,
    )
    review_path = helpers.write_text(f"{ws.root}/REVIEW.md", review_body)
    helpers.git_snapshot(ws.root, "REVIEW")

    final_score = best[0] if best[1] is not None else 0.0
    print(
        f"[done] mode={req.mode} stop_reason={stop_reason} final_score={final_score} "
        f"rounds={len(records)} polish={polish_outcome}"
    )

    return WriteResult(
        status="completed",
        run_id=ws.run_id,
        workspace=ws.root,
        pdf_path=compile_report.pdf_path if compile_report.success else "",
        title=decision.final_title,
        rounds=records,
        final_score=final_score,
        stop_reason=stop_reason,
        todo_path=ws.todo_path,
        review_path=review_path,
        positioning_path=ws.positioning_path,
    ).model_dump()
