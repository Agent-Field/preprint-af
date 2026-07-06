from __future__ import annotations

import asyncio
import fnmatch
import glob
import json
import os
import re

from agentfield import AgentRouter

from . import helpers
from .models import RepairPlan, RepairTask, WorkerResult

router = AgentRouter(prefix="repair", tags=["repair"])

MAX_TASKS = 8


# --- critique compaction ----------------------------------------------------

def _compact_critique(critique: dict) -> dict:
    """Reduce a CritiqueBundle dump to only what planning needs.

    Per persona: all major issues + up to 3 top minor ones. All fidelity findings.
    Narrative transition + promise issues. Slop as file:line rule excerpt records.
    """
    personas = []
    for pr in critique.get("persona_reviews", []) or []:
        issues = pr.get("issues", []) or []
        majors = [i for i in issues if str(i.get("severity", "")).lower() == "major"]
        minors = [i for i in issues if str(i.get("severity", "")).lower() != "major"]
        kept = majors + minors[:3]
        personas.append(
            {
                "acceptance_risk": pr.get("acceptance_risk"),
                "verdict": pr.get("verdict", ""),
                "issues": [
                    {
                        "section": i.get("section", "global"),
                        "issue": i.get("issue", ""),
                        "fix_hint": i.get("fix_hint", ""),
                        "severity": i.get("severity", "minor"),
                    }
                    for i in kept
                ],
            }
        )

    narrative = critique.get("narrative", {}) or {}
    fidelity = critique.get("fidelity", {}) or {}
    slop = critique.get("slop", {}) or {}
    skim = critique.get("skim") or {}

    compact: dict = {
        "personas": personas,
        "narrative": {
            "transition_issues": [
                {
                    "section": i.get("section", "global"),
                    "issue": i.get("issue", ""),
                    "fix_hint": i.get("fix_hint", ""),
                }
                for i in (narrative.get("transition_issues", []) or [])
            ],
            "promise_alignment_issues": narrative.get("promise_alignment_issues", []) or [],
            "arc_assessment": narrative.get("arc_assessment", ""),
        },
        "fidelity": {
            "blocking": bool(fidelity.get("blocking")),
            "unsupported_claims": fidelity.get("unsupported_claims", []) or [],
            "number_mismatches": fidelity.get("number_mismatches", []) or [],
            "citation_issues": fidelity.get("citation_issues", []) or [],
        },
        "slop": [
            f"{v.get('file', '')}:{v.get('line', '')} {v.get('rule', '')} — {v.get('excerpt', '')}"
            for v in (slop.get("violations", []) or [])
        ],
    }

    # Include skim issues when the skim layer does not sell the paper or scores below 0.7.
    if skim and (not skim.get("sells", True) or skim.get("score", 1.0) < 0.7):
        skim_findings = []
        for issue in skim.get("issues", []) or []:
            issue_lower = issue.lower()
            # Route caption/figure issues to front_matter; topic-sentence issues to global;
            # title/abstract issues to front_matter; attribution unclear -> global.
            if any(kw in issue_lower for kw in ("caption", "figure", "title", "abstract")):
                target = "front_matter"
            else:
                target = "global"
            skim_findings.append({"section": target, "issue": issue, "fix_hint": "strengthen skim layer"})
        if skim_findings:
            compact["skim"] = skim_findings

    return compact


def _fidelity_findings(fidelity: dict) -> list[str]:
    return (
        list(fidelity.get("unsupported_claims", []) or [])
        + list(fidelity.get("number_mismatches", []) or [])
        + list(fidelity.get("citation_issues", []) or [])
    )


def _critique_is_dirty(compact: dict) -> bool:
    """True when the critique contains findings that MUST produce repair tasks."""
    if _fidelity_findings(compact.get("fidelity", {}) or {}):
        return True
    if compact.get("fidelity", {}).get("blocking"):
        return True
    for pr in compact.get("personas", []) or []:
        if any(str(i.get("severity", "")).lower() == "major" for i in pr.get("issues", [])):
            return True
    if compact.get("narrative", {}).get("transition_issues"):
        return True
    if compact.get("skim"):
        return True
    return False


