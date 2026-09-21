package prompts

import (
	"strconv"
	"strings"
)

// EvidencePrompt is the exact harness prompt used to build EVIDENCE.md.
func EvidencePrompt(workspace Workspace, figurePython string) string {
	listing := "  (input listing unavailable — walk input/ yourself)"
	if len(workspace.InputFiles) > 0 {
		lines := make([]string, len(workspace.InputFiles))
		for i, path := range workspace.InputFiles {
			lines[i] = "  - input/" + path
		}
		listing = strings.Join(lines, "\n")
	}
	draft := "State whether any prose draft exists in input/. If none, write 'No existing draft.'"
	if workspace.HasExistingDraft {
		draft = "An existing draft/paper is present in input/. Assess it: what it claims, which claims are evidence-backed vs. unsupported, its structure, and what to keep vs. rebuild."
	}
	evidencePath := workspace.EvidencePath
	if evidencePath == "" {
		evidencePath = "EVIDENCE.md"
	}
	return fill(`You are building the FACT LEDGER for a scientific paper. The workspace root is your working
directory. All of the user's raw material lives under §input/§. Your single deliverable is a
file named §EVIDENCE.md§ written at the workspace root (path: §{0}§).

## Step 1 — Explore input/ exhaustively
Do not skim. Actually open and read the material. Depending on what is present, inspect:
- data files: .csv, .tsv, .json, .jsonl, .parquet, .npz, .npy, .pkl, .h5 — load them and look
  at columns, shapes, row counts, ranges, summary statistics;
- notebooks (.ipynb): read code cells AND their recorded outputs (metrics, tables, printed numbers);
- existing drafts or papers (.tex, .md, .txt, .rst) and any PDFs — read them fully;
- figure/plot scripts (.py, .R, .m) and the data they consume;
- logs, results dumps, config/hyperparameter files, README/notes;
- bibliography files (.bib) and any inline reference lists.
There are {1} known files under input/:
{2}

## Step 2 — Compute, never invent
NEVER invent, round-trip-guess, or extrapolate a number. Every quantitative statement must come
from a file you actually read. If a value is trivially derivable from the data (a mean, a max, a
count, a delta, a percentage, a ratio), COMPUTE it with the Python interpreter at:
    {3}
Run small scripts (e.g. §{3} -c "..."§ or a temp .py file) that load the real data files and
print the value. Record HOW each derived number was computed. If something cannot be derived from
the material present, it is a Gap, not a Fact.

## Step 3 — Write EVIDENCE.md with EXACTLY this section structure

### §## Facts§
One §### E<n>§ subheading per fact, numbered E1, E2, E3, ... In each entry give, on their own lines:
- **Statement:** the claim in one sentence.
- **Numbers:** the exact values (with units), copied or computed — never approximated.
- **Source:** the exact file path(s) under input/ the fact comes from.
- **Derivation:** how it was obtained (direct read, or the exact computation / command used).

### §## Figure candidates§
Bullet list. Each item: what plot the ACTUAL data supports (e.g. "accuracy vs. training steps,
line per method"), and the exact input/ source file(s) that back it. Only list figures the real
data can produce.

### §## Gaps§
Bullet list of experiments or data that are MISSING for stronger or additional claims. Phrase
every item as an actionable TODO (start with a verb), e.g. "Run the ablation without component X
to isolate its contribution." These become tracked TODO items.

### §## Existing draft§
{4}

### §## Citation inventory§
Every bibliographic entry or cited work you found in input/ (from .bib files, reference lists, or
inline citations): list a short identifier (author/year or bib key) and the title if available.
If none exist, write 'No citations found in input/.'

## Step 4 — Also report a structured summary (this is your schema output)
Return an §EvidenceSummary§ with these fields:
- §fact_count§: the number of §### E<n>§ entries you wrote.
- §figure_candidates§: the list of figure-candidate descriptions.
- §gaps§: the list of TODO-phrased gap items (same text as the §## Gaps§ bullets).
- §existing_citations§: the identifiers from your §## Citation inventory§.
- §draft_assessment§: a 1–3 sentence assessment of the existing draft (or note that none exists).
- §strongest_factual_thesis§: the single most defensible, evidence-backed thesis this material
  supports, stated in one sentence and grounded only in the Facts above.
- §confident§: true only if you actually read the material and the EVIDENCE.md ledger is complete.

Obey AGENTS.md in this workspace. Do the work now: explore, compute, then write EVIDENCE.md.`,
		evidencePath, strconv.Itoa(len(workspace.InputFiles)), listing, figurePython, draft)
}
