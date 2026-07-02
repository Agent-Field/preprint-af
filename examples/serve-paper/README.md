# Example input: SERVE

A realistic research folder for `preprint-af`: a rough draft plus the figure scripts,
with no compiled output. This is the "existing paper to polish" case.

```
serve-paper/
├── main.tex           # rough LaTeX draft
├── refs.bib           # bibliography the author already has
└── figures/           # matplotlib scripts that reproduce each figure from the reported numbers
    ├── _style.py
    ├── fig1_sota_ablation.py
    ├── fig2_embedding_quality.py
    ├── fig3_recurrence.py
    ├── fig4_best_regime_grid.py
    └── fig5_paws.py
```

`preprint-af` never edits this folder. It copies it into a run workspace under `.tmp/runs/<id>/input/`,
builds an evidence ledger from it, then writes a fresh paper in `.tmp/runs/<id>/paper/`.

Run it with `examples/payload.json`:

```bash
curl -sS -X POST http://localhost:8080/api/v1/execute/async/preprint-af.write_paper \
  -H 'Content-Type: application/json' \
  -d @examples/payload.json | jq -r '.execution_id'
```