def _synthesize_plan(compact: dict, reason: str) -> RepairPlan:
    """Deterministic plan from the critique when the model planner under-delivers.

    Groups persona majors + narrative transition issues by section, routes fidelity
    findings to a high-priority task, and attaches slop lines to their section tasks.
    """
    by_target: dict[str, list[str]] = {}

    def add(target: str, instruction: str) -> None:
        by_target.setdefault(target or "global", []).append(instruction)

    fidelity = compact.get("fidelity", {}) or {}
    for finding in _fidelity_findings(fidelity):
        add("global", f"Resolve fidelity finding: {finding}")

    for pr in compact.get("personas", []) or []:
        for issue in pr.get("issues", []) or []:
            if str(issue.get("severity", "")).lower() == "major":
                add(
                    issue.get("section", "global"),
                    f"{issue.get('issue', '')} Fix: {issue.get('fix_hint', '')}".strip(),
                )

    for issue in (compact.get("narrative", {}) or {}).get("transition_issues", []) or []:
        add(issue.get("section", "global"), f"Transition: {issue.get('issue', '')} Fix: {issue.get('fix_hint', '')}")

    for line in compact.get("slop", []) or []:
        # "paper/sections/03_method.tex:12 rule — excerpt" -> attach to that section's task
        m = re.search(r"\d+_([a-z0-9_]+)\.tex", line) if "sections/" in line else None
        add(m.group(1) if m else "global", f"Slop violation, apply mechanically: {line}")

    for sf in compact.get("skim", []) or []:
        add(sf.get("section", "global"), f"Skim layer: {sf.get('issue', '')} Fix: {sf.get('fix_hint', '')}")

    ordered = sorted(by_target.items(), key=lambda kv: (kv[0] != "global", kv[0]))
    tasks = [
        RepairTask(target=target, instructions=instrs[:12], priority="high" if target == "global" else "medium")
        for target, instrs in ordered
    ][:MAX_TASKS]
    return RepairPlan(
        tasks=tasks,
        notes=f"deterministic synthesized plan ({reason})",
        confident=False,
    )


def _fallback_blocking_plan(fidelity: dict, err: object) -> RepairPlan:
    """A blocking fidelity finding must never be dropped because planning crashed."""
    findings = _fidelity_findings(fidelity)
    task = RepairTask(
        target="global",
        instructions=findings or ["Resolve the blocking fidelity issue flagged by the audit."],
        priority="high",
    )
    return RepairPlan(
        tasks=[task],
        notes=f"planning crashed ({err}); synthesized deterministic plan for blocking fidelity findings",
        confident=False,
    )


def _ledger_block(ledger_open_majors: list[dict] | None) -> str:
    """Render the open-majors ledger block (with attempt counts) for the planner prompt."""
    if not ledger_open_majors:
        return ""
    lines: list[str] = []
    for i in ledger_open_majors:
        iid = i.get("id", "?")
        target = i.get("target", "global")
        attempts = i.get("attempts", 0)
        desc = str(i.get("description", "")).replace("\n", " ").strip()
        hint = str(i.get("fix_hint", "")).replace("\n", " ").strip()
        lines.append(f"- [{iid}] target={target} attempts={attempts}: {desc} (fix: {hint})")
    return "\n\nLEDGER (open majors, with attempt counts):\n" + "\n".join(lines) + "\n"


