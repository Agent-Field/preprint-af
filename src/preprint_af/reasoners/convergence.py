from __future__ import annotations

import glob
import json
import os
import random
import re

from agentfield import AgentRouter

from . import helpers
from .models import (
    DedupDecision,
    LedgerIssue,
    PairwiseVerdict,
    VerificationReport,
)

router = AgentRouter(prefix="convergence", tags=["convergence"])

# Character caps for the .ai() calls in this module.
_VERIFY_CAP = 30_000
_PAIRWISE_CAP = 40_000
_DEDUP_LEDGER_CAP = 8_000


# --------------------------------------------------------------------------- #
# Pure-python ledger management                                               #
# --------------------------------------------------------------------------- #


def _ledger_path(workspace: dict) -> str:
    return os.path.join(workspace["reviews_dir"], "ledger.json")


def _issues_md_path(workspace: dict) -> str:
    return os.path.join(workspace["reviews_dir"], "ISSUES.md")


def load_ledger(workspace: dict) -> list[LedgerIssue]:
    """Load the issue ledger from reviews/ledger.json. Missing/corrupt -> empty list."""
    text = helpers.read_text(_ledger_path(workspace))
    if not text.strip():
        return []
    try:
        raw = json.loads(text)
    except (ValueError, TypeError):
        return []
    issues: list[LedgerIssue] = []
    for item in raw if isinstance(raw, list) else []:
        try:
            issues.append(LedgerIssue(**item))
        except Exception:  # noqa: BLE001 — skip malformed rows, never crash.
            continue
    return issues


def _md_cell(s: object) -> str:
    return str(s).replace("|", "\\|").replace("\n", " ").strip()


def _render_issues_md(issues: list[LedgerIssue]) -> str:
    lines = [
        "# Issue ledger\n",
        f"Total: {len(issues)} | "
        f"open: {sum(1 for i in issues if i.status == 'open')} | "
        f"resolved: {sum(1 for i in issues if i.status == 'resolved')} | "
        f"wontfix: {sum(1 for i in issues if i.status == 'wontfix')}\n",
        "| id | status | severity | target | description |",
        "|---|---|---|---|---|",
    ]
    for it in issues:
        lines.append(
            f"| {_md_cell(it.id)} | {_md_cell(it.status)} | {_md_cell(it.severity)} | "
            f"{_md_cell(it.target)} | {_md_cell(it.description)[:200]} |"
        )
    return "\n".join(lines) + "\n"


def save_ledger(workspace: dict, issues: list[LedgerIssue]) -> None:
    """Persist the ledger to reviews/ledger.json and regenerate the human-readable ISSUES.md."""
    helpers.write_text(
        _ledger_path(workspace),
        json.dumps([i.model_dump() for i in issues], indent=2),
    )
    helpers.write_text(_issues_md_path(workspace), _render_issues_md(issues))


def next_issue_id(round_no: int, issues: list[LedgerIssue]) -> str:
    """Return the next stable 'I-<round>-<seq>' id for this round given existing issues."""
    prefix = f"I-{round_no}-"
    seq = 0
    for it in issues:
        if it.id.startswith(prefix):
            try:
                n = int(it.id[len(prefix):])
            except ValueError:
                continue
            seq = max(seq, n)
    return f"{prefix}{seq + 1}"


_NORMALIZE_RX = re.compile(r"[^a-z0-9 ]+")


def _normalize_words(text: str, n: int = 10) -> str:
    """Lowercase, keep alnum+space only, take the first n words."""
    cleaned = _NORMALIZE_RX.sub(" ", (text or "").lower())
    words = cleaned.split()
    return " ".join(words[:n])


def fingerprint(target: str, description: str) -> str:
    """Deterministic dedup key: '<target>:<first 10 normalized words of description>'."""
    return f"{target or 'global'}:{_normalize_words(description)}"


# --------------------------------------------------------------------------- #
# Candidate finding intake                                                     #
# --------------------------------------------------------------------------- #


