"""
fig4_best_regime_grid.py -- Recurrence x false-serve-budget grid heatmap.

Summarizes the operating regimes where SERVE achieves the largest hit-rate
gains over the fixed-threshold baseline, providing guidance for deployment.
Gain intensifies monotonically toward strict budgets and high recurrence.

Data source: input/figures/fig4_best_regime_grid.py (hardcoded grid values).
Evidence: EVIDENCE.md E7 (recurrence x budget grid discussion).
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(
    os.path.abspath(__file__)))), "input", "figures"))

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap, Normalize
import numpy as np

from _style import COLORS, apply_style, savefig

HERE = os.path.dirname(os.path.abspath(__file__))

# E7: recurrence sweep at matched false-serve budgets
RECURRENCE_LABELS = ["0.5", "0.7", "0.8", "0.9"]
BUDGET_LABELS = ["0.25%", "0.5%", "1%"]

# E7: SERVE relative gain (%) over fixed threshold.
# Rows: r in {0.5, 0.7, 0.8, 0.9}, cols: beta in {0.25%, 0.5%, 1%}.
GAIN_PCT = np.array([
    [154.4,  82.8,  41.7],   # E7: r=0.5 row
    [166.8, 120.3,  64.4],   # E7: r=0.7 row
    [205.3, 141.9,  73.3],   # E7: r=0.8 row
    [205.2, 140.1,  77.8],   # E7: r=0.9 row
])

CMAP = LinearSegmentedColormap.from_list(
    "serve_grid",
    [COLORS["grid_lo"], COLORS["grid_hi"]],
)

# E7: best cell +205.3% at r=0.8, beta<=0.25% (fixed=0.0231, SERVE=0.0707, 3.15x)
BEST_I, BEST_J = np.unravel_index(np.argmax(GAIN_PCT), GAIN_PCT.shape)


def build_figure():
    apply_style()
    fig, ax = plt.subplots(figsize=(3.5, 2.8))

    norm = Normalize(vmin=20.0, vmax=220.0)
    im = ax.imshow(GAIN_PCT, cmap=CMAP, norm=norm, aspect="auto")

    ink = "#141414"
    for i in range(GAIN_PCT.shape[0]):
        for j in range(GAIN_PCT.shape[1]):
            val = GAIN_PCT[i, j]
            rgba = CMAP(norm(val))
            lum = 0.299 * rgba[0] + 0.587 * rgba[1] + 0.114 * rgba[2]
            tcol = "white" if lum < 0.50 else ink
            weight = "bold" if (i == BEST_I and j == BEST_J) else "normal"
            ax.text(j, i, f"+{val:.1f}%", ha="center", va="center",
                    fontsize=8.5, color=tcol, weight=weight)

    # E7: highlight the single best cell
    rect = plt.Rectangle(
        (BEST_J - 0.5, BEST_I - 0.5), 1, 1,
        fill=False, edgecolor=ink, linewidth=1.5,
    )
    ax.add_patch(rect)

    ax.set_xticks(range(len(BUDGET_LABELS)))
    ax.set_xticklabels(BUDGET_LABELS)
    ax.set_yticks(range(len(RECURRENCE_LABELS)))
    ax.set_yticklabels(RECURRENCE_LABELS)
    ax.set_xlabel(r"False-serve budget $\beta$")
    ax.set_ylabel(r"Query recurrence $r$")
    ax.tick_params(length=0)

    ax.set_xticks(np.arange(-0.5, len(BUDGET_LABELS), 1), minor=True)
    ax.set_yticks(np.arange(-0.5, len(RECURRENCE_LABELS), 1), minor=True)
    ax.grid(which="minor", color="white", linewidth=1.2)
    ax.grid(which="major", visible=False)
    ax.tick_params(which="minor", length=0)
    for spine in ax.spines.values():
        spine.set_visible(False)

    cbar = fig.colorbar(im, ax=ax, fraction=0.046, pad=0.04)
    cbar.set_label("SERVE gain over fixed (% rel. hit rate)", fontsize=7.5)
    cbar.ax.tick_params(labelsize=7, length=0)
    cbar.outline.set_visible(False)

    ax.set_title("Best regime: strict budget + high recurrence",
                 fontsize=8.5, color=ink, pad=6)
    return fig


fig = build_figure()
savefig(fig, os.path.join(HERE, "fig4_best_regime_grid.pdf"))

fig = build_figure()
fig.tight_layout(pad=0.4)
fig.savefig(os.path.join(HERE, "fig4_best_regime_grid.png"), dpi=300, bbox_inches=None)
plt.close(fig)

print("Saved fig4_best_regime_grid.pdf and fig4_best_regime_grid.png")