@router.reasoner()
async def plan_repairs(
    workspace: dict,
    critique: dict,
    ledger_open_majors: list[dict] | None = None,
    model: str | None = None,
) -> RepairPlan:
    """One .ai call routing critique findings into at most 8 targeted RepairTasks.

    When ``ledger_open_majors`` is provided, the plan MUST be driven by these open issues (plus
    blocking fidelity findings from the bundle). Each RepairTask carries `addresses` (ledger
    issue ids it resolves). Minor issues are never repaired while any major/blocking issue is open.
    """
    fidelity = critique.get("fidelity", {}) or {}
    print(
        f"[repair] plan_repairs round={critique.get('round')} "
        f"fidelity_blocking={bool(fidelity.get('blocking'))} "
        f"ledger_open_majors={len(ledger_open_majors or [])}"
    )
    try:
        compact = _compact_critique(critique)
        ledger_txt = _ledger_block(ledger_open_majors)
        result = await router.ai(
            system=(
                "You are a repair planner turning reviewer findings into a bounded set of edit "
                "tasks. Emit at MOST 8 RepairTasks. Group by `target`: one task per section slug "
                "at most; use 'front_matter' for title/abstract/main.tex issues; 'figures' for "
                "figure problems; 'bibliography' for citation/refs issues. Every fidelity finding "
                "(unsupported claim, number mismatch, citation issue) MUST be covered by a task "
                "with priority 'high'. Slop violations attach to the task for their section as "
                "mechanical instructions that quote the file and line, for example "
                "'line 34: replace the em dash with a comma'. Skim findings (under the 'skim' key) "
                "are issues with the paper's skim layer (title, abstract, captions, topic "
                "sentences); route them to 'front_matter' for title/abstract/caption issues or to "
                "the relevant section for topic-sentence issues; if attribution is unclear use "
                "'global'. Instructions must be concrete, executable edits ('rewrite the opening "
                "sentence to state the measured 3.2x speedup'), never judgments ('improve the "
                "flow').\n\n"
                "When a LEDGER block is present, the plan MUST be driven by those open major issues "
                "plus any blocking fidelity findings. Fill each RepairTask's `addresses` with the "
                "ledger issue ids it resolves (empty list is allowed only for purely mechanical "
                "tasks with no ledger id). Issues with attempts >= 2 have resisted targeted "
                "patching: for those, plan a full rewrite of the target (fix_hint escalation), not "
                "another patch. NEVER plan repairs for minor issues while any major or blocking "
                "issue is open; batch minors only once every major is exhausted (this prevents "
                "infinite polishing).\n\n"
                "If the critique contains nothing worth fixing (no major issues, no fidelity "
                "findings, only trivial residue), return an EMPTY tasks list — that signals "
                "convergence. Set `confident` truthfully."
            ),
            user=(
                "Critique digest (compacted CritiqueBundle):\n"
                f"{json.dumps(compact, indent=2, default=str)}"
                f"{ledger_txt}\n\n"
                "Return a RepairPlan with tasks (target, instructions, priority, addresses), notes, confident."
            ),
            schema=RepairPlan,
            model=helpers.ai_model(model),
        )

        # Enforce the contract: at most one task per target, cap at 8.
        seen: set[str] = set()
        deduped: list[RepairTask] = []
        for t in result.tasks:
            if t.target in seen:
                continue
            seen.add(t.target)
            deduped.append(t)
        result.tasks = deduped[:MAX_TASKS]

        # An empty plan may only signal convergence when the critique is actually clean.
        # A model that returns no tasks in the face of majors/fidelity findings is wrong;
        # synthesize a deterministic plan instead of silently converging.
        if not result.tasks and _critique_is_dirty(compact):
            print("[repair] planner returned 0 tasks on a dirty critique -> synthesizing plan")
            return _synthesize_plan(compact, "planner returned empty on dirty critique")

        print(f"[repair] plan_repairs -> {len(result.tasks)} task(s)")
        return result
    except Exception as err:  # noqa: BLE001
        print(f"[repair] plan_repairs crashed: {err}")
        compact = _compact_critique(critique)
        if _critique_is_dirty(compact):
            return _synthesize_plan(compact, f"planner crashed: {err}")
        if fidelity.get("blocking"):
            return _fallback_blocking_plan(fidelity, err)
        return helpers.safe_ai_fallback(RepairPlan, notes=str(err))


# --- application ------------------------------------------------------------

