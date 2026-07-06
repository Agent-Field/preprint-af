from __future__ import annotations

import os

from agentfield import AgentRouter

from . import helpers
from .models import EvidenceSummary, Workspace

router = AgentRouter(prefix="intake", tags=["intake"])


@router.reasoner()
async def prepare_workspace(folder_path: str, model: str | None = None) -> Workspace:
    """P0a: deterministically scaffold the run workspace from the user's folder."""
    workspace = helpers.create_workspace(folder_path)
    print(f"[intake] workspace ready run_id={workspace.run_id} root={workspace.root} "
          f"input_files={len(workspace.input_files)} draft={workspace.has_existing_draft} "
          f"data={workspace.has_data_files}")
    return workspace


def _evidence_prompt(workspace: dict, mode: str = "position") -> str:
    input_files = workspace.get("input_files") or []
    listing = "\n".join(f"  - input/{p}" for p in input_files) or "  (input listing unavailable — walk input/ yourself)"
    py = helpers.figure_python()
    evidence_path = workspace.get("evidence_path", "EVIDENCE.md")
    has_draft = workspace.get("has_existing_draft", False)
    if mode == "position":
        gaps_block = (
            "Bullet list of SCOPE NOTES: where the evidence is bounded, so the paper's "
            "claims can be scoped down to what it already supports. Phrase every item as a "
            "limitation / scope-down candidate, NOT as an experiment TODO, e.g. \"evidence "
            "covers accuracy on dataset X but not on Y; claims must be scoped to X\". These "
            "are recorded in EVIDENCE.md ONLY (they are NOT tracked as action items); they "
            "tell the writer where to bound the story, not what new experiment to run."
        )
    else:
        gaps_block = (
            "Bullet list of experiments or data that are MISSING for stronger or additional "
            "claims. Phrase every item as an actionable TODO (start with a verb), e.g. \"Run "
            "the ablation without component X to isolate its contribution.\" These become "
            "tracked TODO items."
        )
    return f"""\
You are building the FACT LEDGER for a scientific paper. The workspace root is your working
directory. All of the user's raw material lives under `input/`. Your single deliverable is a
file named `EVIDENCE.md` written at the workspace root (path: `{evidence_path}`).

## Step 1 — Explore input/ exhaustively
Do not skim. Actually open and read the material. Depending on what is present, inspect:
- data files: .csv, .tsv, .json, .jsonl, .parquet, .npz, .npy, .pkl, .h5 — load them and look
  at columns, shapes, row counts, ranges, summary statistics;
- notebooks (.ipynb): read code cells AND their recorded outputs (metrics, tables, printed numbers);
- existing drafts or papers (.tex, .md, .txt, .rst) and any PDFs — read them fully;
- figure/plot scripts (.py, .R, .m) and the data they consume;
- logs, results dumps, config/hyperparameter files, README/notes;
- bibliography files (.bib) and any inline reference lists.
There are {len(input_files)} known files under input/:
{listing}

## Step 2 — Compute, never invent
NEVER invent, round-trip-guess, or extrapolate a number. Every quantitative statement must come
from a file you actually read. If a value is trivially derivable from the data (a mean, a max, a
count, a delta, a percentage, a ratio), COMPUTE it with the Python interpreter at:
    {py}
Run small scripts (e.g. `{py} -c "..."` or a temp .py file) that load the real data files and
print the value. Record HOW each derived number was computed. If something cannot be derived from
the material present, it is a Gap, not a Fact.

## Step 3 — Write EVIDENCE.md with EXACTLY this section structure

### `## Facts`
One `### E<n>` subheading per fact, numbered E1, E2, E3, ... In each entry give, on their own lines:
- **Statement:** the claim in one sentence.
- **Numbers:** the exact values (with units), copied or computed — never approximated.
- **Source:** the exact file path(s) under input/ the fact comes from.
- **Derivation:** how it was obtained (direct read, or the exact computation / command used).

### `## Figure candidates`
Bullet list. Each item: what plot the ACTUAL data supports (e.g. "accuracy vs. training steps,
line per method"), and the exact input/ source file(s) that back it. Only list figures the real
data can produce.

### `## Gaps`
{gaps_block}

### `## Existing draft`
{"An existing draft/paper is present in input/. Assess it: what it claims, which claims are"
 " evidence-backed vs. unsupported, its structure, and what to keep vs. rebuild."
 if has_draft else
 "State whether any prose draft exists in input/. If none, write 'No existing draft.'"}

### `## Citation inventory`
Every bibliographic entry or cited work you found in input/ (from .bib files, reference lists, or
inline citations): list a short identifier (author/year or bib key) and the title if available.
If none exist, write 'No citations found in input/.'

## Step 4 — Also report a structured summary (this is your schema output)
Return an `EvidenceSummary` with these fields:
- `fact_count`: the number of `### E<n>` entries you wrote.
- `figure_candidates`: the list of figure-candidate descriptions.
- `gaps`: the list of gap items (same text as the `## Gaps` bullets: scope notes in position
  mode, TODO-phrased missing experiments in propose mode).
- `existing_citations`: the identifiers from your `## Citation inventory`.
- `draft_assessment`: a 1–3 sentence assessment of the existing draft (or note that none exists).
- `strongest_factual_thesis`: the single most defensible, evidence-backed thesis this material
  supports, stated in one sentence and grounded only in the Facts above.
- `confident`: true only if you actually read the material and the EVIDENCE.md ledger is complete.

Obey AGENTS.md in this workspace. Do the work now: explore, compute, then write EVIDENCE.md."""


@router.reasoner()
async def build_evidence_ledger(
    workspace: dict, mode: str = "position", model: str | None = None
) -> EvidenceSummary:
    """P0b: one OpenCode harness pass that reads input/ and writes EVIDENCE.md."""
    root = workspace["root"]
    evidence_path = workspace["evidence_path"]
    todo_path = workspace["todo_path"]

    print(f"[intake] building evidence ledger at {evidence_path} (mode={mode!r})")
    result = await router.harness(
        _evidence_prompt(workspace, mode),
        provider="opencode",
        model=helpers.opencode_model(model),
        cwd=root,
        project_dir=root,
        schema=EvidenceSummary,
    )

    # never trust the transcript — verify the on-disk effect.
    on_disk = helpers.read_text(evidence_path)
    if result.is_error or result.parsed is None or not os.path.exists(evidence_path) or len(on_disk) < 500:
        reason = result.error_message if result.is_error else "EVIDENCE.md missing, too small, or unparsed"
        print(f"[intake] evidence ledger FAILED: {reason}")
        return helpers.safe_ai_fallback(
            EvidenceSummary,
            draft_assessment=f"Evidence intake failed: {reason}",
        )

    summary: EvidenceSummary = result.parsed
    # In position mode, gaps are scope notes recorded in EVIDENCE.md only; they are NOT
    # appended to TODO.md as action items. In propose mode, gaps are experiment TODOs.
    if summary.gaps and mode != "position":
        helpers.append_todos(todo_path, summary.gaps)
    print(f"[intake] evidence ledger done facts={summary.fact_count} "
          f"figure_candidates={len(summary.figure_candidates)} gaps={len(summary.gaps)}")
    return summary
