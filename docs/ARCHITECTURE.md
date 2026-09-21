# preprint-af architecture

This document is the binding contract for the Go implementation. The node turns an existing
research folder into a compiled, evidence-grounded paper. It does not run experiments or invent
results.

## Runtime

- One AgentField node: `preprint-af`.
- One stripped Go binary built from `go/cmd/preprint-af`.
- Nineteen registered reasoners. Every cross-reasoner edge is tracked by AgentField.
- Direct structured reasoning uses the AgentField Go SDK AI client.
- File-reading, section-writing, bibliography, figure, LaTeX-repair, and paper-repair work uses
  OpenCode through the AgentField harness.
- Figure workers generate Python/Matplotlib programs as paper artifacts. Python is a plotting
  subprocess only; it is not an AgentField node implementation.
- `latexmk` and `pdflatex` are mandatory compile gates.

The default model is `openrouter/deepseek/deepseek-v4-pro`. `AI_MODEL` and `OPENCODE_MODEL`
override it. `FIGURE_PYTHON` selects the interpreter used by generated plotting programs.

## Source layout

```text
go/
  cmd/preprint-af/       process entrypoint
  internal/afx/          AgentField result binding
  internal/node/         node construction, schemas, 19 reasoner registrations
  internal/pipeline/     orchestration, models, filesystem and compile operations
  internal/prompts/      exact prompt contracts and immutable reference goldens
  testdata/latex-smoke/  container compile fixture
```

`go/internal/node/register.go` owns the public AgentField surface. `go/internal/pipeline` owns
control flow. `go/internal/prompts` owns prompt bytes. Keep those responsibilities separate.

## Public reasoners

| Phase | Reasoners |
|---|---|
| Intake | `intake_prepare_workspace`, `intake_build_evidence_ledger` |
| Positioning | `positioning_generate_frames`, `positioning_judge_frame`, `positioning_scan_novelty`, `positioning_run_positioning` |
| Blueprint | `blueprint_design_blueprint` |
| Build | `build_write_section`, `build_build_figure`, `build_build_bibliography`, `build_run_build` |
| Compile | `latex_compile_paper` |
| Critique | `critique_persona_review`, `critique_narrative_review`, `critique_fidelity_audit`, `critique_run_critique` |
| Repair | `repair_plan_repairs`, `repair_apply_repairs` |
| Entry | `write_paper` |

`write_paper` accepts `folder_path`, `target_venue`, `field_hint`, `max_rounds`, `allow_web`,
`dry_run`, `quality_threshold`, `plateau_delta`, and `model`.

## Call graph

```text
write_paper
  intake_prepare_workspace
  intake_build_evidence_ledger
  positioning_run_positioning
    positioning_generate_frames
    positioning_judge_frame × selected frames × four personas  (parallel)
    positioning_scan_novelty                             (parallel with judging)
  blueprint_design_blueprint
  build_run_build
    build_build_bibliography
    build_write_section × sections                      (parallel)
    build_build_figure × buildable figures              (parallel)
  latex_compile_paper
  critique/repair loop × max_rounds
    critique_run_critique
      critique_persona_review × four personas            (parallel)
      critique_narrative_review                           (parallel)
      critique_fidelity_audit                             (parallel)
      deterministic slop lint
    repair_plan_repairs
    repair_apply_repairs
      front matter first when present
      independent section/figure/bibliography tasks       (parallel)
    latex_compile_paper
```

Positioning runs once. The critique/repair loop stops when the quality threshold is met with a
clean compile and no fidelity blocker, when quality plateaus, when no repairs remain, or when
`max_rounds` is reached.

## Phase contracts

### Intake

Copy the source folder into an isolated run workspace, initialize git, inventory input files, and
scaffold paper directories. The evidence worker writes `EVIDENCE.md` with claimable facts,
provenance, figure candidates, and explicit gaps. Quantitative derivations must be computed with
the configured plotting interpreter.

### Positioning

Generate five to six evidence-supported frames, rank them, and retain the top four. Judge every
retained frame from four fixed reader perspectives while a novelty worker checks nearby real
literature. A meta-editor selects one title, abstract, opening thesis, and contribution order and
writes `POSITIONING.md`. Any judge or novelty boundary failure fails the phase after the parallel
fan-out completes.

### Blueprint

Produce ordered section specifications, transition contracts, evidence IDs, citation needs, and
figure specifications. Every buildable figure must name real input files. Write `BLUEPRINT.md`
and the LaTeX skeleton. There is no fabricated fallback blueprint.

### Build

Bibliography preparation starts before the section and figure fan-out. Every section worker owns
one `.tex` file. Every buildable figure worker owns one `.py` program and its `.pdf`/`.png`
outputs. Missing evidence becomes `\todobox{...}` and `TODO.md`, never an invented result.

### Compile

Run `latexmk` for at most three attempts. A failed attempt may invoke one LaTeX-only repair worker;
that worker may repair compilation but may not change scientific content, numbers, citations, or
prose meaning. The Go compile gate requires a zero `latexmk` exit status; it deliberately rejects
a partial PDF left behind by a non-zero build.

### Critique and repair

Four reviewer personas, a narrative reviewer, and a fidelity auditor read the assembled paper in
parallel. A deterministic slop linter runs locally. Fidelity blocks convergence on fabricated
numbers, unsupported material claims, or citation keys absent from `refs.bib`.

The repair planner emits at most eight tasks and groups edits by writable target. Front matter is
repaired alone before the remaining independent targets fan out. Each round recompiles and records
its artifacts and score.

## Workspace contract

```text
.tmp/runs/<id>/
  input/
  EVIDENCE.md
  POSITIONING.md
  BLUEPRINT.md
  TODO.md
  REVIEW.md
  STATE.json
  paper/
    main.tex
    main.pdf
    refs.bib
    sections/
    figures/
  reviews/round_<N>/
```

The workspace is initialized as a git repository. Phase and round snapshots make changes
inspectable and recoverable.

## Invariants

1. No quantitative claim without evidence provenance or a visible TODO.
2. No invented paper, author, title, DOI, URL, or BibTeX key.
3. No untracked same-node shortcut for ordinary composition; child calls cross AgentField so the
   workflow DAG remains visible. The scalar `repair_apply_repairs` result uses the SDK's tracked
   local compatibility path because the current Go call decoder expects object results.
4. Parallelize independent work; keep bibliography ordering, front-matter repair, compile gates,
   and critique rounds sequential where dependencies require it.
5. Bound every fan-out and loop: four positioning frames, four personas, ten concurrent OpenCode
   workers by default, eight repair tasks, three compile attempts, and request-scoped rounds.
6. Prompt text and discovery schemas are API contracts. Golden fixtures under package-local
   `testdata/` were captured from the final reference implementation and require intentional
   review to change.
7. Quality-threshold convergence requires a compiled PDF and no fidelity blocker. Plateau,
   no-repair, or safety-cap stops remain honest terminal outcomes and return an empty `pdf_path`
   when the final compile did not pass.

## Packaging and verification

`docker compose up --build` starts the control plane and Go node. `go/Dockerfile` bundles the
binary, OpenCode, TeX Live, and the scientific Python plotting stack. The image build renders a
Matplotlib fixture and compiles a LaTeX fixture.

```bash
cd go
make check
make size
cd ..
docker compose config
```

The repository must contain no second AgentField implementation. Tracked Python files are allowed
only as example or generated figure programs.