def _resolve(task: RepairTask, workspace: dict) -> tuple[list[str], list[str]]:
    """Return (display file list for the prompt, root-relative match patterns).

    A pattern ending with '/' is a directory prefix; others are exact/glob paths.
    """
    root = workspace["root"]
    sections_dir = workspace["sections_dir"]
    target = task.target

    if target == "front_matter":
        rel = ["paper/main.tex"]
        return rel, rel
    if target == "figures":
        return ["paper/figures/ (figure scripts and their pdfs)"], ["paper/figures/"]
    if target == "bibliography":
        rel = ["paper/refs.bib", "paper/CITATIONS_LOG.md"]
        return rel, rel

    # Section slug: resolve NN_<slug>.tex by globbing the sections directory.
    matches = sorted(glob.glob(os.path.join(sections_dir, f"*_{target}.tex")))
    if matches:
        rels = [os.path.relpath(m, root).replace(os.sep, "/") for m in matches]
        return rels, rels
    # 'global' (e.g. the blocking-fidelity fallback plan): any section file may be edited.
    if target == "global":
        return (
            ["paper/sections/ (any section file needed to resolve the findings)"],
            ["paper/sections/", "paper/main.tex"],
        )
    # Not yet on disk: allow the whole sections tree; prompt names the expectation.
    return (
        [f"paper/sections/NN_{target}.tex"],
        [f"paper/sections/*_{target}.tex", "paper/sections/"],
    )


def _matches(changed: set[str], patterns: list[str]) -> bool:
    for c in changed:
        cp = c.replace("\\", "/")
        for pat in patterns:
            if pat.endswith("/"):
                if cp.startswith(pat):
                    return True
            elif cp == pat or cp.endswith("/" + pat) or fnmatch.fnmatch(cp, pat):
                return True
    return False


async def _run_repair_task(task: RepairTask, workspace: dict, model: str | None):
    root = workspace["root"]
    prompt_files, _ = _resolve(task, workspace)
    numbered = "\n".join(f"{i}. {s}" for i, s in enumerate(task.instructions, 1))
    prompt = (
        f"You may modify ONLY these files: {', '.join(prompt_files)}.\n\n"
        f"Apply exactly these instructions:\n{numbered}\n\n"
        "Obey AGENTS.md. Never change evidence-backed numbers unless an instruction states the "
        "number contradicts EVIDENCE.md. Preserve \\todobox items unless an instruction resolves one."
    )
    try:
        return await router.harness(
            prompt,
            provider="opencode",
            model=helpers.opencode_model(model),
            cwd=root,
            project_dir=root,
            schema=WorkerResult,
        )
    except Exception as err:  # noqa: BLE001 — a crashed harness call counts as not applied.
        print(f"[repair] harness crashed target={task.target}: {err}")
        return None


@router.reasoner()
async def apply_repairs(
    workspace: dict, plan: dict, model: str | None = None
) -> int:
    """Apply the repair plan: front_matter first alone, then the rest in one gather."""
    root = workspace["root"]
    tasks = [RepairTask(**t) for t in (plan.get("tasks", []) or [])]
    if not tasks:
        print("[repair] apply_repairs: no tasks")
        return 0

    front = [t for t in tasks if t.target == "front_matter"]
    rest = [t for t in tasks if t.target != "front_matter"]
    applied = 0

    # Phase 1 — front_matter alone (it touches main.tex; keep it off the parallel path).
    if front:
        task = front[0]
        before = set(helpers.git_changed_files(root))
        result = await _run_repair_task(task, workspace, model)
        after = set(helpers.git_changed_files(root))
        _, patterns = _resolve(task, workspace)
        ok = result is not None and not result.is_error and _matches(after - before, patterns)
        applied += int(ok)
        print(f"[repair] front_matter applied={ok}")

    # Phase 2 — everything else in one gather.
    if rest:
        before = set(helpers.git_changed_files(root))
        results = await asyncio.gather(
            *[_run_repair_task(t, workspace, model) for t in rest],
            return_exceptions=True,
        )
        after = set(helpers.git_changed_files(root))
        newly = after - before
        for task, result in zip(rest, results):
            if isinstance(result, Exception) or result is None or result.is_error:
                print(f"[repair] task target={task.target} failed (no valid harness result)")
                continue
            _, patterns = _resolve(task, workspace)
            ok = _matches(newly, patterns)
            applied += int(ok)
            print(f"[repair] task target={task.target} applied={ok}")

    helpers.git_snapshot(root, f"repairs applied ({applied} tasks)")
    print(f"[repair] apply_repairs done applied={applied}/{len(tasks)}")
    return applied
