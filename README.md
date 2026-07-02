<div align="center">

# preprint-af

### You do the research. `preprint-af` writes the paper. Point it at your data and results, get a submission-ready preprint. Built on [AgentField](https://github.com/Agent-Field?utm_source=github&utm_medium=readme&utm_campaign=preprint-af).

[![Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-16a34a?style=for-the-badge)](LICENSE)
[![Python](https://img.shields.io/badge/python-3.11%2B-3776AB?style=for-the-badge&logo=python&logoColor=white)](https://www.python.org/downloads/)
[![Built with AgentField](https://img.shields.io/badge/Built%20with-AgentField-0A66C2?style=for-the-badge)](https://github.com/Agent-Field?utm_source=github&utm_medium=readme&utm_campaign=preprint-af)
[![More from Agent-Field](https://img.shields.io/badge/More_from-Agent--Field-111827?style=for-the-badge&logo=github)](https://github.com/Agent-Field)

<p>
  <a href="#quick-start">Quick Start</a> •
  <a href="#how-it-works">How It Works</a> •
  <a href="#dynamic-pipeline-architecture">Architecture</a> •
  <a href="#outputs">Outputs</a> •
  <a href="examples/sample-run.md">Sample Run</a>
</p>

</div>

Most AI writing tools autocomplete plausible prose and invent citations. `preprint-af` writes a real paper from results you already have.

> **What it is:** a preprint writer, not a research agent. You run the experiments and gather the data; `preprint-af` turns that finished work into a submission-ready, evidence-grounded paper. It does not design studies, run experiments, or invent results. Where the data to support a claim does not exist, it flags a `\todobox` and a `TODO.md` entry instead of making one up.

Point it at a folder (data, result tables, notebooks, a rough draft, an existing paper) and autonomous agents build the paper around your actual numbers. An evidence ledger traces every claim back to a source. A positioning tournament locks one governing story before a section is written. Parallel agents draft each section and render figures by running your own scripts. Then an autonomous review loop critiques the whole draft (reviewer personas, a narrative critic, a fidelity auditor, and a deterministic slop linter) and self-corrects it round after round until it converges. It compiles to a submission-ready LaTeX PDF and never invents a number. Free, open source, one API call.

<p align="center">
<img src="assets/hero.png" alt="preprint-af: you do the research, it writes the paper from your data and results" width="100%" />
</p>

Real output, not a mockup: the bundled example ([`examples/serve-paper`](examples/serve-paper)) compiled by `preprint-af` into [`main.pdf`](examples/serve-paper/main.pdf). The screenshots below are rendered from the checked-in PDF: pages 1 and 2, plus page 7 where the generated figures and ablation table appear.

<p align="center">
  <a href="examples/serve-paper/main.pdf">
    <img src="assets/paper-preview/page-1.png" alt="Generated SERVE paper page 1" width="31%" />
  </a>
  <a href="examples/serve-paper/main.pdf">
    <img src="assets/paper-preview/page-2.png" alt="Generated SERVE paper page 2" width="31%" />
  </a>
  <a href="examples/serve-paper/main.pdf">
    <img src="assets/paper-preview/page-7.png" alt="Generated SERVE paper page 7 with figure and ablation table" width="31%" />
  </a>
</p>

---

## Why preprint-af

- **Writes like a scientist, not a chatbot.** Claims-first paragraphs, precise language, varied sentence rhythm. A deterministic linter strips AI tells (em dashes, hype phrases, uniform cadence) before any model spends a token judging the prose. No writing skill required on your end.
- **Rigorous and evidence-grounded.** Every quantitative claim traces to a fact in your data. A fidelity auditor fails the build on any number or citation it cannot source, so the paper cannot drift into confabulation. It never invents a result.
- **Finds the strongest story.** A positioning tournament tests five to six framings of your results and locks the one that lands, so the whole paper argues in one direction instead of listing findings.
- **Organizes the narrative flow.** A blueprint gives every section a job and a transition contract (what it must establish for the next), and a narrative critic checks that the body delivers what the title promises.
- **Tells you what would make it stronger.** A reviewer panel raises peer-review-grade objections, and `REVIEW.md` plus `TODO.md` collect the missing experiments, soft claims, and open gaps as concrete next steps.
- **One call, no setup.** Point it at a folder and get a compiled LaTeX PDF with real figures. No prompts to engineer, no template to fill.

---

## One-Call DX

```bash
curl -sS -X POST http://localhost:8080/api/v1/execute/async/preprint-af.write_paper \
  -H 'Content-Type: application/json' \
  -d '{"input": {"folder_path": "./examples/serve-paper",
                 "target_venue": "high-impact machine learning systems venue",
                 "field_hint": "machine learning systems",
                 "max_rounds": 6}}'
```

```json
{ "execution_id": "exec_...", "status": "queued" }
```

Poll the execution; the result points at the compiled paper and the run workspace:

```json
{
  "status": "succeeded",
  "result": {
    "title": "Why Semantic Caching Fails: Margin, Competition, and Local Density",
    "pdf_path": ".tmp/runs/<id>/paper/main.pdf",
    "positioning_path": ".tmp/runs/<id>/POSITIONING.md",
    "review_path": ".tmp/runs/<id>/REVIEW.md",
    "final_score": 0.66,
    "stop_reason": "quality_plateau",
    "rounds": [ { "round": 2, "total_score": 0.66, "fidelity_score": 0.93, "compile_ok": true } ]
  }
}
```

Everything a run produces lives in a single git-tracked workspace under `.tmp/runs/<id>/`, so
every phase and revision round is a commit you can diff or roll back to.

---

## Dynamic Pipeline Architecture

<p align="center">
<img src="assets/architecture.png" alt="preprint-af seven-phase paper pipeline: evidence ledger, positioning tournament, blueprint, parallel build, compile gate, critique and repair loop" width="100%" />
</p>

```mermaid
flowchart TD
    A[research folder] --> P0[P0 · Evidence ledger<br/>facts traced to your data]
    P0 --> P1[P1 · Positioning tournament<br/>run once]
    P1 --> P2[P2 · Blueprint<br/>section beats + transition contract]
    P2 --> P3[P3 · Parallel build]

    subgraph P3 [ ]
      direction LR
      S[section writers] --- F[figure builders<br/>run scripts on real data] --- B[bibliography<br/>web-verified citations]
    end

    P3 --> P4[P4 · Compile gate · latexmk]
    P4 --> LOOP{P5/P6 · converge?}
    LOOP -- personas ∥ narrative ∥ fidelity ∥ slop --> R[P6 · targeted repairs]
    R --> P4
    LOOP -- threshold / plateau / clean --> OUT[main.pdf · TODO.md · REVIEW.md]

    classDef gate fill:#fff3e0,stroke:#e65100;
    class P4,LOOP gate;
```

Seven phases, driven by [AgentField](https://github.com/Agent-Field?utm_source=github&utm_medium=readme&utm_campaign=preprint-af) reasoners for the thinking
and [OpenCode](https://opencode.ai?utm_source=github&utm_medium=readme&utm_campaign=preprint-af) coding agents for every read and write of a real file:

1. **Evidence ledger.** An agent explores your folder (CSVs, notebooks, drafts, PDFs, figure
   scripts) and writes `EVIDENCE.md`: every claimable fact with exact numbers, source paths, and
   how it was derived. Nothing enters the paper unless it traces here.
2. **Positioning tournament.** 5 to 6 genuinely different framings of the same results (speed vs
   memory vs mechanism vs reliability), each written as a concrete title and mini-abstract, judged
   in parallel by reviewer and editor personas, with a live novelty scan of related work. The
   winner is frozen into `POSITIONING.md`. This runs **once**, so the paper's story does not drift.
3. **Blueprint.** Section-by-section beats with an explicit *transition contract* (what each
   section must establish for the next), the fact ids each section may use, and a figure plan tied
   to real data files.
4. **Parallel build.** One agent per section file, one per figure (it writes the matplotlib
   script, *runs it* against your data, and verifies the PDF), and one for the bibliography. No
   merge conflicts: each agent owns one file.
5. **Compile gate.** `latexmk` must produce a PDF; failures get a targeted, LaTeX-only repair.
6. **Critique.** Reviewer personas read the *whole* paper, a narrative critic checks flow and
   whether the body delivers what the title promises, a fidelity auditor traces every number back
   to the ledger, and a deterministic linter flags AI-slop patterns.
7. **Repair and converge.** Findings route into bounded per-section edits, the paper recompiles and
   rescores, and the loop repeats until it converges.

---

## How It Works

### Evidence grounding, not confabulation
Every quantitative claim in the paper must trace to a fact id in `EVIDENCE.md`. The fidelity
auditor runs each round and **fails closed**: fabricated numbers or citation keys that are absent
from `refs.bib` are caught deterministically and block convergence until fixed. Where the data to
support a claim does not exist, the writer emits a `\todobox{...}` and a `TODO.md` entry instead of
inventing a result.

### Positioning as a tournament, decided once
The same results can be told as a speed story, a memory story, a reliability story. `preprint-af`
generates those framings as concrete title/abstract artifacts, has distinct personas score them for
comprehension, credibility, and how natural they read, scans for collisions with existing work, and
picks one governing story before a single section is written, so the whole paper pulls in one
direction.

### Prose that does not read like a machine
A frozen style contract ships in each workspace as `AGENTS.md` (auto-read by every OpenCode agent):
no em dashes, a banned-phrase list, claims-first paragraphs, varied sentence rhythm, minimal
headings. A deterministic linter enforces it: em-dash census, banned lexicon, "not only, but also"
constructions, heading density, and uniform-rhythm detection, so slop is caught mechanically before any
model tokens are spent judging it.

### Convergence, not a fixed number of passes
The revision loop stops when the paper is actually done: the quality threshold is met with a clean
compile and no fidelity blockers, or the score plateaus across rounds, or there is nothing left to
repair. `max_rounds` is only a safety cap. Because OpenCode enforces no turn or budget limits, the
loop budget is enforced in Python.

---

## Quick Start

### Host-native (recommended)

The compile gate needs a real LaTeX toolchain, and figure scripts need Python. Running on the host
uses your local TeX Live directly and lets the node read your real paper folders without volume
mounts.

**Prerequisites:** the [`af`](https://github.com/Agent-Field?utm_source=github&utm_medium=readme&utm_campaign=preprint-af) CLI, the [`opencode`](https://opencode.ai?utm_source=github&utm_medium=readme&utm_campaign=preprint-af)
CLI, a LaTeX toolchain (`latexmk` + `pdflatex`, e.g. MacTeX or TeX Live), and an
`OPENROUTER_API_KEY`.

```bash
git clone <this-repo> && cd preprint-af
cp .env.example .env                 # set OPENROUTER_API_KEY
python -m venv .venv && .venv/bin/pip install -r requirements.txt

af server &                          # control plane on :8080
PATH="$HOME/.opencode/bin:$PATH" .venv/bin/python main.py   # node on :8001
```

Confirm the node registered:

```bash
curl -fsS http://localhost:8080/api/v1/discovery/capabilities \
  | jq '.capabilities[] | select(.agent_id=="preprint-af") | .reasoners[].id'
```

Run the bundled example (an existing draft to polish):

```bash
EXEC=$(curl -sS -X POST http://localhost:8080/api/v1/execute/async/preprint-af.write_paper \
  -H 'Content-Type: application/json' -d @examples/payload.json | jq -r '.execution_id')

curl -sS http://localhost:8080/api/v1/executions/$EXEC | jq '{status, result}'
```

### Docker

```bash
cp .env.example .env                 # set OPENROUTER_API_KEY
docker compose up --build
```

The image bundles TeX Live and OpenCode. Mount the folder you want to write about and pass its
in-container path as `folder_path`.

---

## Inputs

`preprint-af.write_paper` accepts:

| field | default | meaning |
|---|---|---|
| `folder_path` | _required_ | Folder with your research: data, results, notebooks, a draft, or a paper. |
| `target_venue` | `null` | Target journal/conference; shapes structure and positioning. |
| `field_hint` | `null` | Field, e.g. `"machine learning systems"`. |
| `max_rounds` | `6` | Safety cap on critique/repair rounds after the first build. |
| `allow_web` | `true` | Allow web lookups for real, verifiable citations and a novelty scan. |
| `dry_run` | `false` | Stop after evidence + positioning + blueprint; write no paper. |
| `quality_threshold` | `0.90` | Score at which the loop may converge. |
| `model` | DeepSeek v4 Pro | Any LiteLLM-style `openrouter/…` model for both reasoning and OpenCode. |

Set `dry_run: true` for a cheap preview of the strategy (the winning title, abstract, and section
plan) before committing to a full write.

---

## Outputs

A run workspace under `.tmp/runs/<id>/`:

```
EVIDENCE.md        fact ledger with provenance
POSITIONING.md     winning frame, title, abstract, rejected alternatives
BLUEPRINT.md       section beats and transition contract
paper/main.pdf     the compiled paper
paper/             main.tex, sections/, figures/ (scripts + rendered PDFs), refs.bib
reviews/round_N/   every persona, narrative, fidelity, and slop report
TODO.md            missing experiments and unresolvable citations
REVIEW.md          honest unresolved problems at stop time
```

---

## Development

`docs/ARCHITECTURE.md` is the binding module contract; `CLAUDE.md` has the working rules and the
verified AgentField/OpenCode behaviors the code depends on.

```bash
.venv/bin/python -m py_compile main.py src/preprint_af/reasoners/*.py
```

---

## About

Built on [AgentField](https://github.com/Agent-Field?utm_source=github&utm_medium=readme&utm_campaign=preprint-af) (reasoner orchestration, workflow provenance)
and [OpenCode](https://opencode.ai?utm_source=github&utm_medium=readme&utm_campaign=preprint-af) (file-editing coding agents). Default model:
`openrouter/deepseek/deepseek-v4-pro`. Apache 2.0.
