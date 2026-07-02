"""
fig4_best_regime_grid.py -- Recurrence x false-serve-budget grid.

Heatmap of SERVE's relative hit-rate gain over the fixed threshold across a
4x3 grid crossing query recurrence r against false-serve budget beta,
4 seeds per cell. Both axes compound toward the strictest-budget cell
among the high-recurrence rows.

Source of truth: EVIDENCE.md, section
"SERVE best-regime grid: recurrence x strict false-serve budget
(serve_best_regime.py)" (also reproduced in main.tex Table tab:grid).

  r=0.5: fs<=0.25%=+154.4% fs<=0.5%=+82.8% fs<=1%=+41.7%
  r=0.7: fs<=0.25%=+166.8% fs<=0.5%=+120.3% fs<=1%=+64.4%
  r=0.8: fs<=0.25%=+205.3% fs<=0.5%=+141.9% fs<=1%=+73.3%
  r=0.9: fs<=0.25%=+205.2% fs<=0.5%=+140.1% fs<=1%=+77.8%

  BEST CELL: +205.3% at r=0.8, fs<=0.25% (fixed=0.0231, SERVE=0.0707),
  matching EVIDENCE.md's stated best-cell figure and main.tex Table
  tab:grid.
"""

import os
import numpy as np
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap

from _style import COLUMN_WIDTH_IN, COLORS, apply_style, savefig

# ---------------------------------------------------------------------------
# Hardcoded verified numbers -- EVIDENCE.md "SERVE best-regime grid" block
# (serve_best_regime.py, 4 seeds per cell); also main.tex Table tab:grid.
# ---------------------------------------------------------------------------
RECURRENCE_LABELS = ["r=0.5", "r=0.7", "r=0.8", "r=0.9"]
BUDGET_LABELS = [r"$\beta\leq0.25\%$", r"$\beta\leq0.5\%$", r"$\beta\leq1\%$"]

# Rows = recurrence (top->bottom: 0.5, 0.7, 0.8, 0.9), cols = false-serve
# budget (strict->loose: 0.25%, 0.5%, 1%). Values = SERVE relative
# hit-rate gain over the fixed threshold (%).
GAIN_PCT = np.array([
    [154.4, 82.8, 41.7],   # r=0.5
    [166.8, 120.3, 64.4],  # r=0.7
    [205.3, 141.9, 73.3],  # r=0.8
    [205.2, 140.1, 77.8],  # r=0.9
])

CMAP = LinearSegmentedColormap.from_list(
    "serve_grid", [COLORS["grid_lo"], COLORS["grid_hi"]]
)
VMAX = 220.0
BEST_I, BEST_J = np.unravel_index(np.argmax(GAIN_PCT), GAIN_PCT.shape)


def build_figure():
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 3.0))
    im = ax.imshow(GAIN_PCT, cmap=CMAP, vmin=0, vmax=VMAX, aspect="auto")

    # Annotate each cell with its value; bold the single best cell.
    for i in range(GAIN_PCT.shape[0]):
        for j in range(GAIN_PCT.shape[1]):
            val = GAIN_PCT[i, j]
            frac = val / VMAX
            text_color = "white" if frac > 0.55 else "black"
            weight = "bold" if (i == BEST_I and j == BEST_J) else "normal"
            ax.text(j, i, f"+{val:.1f}%", ha="center", va="center",
                     fontsize=7.5, color=text_color, weight=weight)

    ax.set_xticks(range(len(BUDGET_LABELS)))
    ax.set_xticklabels(BUDGET_LABELS)
    ax.set_yticks(range(len(RECURRENCE_LABELS)))
    ax.set_yticklabels(RECURRENCE_LABELS)
    ax.set_xlabel("False-serve budget $\\beta$")
    ax.set_ylabel("Query recurrence $r$")

    # Highlight the best cell with a distinct border.
    rect = plt.Rectangle((BEST_J - 0.5, BEST_I - 0.5), 1, 1,
                          fill=False, edgecolor="black", linewidth=1.4)
    ax.add_patch(rect)

    ax.set_xticks(np.arange(-0.5, len(BUDGET_LABELS), 1), minor=True)
    ax.set_yticks(np.arange(-0.5, len(RECURRENCE_LABELS), 1), minor=True)
    ax.grid(which="minor", color="white", linewidth=1.2)
    ax.grid(which="major", visible=False)
    ax.tick_params(which="minor", length=0)

    cbar = fig.colorbar(im, ax=ax, fraction=0.046, pad=0.04)
    cbar.set_label("SERVE gain over fixed (% rel. hit rate)", fontsize=7.5)
    cbar.ax.tick_params(labelsize=7)

    ax.set_title("Both axes compound toward the strictest budget\namong the high-recurrence rows",
                 fontsize=8.5)
    return fig


if __name__ == "__main__":
    out_dir = os.path.dirname(os.path.abspath(__file__))

    apply_style()
    fig_pdf = build_figure()
    savefig(fig_pdf, f"{out_dir}/fig4_best_regime_grid.pdf")

    apply_style()
    fig_png = build_figure()
    fig_png.tight_layout(pad=0.4)
    fig_png.savefig(f"{out_dir}/fig4_best_regime_grid.png", dpi=300, bbox_inches=None)
    plt.close(fig_png)

    print("Saved fig4_best_regime_grid.pdf and .png")
