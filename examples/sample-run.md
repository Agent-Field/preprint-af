# Sample run: `examples/serve-paper` → a paper

This is an actual run of `preprint-af` on [`examples/serve-paper`](./serve-paper) (an existing
rough draft plus figure scripts), using `openrouter/deepseek/deepseek-v4-pro` for both the
reasoners and the OpenCode agents. It shows what the pipeline produces and how the paper
*evolves* across the critique/repair loop.

## The positioning it chose

From the same evidence the tournament generated framings around speed, benchmark wins,
robustness, reliability, and mechanism. The **mechanistic** frame won:

> **Title.** Why Semantic Caching Fails: Margin, Competition, and Local Density
>
> **Opening thesis.** Two queries with identical top-1 similarity can carry very different
> false-serve risk; the local retrieval geometry, not the similarity score, tells them apart.

Rejected alternatives and the reasoning are recorded in the run's `POSITIONING.md`, alongside a
novelty scan of the closest real related work.

## How the paper evolved across rounds

The loop reviews the whole compiled paper each round (four reviewer personas, a narrative critic,
a fidelity auditor, and a deterministic slop linter), routes the findings into per-section
repairs, recompiles, and rescores. It stops when the score plateaus, not at a fixed round count.

| Round | Total | Persona | Narrative | Fidelity | Slop | Compiles | Repairs | Note |
|------:|------:|--------:|----------:|---------:|-----:|:--------:|:-------:|------|
| 1 | 0.43 | 0.16 | 0.65 | 0.30 | 0.94 | ✓ | 4 | first draft; invented-citation block trips |
| 2 | **0.66** | 0.25 | 0.78 | 0.93 | 0.98 | ✓ | 6 | citations grounded, fidelity clears |
| 3 | 0.62 | 0.22 | 0.72 | 0.85 | 0.98 | ✓ | 5 | minor regression |
| 4 | 0.56 | 0.06 | 0.65 | 0.92 | 1.00 | ✓ | 0 | plateau → **stop** |

Every row is a compiled PDF and a git commit inside the run workspace, so any round is
recoverable.

## What it flagged as still-unresolved

`REVIEW.md` is the honest part: the problems the pipeline could not fix by editing, because they
need new experiments or a narrower claim. On this paper it surfaced peer-review-grade objections,
not cosmetics, for example:

- The absolute hit-rate delta is small (0.0338 → 0.0379) even though the relative gain reads as
  +12.1%; contextualize the practical impact.
- No conformal-prediction baseline that also controls the false-serve rate.
- "Budget control" is empirical calibration on a held-out set, not a bound; soften the claim.
- Correctness labels are only available for paraphrase datasets; scope the generality claim.

Those, plus the missing experiments the evidence ledger identified, are collected in `TODO.md` as
runnable next steps.

## Reproduce it

```bash
curl -sS -X POST http://localhost:8080/api/v1/execute/async/preprint-af.write_paper \
  -H 'Content-Type: application/json' -d @examples/payload.json
```

Your exact scores and wording will differ run to run; the shape (grounded evidence, one governing
story, honest unresolved list) is the point.