def make_candidate(
    persona: str, target: str, severity: str, description: str, fix_hint: str = ""
) -> dict:
    """Build a normalized candidate finding dict with its deterministic fingerprint."""
    target = target or "global"
    return {
        "persona": persona,
        "target": target,
        "severity": severity if severity in ("major", "minor") else "minor",
        "description": description or "",
        "fix_hint": fix_hint or "",
        "fingerprint": fingerprint(target, description or ""),
    }


def merge_candidates(
    issues: list[LedgerIssue],
    candidates: list[dict],
    round_no: int,
    duplicates: dict[str, str] | None = None,
) -> tuple[list[LedgerIssue], int, set[str]]:
    """Merge candidate findings into the ledger.

    Deterministic exact-fingerprint duplicates are merged in code (no LLM). ``duplicates``
    maps a candidate key (its index as a string) to an existing ledger id for the semantic
    pass; those candidates re-open a resolved issue rather than create a new one. Returns
    the updated ledger, the count of NEW unique major issues opened this round, and the set
    of ledger ids re-opened this round (each already bumped attempts, so apply_verifications
    must NOT bump them again in the same round).
    """
    duplicates = duplicates or {}
    by_fp: dict[str, LedgerIssue] = {}
    for it in issues:
        by_fp.setdefault(it.fingerprint, it)
    new_majors = 0
    reopened_ids: set[str] = set()

    for idx, cand in enumerate(candidates):
        existing = by_fp.get(cand["fingerprint"])
        mapped_id = duplicates.get(str(idx))
        if existing is None and mapped_id:
            existing = next((i for i in issues if i.id == mapped_id), None)

        if existing is not None:
            # Re-open a resolved issue that resurfaced; bump attempts.
            if existing.status == "resolved":
                existing.status = "open"
                existing.round_resolved = None
                existing.attempts += 1
                reopened_ids.add(existing.id)
            continue

        new = LedgerIssue(
            id=next_issue_id(round_no, issues),
            fingerprint=cand["fingerprint"],
            persona=cand["persona"],
            target=cand["target"],
            severity=cand["severity"],
            description=cand["description"],
            fix_hint=cand.get("fix_hint", ""),
            status="open",
            round_opened=round_no,
        )
        issues.append(new)
        by_fp.setdefault(new.fingerprint, new)
        if new.severity == "major":
            new_majors += 1

    return issues, new_majors, reopened_ids


def open_issue_dicts(issues: list[LedgerIssue]) -> list[dict]:
    """Serialize open + resolved issues for anchoring (critique/planner payloads)."""
    return [
        {
            "id": i.id,
            "status": i.status,
            "severity": i.severity,
            "target": i.target,
            "description": i.description,
            "fix_hint": i.fix_hint,
            "attempts": i.attempts,
        }
        for i in issues
        if i.status in ("open", "resolved")
    ]


def open_majors(issues: list[LedgerIssue]) -> list[LedgerIssue]:
    return [i for i in issues if i.status == "open" and i.severity == "major"]


# --------------------------------------------------------------------------- #
# Reasoner: semantic dedup of surviving candidates                            #
# --------------------------------------------------------------------------- #


