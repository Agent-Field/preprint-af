from __future__ import annotations

import os

from agentfield import AgentRouter

from . import helpers
from .models import CompileReport, FidelityAudit, Workspace

router = AgentRouter(prefix="prose", tags=["prose"])


def _polish_prompt(ws: Workspace) -> str:
    return f"""\
You are the single author doing the final read-aloud rewrite of this paper. The sections were
drafted by different writers; unify them into ONE voice. The workspace root is your working
directory and OpenCode reads AGENTS.md there — obey it in full.

## Read first (do not write until you have)
Read these workspace files completely before rewriting a word:
  - AGENTS.md        (the binding style and fidelity contract — this governs everything)
  - POSITIONING.md   (the chosen frame, final title and abstract, opening thesis)
Then read every file under `paper/sections/` and `paper/main.tex`, front to back, so the whole
paper is in your head before you touch it.

## What to rewrite for
- ONE consistent voice and register across every section, as if one senior author wrote the paper.
- Varied sentence and paragraph rhythm; break up any stretch of uniform-length sentences.
- Topic sentences that carry claims: reading only the first sentence of each paragraph must yield
  the paper's argument (the skim test).
- Old-before-new information flow: open each sentence with what the reader already knows and put
  the payload at the stress position, the end of the sentence.
- Transitions that carry scientific content, never signposts. No "In this section, we ...".
- No two sections opening with the same construction.

## HARD constraints (do not violate any of these)
- Do NOT change any number, citation key, claim strength, equation, figure reference, `% E<n>`
  evidence comment line, section order, or heading.
- Do NOT add or remove any `\\todobox` item.
- You may ONLY rewrite prose within these constraints. Rewording sentences, splitting or joining
  sentences, resequencing paragraphs within a section for flow, and smoothing transitions are all
  allowed as long as every constraint above holds and every claim keeps its exact numbers,
  citations, and strength.

## Writable files (touch nothing else)
  - paper/sections/*.tex   (every section file)
  - the abstract inside paper/main.tex ONLY (the text between \\begin{{abstract}} and
    \\end{{abstract}}); do NOT touch the title, preamble, \\input lines, bibliography, or the
    \\todobox definition in main.tex.

Do the work now: read AGENTS.md, POSITIONING.md, and the whole paper, then rewrite for one voice
within the hard constraints. Return a WorkerResult naming the files you rewrote."""


@router.reasoner()
async def polish_voice(workspace: dict, model: str | None = None) -> dict:
    """Final voice-unification pass: ONE harness rewrites the whole paper into a single voice,
    guarded by a pre-pass commit, a post-pass fidelity re-audit, and a recompile. If fidelity
    comes back blocking or the compile fails, the workspace is reverted to the pre-pass commit.

    Returns a plain dict: {outcome, reverted, detail} where outcome is one of
    'applied' | 'reverted' | 'skipped'.
    """
    ws = Workspace(**workspace)
    root = ws.root
    nid = helpers.node_id()
    print(f"[prose] polish_voice: unifying voice across {root}")

    # -- safety net: commit the pre-pass state so we can roll back cleanly -----
    helpers.git_snapshot(root, "pre-polish snapshot")
    head = helpers._git(root, ["rev-parse", "HEAD"], timeout=30)
    pre_commit = head.stdout.strip() if head is not None and head.returncode == 0 else ""
    if not pre_commit:
        print("[prose] polish_voice skipped: could not resolve a pre-pass commit to roll back to")
        return {"outcome": "skipped", "reverted": False, "detail": "no pre-pass commit available"}

    # -- the single voice-unification harness pass ----------------------------
    result = await router.harness(
        _polish_prompt(ws),
        provider="opencode",
        model=helpers.opencode_model(model),
        cwd=root,
        project_dir=root,
    )
    if result.is_error:
        print(f"[prose] polish harness errored, reverting: {result.error_message}")
        _revert(root, pre_commit)
        return {"outcome": "reverted", "reverted": True, "detail": f"harness error: {result.error_message}"}

    helpers.git_snapshot(root, "post-polish draft")

    # -- re-run the fidelity audit: the rewrite must not have drifted any claim -
    fidelity = FidelityAudit(
        **await router.call(
            f"{nid}.critique_fidelity_audit",
            workspace=workspace,
            round_no=0,
            model=model,
        )
    )
    # -- recompile: the rewrite must not have broken LaTeX --------------------
    compile_report = CompileReport(
        **await router.call(
            f"{nid}.latex_compile_paper",
            workspace=workspace,
            model=model,
        )
    )
    print(
        f"[prose] post-polish gates: fidelity_blocking={fidelity.blocking} "
        f"compile_ok={compile_report.success}"
    )

    if fidelity.blocking or not compile_report.success:
        reason = (
            "fidelity blocking" if fidelity.blocking else "compile failed"
        )
        print(f"[prose] polish reverted ({reason})")
        _revert(root, pre_commit)
        return {"outcome": "reverted", "reverted": True, "detail": f"post-pass gate: {reason}"}

    helpers.git_snapshot(root, "polish applied")
    print("[prose] polish applied: voice unified, fidelity clean, compile ok")
    return {"outcome": "applied", "reverted": False, "detail": "voice unified within constraints"}


# --------------------------------------------------------------------------- #
# Helpers (plain functions)
# --------------------------------------------------------------------------- #
def _revert(root: str, commit: str) -> None:
    """Roll the workspace tree back to a known-good commit, then snapshot the revert."""
    helpers._git(root, ["checkout", "-f", commit, "--", "."], timeout=60)
    helpers.git_snapshot(root, "polish reverted")
