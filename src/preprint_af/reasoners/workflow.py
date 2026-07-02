from __future__ import annotations

from statistics import mean

from agentfield import AgentRouter

from . import helpers
from .models import (
    Blueprint,
    BuildReport,
    CompileReport,
    CritiqueBundle,
    EvidenceSummary,
    PositioningDecision,
    RepairPlan,
    RoundRecord,
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


def _compose_review(
    bundle: CritiqueBundle | None,
    records: list[RoundRecord],
    stop_reason: str,
    todo_digest: str,
) -> str:
    parts: list[str] = ["# REVIEW — unresolved problems\n"]
    parts.append(f"Stop reason: **{stop_reason}**\n")

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
    parts.append("## Per-round scores\n")
    parts.append(_round_table(records) if records else "(no critique rounds ran)")
    parts.append("\n## TODO digest\n")
    parts.append(todo_digest.strip() or "(TODO.md is empty)")
    return "\n".join(parts) + "\n"


@router.reasoner(tags=["entry"])
async def write_paper(
    folder_path: str,
    target_venue: str | None = None,
    field_hint: str | None = None,
    max_rounds: int = 6,
    allow_web: bool = True,
    dry_run: bool = False,
    quality_threshold: float = 0.90,
    plateau_delta: float = 0.01,
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
        model=model,
    )
    node = helpers.node_id()

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
            model=req.model,
        )
    )
    print(f"[P0] evidence ledger built: {evidence.fact_count} facts, confident={evidence.confident}")
    helpers.save_state(ws.root, {"phase": "P0", "fact_count": evidence.fact_count, "confident": evidence.confident})
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
            model=req.model,
        )
    )
    print(f"[P2] blueprint: {len(bp.sections)} sections, {len(bp.figures)} figures")
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

    # ---- Convergence loop -------------------------------------------------
    records: list[RoundRecord] = []
    prev_score: float | None = None
    no_improve = 0
    best: tuple[float, int | None] = (-1.0, None)
    stop_reason = "safety_cap_reached"
    bundle: CritiqueBundle | None = None

    for round_no in range(1, req.max_rounds + 1):
        print(f"[round {round_no}] running critique")
        bundle = CritiqueBundle(
            **await router.call(
                f"{node}.critique_run_critique",
                workspace=ws.model_dump(),
                round_no=round_no,
                model=req.model,
            )
        )

        if bundle.persona_reviews:
            persona_score = 1 - mean(pr.acceptance_risk for pr in bundle.persona_reviews)
        else:
            persona_score = 0.5
        narrative_score = bundle.narrative.score
        fidelity_score = bundle.fidelity.score
        slop_score = bundle.slop.score
        total = round(
            0.35 * persona_score
            + 0.25 * narrative_score
            + 0.25 * fidelity_score
            + 0.15 * slop_score,
            4,
        )
        compile_ok = compile_report.success

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
            f"[round {round_no}] total={total} persona={persona_score:.4f} "
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
                    "repairs_applied": record.repairs_applied,
                    "stop": record.stop,
                    "stop_reason": record.stop_reason,
                },
            )

        # convergence: quality threshold met
        converged = compile_ok and total >= quality_threshold and not bundle.fidelity.blocking
        if converged:
            record.stop = True
            record.stop_reason = stop_reason = "quality_threshold_met"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break

        # plateau detection
        if prev_score is not None and (total - prev_score) < plateau_delta:
            no_improve += 1
            if no_improve >= 2:
                record.stop = True
                record.stop_reason = stop_reason = "quality_plateau"
                print(f"[round {round_no}] STOP {stop_reason}")
                _persist()
                break
        else:
            no_improve = 0

        # plan repairs
        plan = RepairPlan(
            **await router.call(
                f"{node}.repair_plan_repairs",
                workspace=ws.model_dump(),
                critique=bundle.model_dump(),
                model=req.model,
            )
        )
        if not plan.tasks:
            record.stop = True
            record.stop_reason = stop_reason = "no_repairs_needed"
            print(f"[round {round_no}] STOP {stop_reason}")
            _persist()
            break

        # apply repairs
        applied = await router.call(
            f"{node}.repair_apply_repairs",
            workspace=ws.model_dump(),
            plan=plan.model_dump(),
            model=req.model,
        )
        record.repairs_applied = int(applied)
        print(f"[round {round_no}] applied {record.repairs_applied} repair task(s)")

        # post-repair compile gate
        compile_report = CompileReport(
            **await router.call(
                f"{node}.latex_compile_paper",
                workspace=ws.model_dump(),
                model=req.model,
            )
        )
        print(f"[round {round_no}] post-repair compile success={compile_report.success}")

        prev_score = total
        _persist()
    else:
        stop_reason = "safety_cap_reached"
        if records:
            records[-1].stop = True
            records[-1].stop_reason = stop_reason
        print(f"[loop] STOP {stop_reason}")

    # ---- REVIEW.md + final result ----------------------------------------
    todo_digest = helpers.read_text(ws.todo_path, limit=6000)
    review_body = _compose_review(bundle, records, stop_reason, todo_digest)
    review_path = helpers.write_text(f"{ws.root}/REVIEW.md", review_body)
    helpers.git_snapshot(ws.root, "REVIEW")

    final_score = best[0] if best[1] is not None else 0.0
    print(f"[done] stop_reason={stop_reason} final_score={final_score} rounds={len(records)}")

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