@router.reasoner()
async def dedup_issues(
    workspace: dict, ledger: list[dict], candidates: dict[str, dict], model: str | None = None
) -> DedupDecision:
    """One .ai() call mapping candidate findings that are semantically the same underlying
    problem to an existing ledger issue id. Degrades to "no duplicates" on any failure so
    every candidate is then treated as new.

    ``candidates`` is a dict keyed by candidate key (index string) -> finding dict.
    """
    if not candidates or not ledger:
        return DedupDecision(duplicates={}, confident=True)
    print(f"[convergence] dedup_issues ledger={len(ledger)} candidates={len(candidates)}")
    ledger_lines = "\n".join(
        f"- {i.get('id')} [{i.get('status')}/{i.get('severity')}] "
        f"({i.get('target')}) {i.get('description', '')[:200]}"
        for i in ledger
    )[:_DEDUP_LEDGER_CAP]
    cand_lines = "\n".join(
        f"- {key}: ({c.get('target')}) {c.get('description', '')[:200]}"
        for key, c in candidates.items()
    )
    try:
        result = await router.ai(
            system=(
                "You deduplicate reviewer findings against a ledger of already-tracked issues. "
                "A candidate is a duplicate only when it describes the SAME underlying problem in "
                "the same place as an existing issue, even if worded differently. Do NOT merge "
                "distinct problems that happen to share a section. Return `duplicates`: a mapping "
                "from each duplicate candidate's key to the existing ledger issue id it matches. "
                "Omit candidates that are genuinely new. Set `confident` truthfully."
            ),
            user=(
                "EXISTING LEDGER ISSUES (id [status/severity] (target) description):\n"
                f"{ledger_lines or '(none)'}\n\n"
                "CANDIDATE FINDINGS (key: (target) description):\n"
                f"{cand_lines or '(none)'}\n\n"
                "Return DedupDecision with `duplicates` (candidate key -> existing id) and `confident`."
            ),
            schema=DedupDecision,
            model=helpers.ai_model(model),
        )
        # Guard: only keep mappings whose value is a real ledger id.
        valid_ids = {i.get("id") for i in ledger}
        result.duplicates = {
            k: v for k, v in (result.duplicates or {}).items() if v in valid_ids and k in candidates
        }
        print(f"[convergence] dedup_issues -> {len(result.duplicates)} semantic duplicate(s)")
        return result
    except Exception as err:  # noqa: BLE001 — degrade: treat all candidates as new.
        print(f"[convergence] dedup_issues crashed (treating all as new): {err}")
        return DedupDecision(duplicates={}, confident=False)


# --------------------------------------------------------------------------- #
# Reasoner: verify that this round's repairs resolved their issues            #
# --------------------------------------------------------------------------- #


def _section_text_for_target(workspace: dict, target: str) -> str:
    """Return the relevant paper text for a target: the section file for a section slug,
    or the assembled paper (capped) for front_matter/global/other targets."""
    sections_dir = workspace["sections_dir"]
    paper_dir = workspace["paper_dir"]
    if target and target not in ("front_matter", "global", "figures", "bibliography"):
        matches = sorted(glob.glob(os.path.join(sections_dir, f"*_{target}.tex")))
        if matches:
            return helpers.read_text(matches[0])[:_VERIFY_CAP]
    return helpers.assemble_paper_text(paper_dir)[:_VERIFY_CAP]


@router.reasoner()
async def verify_resolutions(
    workspace: dict, issues: list[dict], model: str | None = None
) -> VerificationReport:
    """One .ai() call giving a binary resolved verdict + evidence quote per addressed issue.

    ``issues`` = the issues this round's repairs addressed (id, description, target, fix_hint).
    Degrades to "all unresolved" on any failure (issues stay open).
    """
    if not issues:
        return VerificationReport(verifications=[], confident=True)
    print(f"[convergence] verify_resolutions issues={len(issues)}")

    # Gather relevant text once per distinct target (section file or assembled paper).
    targets = {str(i.get("target") or "global") for i in issues}
    text_blocks: list[str] = []
    for tgt in sorted(targets):
        block = _section_text_for_target(workspace, tgt)
        text_blocks.append(f"=== TEXT FOR TARGET '{tgt}' ===\n{block}")
    paper_text = "\n\n".join(text_blocks)[: _VERIFY_CAP * 2]

    issue_lines = "\n".join(
        f"- {i.get('id')} (target={i.get('target')}): {i.get('description', '')} "
        f"[fix hint: {i.get('fix_hint', '')}]"
        for i in issues
    )
    try:
        result = await router.ai(
            system=(
                "You verify whether SPECIFIC previously-identified issues have been fixed in the "
                "current text. For each issue answer resolved true/false. resolved=true ONLY if you "
                "can quote text demonstrating the fix. When uncertain, resolved=false. Put the "
                "supporting quote (or the reason it is still unresolved) in `evidence`."
            ),
            user=(
                "ISSUES TO VERIFY (id (target): description [fix hint]):\n"
                f"{issue_lines}\n\n"
                "CURRENT PAPER TEXT:\n"
                f"{paper_text}\n\n"
                "Return VerificationReport with one IssueVerification per issue id above."
            ),
            schema=VerificationReport,
            model=helpers.ai_model(model),
        )
        return result
    except Exception as err:  # noqa: BLE001 — degrade: leave issues open (unverified).
        print(f"[convergence] verify_resolutions crashed (leaving issues open): {err}")
        return VerificationReport(
            verifications=[
                {"issue_id": str(i.get("id")), "resolved": False, "evidence": f"verify crashed: {err}"}
                for i in issues
            ],
            confident=False,
        )


