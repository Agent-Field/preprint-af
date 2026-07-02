"""
fig5_paws.py -- External validation on PAWS (ROC-AUC bar chart).

Source: EVIDENCE.md, section "## SERVE on a STANDARD EXTERNAL BENCHMARK: PAWS
(serve_paws.py)".

Quote (EVIDENCE.md lines 64-70):
  RESULT: fixed (raw top-1 cosine similarity) AUC = 0.1408 -- WORSE than random
  (0.5) ... Sign-corrected fixed baseline (serve on LOW similarity, an
  unrealistic manual correction requiring advance knowledge the relationship
  is inverted): AUC = 0.8592. SERVE (trained normally, no sign knowledge, no
  manual correction): AUC = 0.9025, beating even the hand-corrected
  oracle-sign single-feature baseline by +0.0433 absolute.

No per-seed spread is reported for this PAWS comparison in EVIDENCE.md, so no
error bars are drawn (single held-out test split, not a multi-seed sweep).
"""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import matplotlib.pyplot as plt

from _style import COLUMN_FIGSIZE, COLORS, apply_style, savefig

# ---------------------------------------------------------------------------
# Hardcoded, verified numbers -- EVIDENCE.md, "SERVE on a STANDARD EXTERNAL
# BENCHMARK: PAWS (serve_paws.py)" block.
# ---------------------------------------------------------------------------
CHANCE_AUC = 0.5

METHODS = [
    "Fixed threshold\n(raw cosine sim.)",
    "Sign-corrected\noracle baseline",
    "SERVE",
]
AUC_VALUES = [0.1408, 0.8592, 0.9025]
BAR_COLORS = [COLORS["fixed"], COLORS["oracle"], COLORS["serve"]]

apply_style()

fig, ax = plt.subplots(figsize=COLUMN_FIGSIZE)

x = range(len(METHODS))
bars = ax.bar(
    x,
    AUC_VALUES,
    width=0.55,
    color=BAR_COLORS,
    edgecolor="black",
    linewidth=0.6,
    zorder=3,
)

# Chance reference line.
ax.axhline(
    CHANCE_AUC,
    color=COLORS["chance"],
    linestyle="--",
    linewidth=1.1,
    zorder=2,
    label="Chance (AUC = 0.5)",
)

# Value labels above/below each bar.
for xi, v in zip(x, AUC_VALUES):
    if v >= CHANCE_AUC:
        ax.text(xi, v + 0.025, f"{v:.4f}", ha="center", va="bottom", fontsize=7)
    else:
        ax.text(xi, v + 0.025, f"{v:.4f}", ha="center", va="bottom", fontsize=7)

ax.set_xticks(list(x))
ax.set_xticklabels(METHODS, fontsize=7)
ax.set_ylabel("ROC-AUC on PAWS")
ax.set_ylim(0, 1.05)
ax.set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])

ax.set_axisbelow(True)
ax.yaxis.grid(True, zorder=0)
ax.xaxis.grid(False)

for spine in ("top", "right"):
    ax.spines[spine].set_visible(False)

ax.legend(loc="upper left", handlelength=1.6, fontsize=7)

savefig(fig, os.path.join(os.path.dirname(os.path.abspath(__file__)), "fig5_paws.pdf"))

# Re-render for PNG (savefig() closes the figure, so build it again).
apply_style()
fig, ax = plt.subplots(figsize=COLUMN_FIGSIZE)
bars = ax.bar(
    x,
    AUC_VALUES,
    width=0.55,
    color=BAR_COLORS,
    edgecolor="black",
    linewidth=0.6,
    zorder=3,
)
ax.axhline(
    CHANCE_AUC,
    color=COLORS["chance"],
    linestyle="--",
    linewidth=1.1,
    zorder=2,
    label="Chance (AUC = 0.5)",
)
for xi, v in zip(x, AUC_VALUES):
    ax.text(xi, v + 0.025, f"{v:.4f}", ha="center", va="bottom", fontsize=7)
ax.set_xticks(list(x))
ax.set_xticklabels(METHODS, fontsize=7)
ax.set_ylabel("ROC-AUC on PAWS")
ax.set_ylim(0, 1.05)
ax.set_yticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
ax.set_axisbelow(True)
ax.yaxis.grid(True, zorder=0)
ax.xaxis.grid(False)
for spine in ("top", "right"):
    ax.spines[spine].set_visible(False)
ax.legend(loc="upper left", handlelength=1.6, fontsize=7)

fig.tight_layout(pad=0.4)
fig.savefig(
    os.path.join(os.path.dirname(os.path.abspath(__file__)), "fig5_paws.png"),
    dpi=300,
    bbox_inches=None,
)
plt.close(fig)

print("Saved fig5_paws.pdf and fig5_paws.png")