def apply_verifications(
    issues: list[LedgerIssue],
    report: VerificationReport,
    round_no: int,
    skip_attempt_ids: set[str] | None = None,
) -> list[LedgerIssue]:
    """Update ledger from a VerificationReport: resolved -> status=resolved; else attempts += 1.

    ``skip_attempt_ids`` are ids re-opened by merge_candidates this round (already bumped
    once); an unresolved verification for those does NOT bump attempts again this round.
    """
    skip_attempt_ids = skip_attempt_ids or set()
    by_id = {i.id: i for i in issues}
    for v in report.verifications:
        it = by_id.get(v.issue_id)
        if it is None or it.status != "open":
            continue
        if v.resolved:
            it.status = "resolved"
            it.round_resolved = round_no
        elif it.id not in skip_attempt_ids:
            it.attempts += 1
    return issues


# --------------------------------------------------------------------------- #
# Reasoner: blind pairwise comparison of consecutive drafts                    #
# --------------------------------------------------------------------------- #


@router.reasoner()
async def pairwise_compare(
    workspace: dict,
    previous_text: str,
    current_text: str,
    model: str | None = None,
) -> PairwiseVerdict:
    """One BLIND .ai() call judging which draft is closer to acceptance. The two drafts are
    randomly assigned to labels A/B in code so the judge cannot tell which is newer; the
    verdict is mapped back to current/previous/tie. Degrades to a tie on any failure.
    """
    prev = (previous_text or "")[:_PAIRWISE_CAP]
    curr = (current_text or "")[:_PAIRWISE_CAP]
    if not prev or not curr:
        return PairwiseVerdict(winner="tie", rationale="missing draft text", confident=False)

    # Blind assignment: randomly decide which physical draft is Draft A.
    current_is_a = random.random() < 0.5
    draft_a, draft_b = (curr, prev) if current_is_a else (prev, curr)
    print(f"[convergence] pairwise_compare current_is_A={current_is_a}")
    try:
        result = await router.ai(
            system=(
                "You compare two drafts of the same scientific paper and judge which is closer to "
                "acceptance at a top venue. Judge argument clarity, evidence presentation, and prose "
                "quality. Small wording differences are a tie; declare a winner only for a clearly "
                "better draft. Answer with `winner` = 'Draft A', 'Draft B', or 'tie'."
            ),
            user=(
                "=== DRAFT A ===\n"
                f"{draft_a}\n\n"
                "=== DRAFT B ===\n"
                f"{draft_b}\n\n"
                "Return PairwiseVerdict with `winner` ('Draft A' | 'Draft B' | 'tie'), rationale, confident."
            ),
            schema=PairwiseVerdict,
            model=helpers.ai_model(model),
        )
        raw = (result.winner or "tie").strip().lower()
        if "a" in raw and "draft" in raw:
            label = "a"
        elif "b" in raw and "draft" in raw:
            label = "b"
        elif raw in ("a", "b"):
            label = raw
        else:
            label = "tie"
        if label == "tie":
            mapped = "tie"
        elif (label == "a") == current_is_a:
            mapped = "current"
        else:
            mapped = "previous"
        print(f"[convergence] pairwise_compare -> winner={mapped} (raw={result.winner!r})")
        return PairwiseVerdict(winner=mapped, rationale=result.rationale, confident=result.confident)
    except Exception as err:  # noqa: BLE001 — degrade: treat as a tie / no-op.
        print(f"[convergence] pairwise_compare crashed (treating as tie): {err}")
        return PairwiseVerdict(winner="tie", rationale=f"pairwise crashed: {err}", confident=False)
