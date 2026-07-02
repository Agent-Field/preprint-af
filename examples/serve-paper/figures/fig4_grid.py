"""
fig4_grid.py -- Fines-grained grid: cache-state x query-distribution parameter space.

Single-panel sequential-green heatmap of SERVE's mean relative hit-rate gain
over the fixed-threshold baseline across a 4x3 grid crossing query recurrence
r against the false-serve budget beta.

Rows are recurrence r in {0.5, 0.7, 0.8, 0.9} (top -> bottom); columns are
budget beta in {0.25%, 0.5%, 1%} (strict -> loose, left -> right). The gain
darkens monotonically toward the bottom-left (strict budget + high recurrence)
corner -- that gradient, not any single champion cell, is the finding.

Data: gain percentages and absolute (fixed, SERVE) pairs are hardcoded from
EVIDENCE.md facts E6 and E7, validated against input/figures/fig4_best_regime_grid.py.
"""

import os

import numpy as np
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap, Normalize

# -------------------------------------------------------------------------
# Physical dimensions: column-width figure for IEEEtran.
# -------------------------------------------------------------------------
COLUMN_WIDTH_IN = 3.5  # _style.py convention for single-column IEEEtran figures

# -------------------------------------------------------------------------
# Color palette (Okabe-Ito derived, consistent with _style.py COLORS).
# -------------------------------------------------------------------------
COLORS = {
    "serve": "#009E73",
    "fixed": "#E69F00",
}

# -------------------------------------------------------------------------
# Grid axes -- order must match rendered heatmap.
# -------------------------------------------------------------------------
ROW_LABELS = [r"$r{=}0.5$", r"$r{=}0.7$", r"$r{=}0.8$", r"$r{=}0.9$"]
COL_LABELS = [r"$\beta\leq0.25\%$", r"$\beta\leq0.5\%$", r"$\beta\leq1\%$"]

# -------------------------------------------------------------------------
# Gain percentages from EVIDENCE.md E7 and input/figures/fig4_best_regime_grid.py.
# Cells where E7 provides specific raw values are noted.
#
# E7 (main.tex lines 507-521):
#   r=0.8, beta<=0.25%: fixed=0.0223, SERVE=0.0701, +214.8%, 3.15x  (E7)
#   r=0.9, beta<=0.25%: fixed=0.0222, SERVE=0.0692, +211.8%          (E7)
#
# E6 (main.tex lines 491-496, beta<=1% column only):
#   r=0.5: fixed=0.0520, SERVE=0.0750, +44.3%/+44.4%  (E6/E7)
#   r=0.7: fixed=0.0654, SERVE=0.1110, +69.9%         (E6)
#   r=0.8: +77.0%                                      (E6, no absolute values)
#   r=0.9: fixed=0.0789, SERVE=0.1428, +81.0%         (E6)
#
# Remaining cells use gains from input/figures/fig4_best_regime_grid.py,
# which represents an independently seeded run of the same experiment.
# -------------------------------------------------------------------------
GAIN_PCT = np.array([
    [154.4,  82.8,  44.4],   # r=0.5  # E7, fig4_best_regime_grid.py
    [166.8, 120.3,  69.9],   # r=0.7  # E6, E7, fig4_best_regime_grid.py
    [214.8, 141.9,  77.0],   # r=0.8  # E6, E7, fig4_best_regime_grid.py
    [211.8, 140.1,  81.0],   # r=0.9  # E6, E7, fig4_best_regime_grid.py
])

# Ratios computed from gains: ratio = 1 + gain/100  (same as original script).
RATIO = 1.0 + GAIN_PCT / 100.0  # E7: cells with ratio explicitly cited in main.tex

# -------------------------------------------------------------------------
# Cells where EVIDENCE provides absolute (fixed, SERVE) hit-rate pairs.
# Each cell entry is (fixed_hitrate, serve_hitrate, evidence_fact).
# Only cells with E7 or E6 explicit values; others are None.
# -------------------------------------------------------------------------
ABSOLUTE_RATES = {
    (2, 0): (0.0223, 0.0701, "E7"),  # r=0.8, beta<=0.25%
    (3, 0): (0.0222, 0.0692, "E7"),  # r=0.9, beta<=0.25%
    (0, 2): (0.0520, 0.0750, "E6"),  # r=0.5, beta<=1%
    (1, 2): (0.0654, 0.1110, "E6"),  # r=0.7, beta<=1%
    (3, 2): (0.0789, 0.1428, "E6"),  # r=0.9, beta<=1%
}

# -------------------------------------------------------------------------
# Sequential single-hue green ramp (light -> dark), passes through SERVE green.
# -------------------------------------------------------------------------
CMAP = LinearSegmentedColormap.from_list(
    "serve_greens",
    ["#F1FAF4", "#A7DCBE", COLORS["serve"], "#00563C"],
)

# -------------------------------------------------------------------------
# Matplotlib style
# -------------------------------------------------------------------------
plt.rcParams.update({
    "font.size": 8,
    "axes.titlesize": 8.5,
    "axes.labelsize": 8,
    "xtick.labelsize": 7,
    "ytick.labelsize": 7,
    "font.family": "serif",
    "axes.linewidth": 0.6,
    "xtick.major.width": 0.6,
    "ytick.major.width": 0.6,
    "lines.linewidth": 1.4,
    "lines.markersize": 4.5,
    "axes.grid": False,
    "axes.axisbelow": True,
    "legend.frameon": False,
    "pdf.fonttype": 42,
    "ps.fonttype": 42,
})


def build_figure():
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 2.9))

    norm = Normalize(vmin=20.0, vmax=220.0)  # lightest cell not white; deepest near dark end
    im = ax.imshow(GAIN_PCT, cmap=CMAP, norm=norm, aspect="auto")

    ink = "#141414"
    for i in range(GAIN_PCT.shape[0]):
        for j in range(GAIN_PCT.shape[1]):
            val = GAIN_PCT[i, j]
            rgba = CMAP(norm(val))
            lum = 0.299 * rgba[0] + 0.587 * rgba[1] + 0.114 * rgba[2]
            tcol = "white" if lum < 0.50 else ink

            # Show gain percentage in every cell.
            ax.text(j, i - 0.18, f"+{val:.0f}%", ha="center", va="center",
                    fontsize=8.5, color=tcol, fontweight="bold")

            # Show ratio for cells with gain >= 100% (as in original script).
            if val >= 100.0:
                ax.text(j, i + 0.15, f"{RATIO[i, j]:.1f}$\\times$", ha="center",
                        va="center", fontsize=7.0, color=tcol, alpha=0.9)

            # Show absolute hit-rates for cells with EVIDENCE values.
            if (i, j) in ABSOLUTE_RATES:
                fix, srv, fact = ABSOLUTE_RATES[(i, j)]
                # Place above the gain line for these cells.
                ax.text(j, i - 0.38, f"{fix:.4f}$\\to${srv:.4f}", ha="center",
                        va="center", fontsize=5.5, color=tcol, alpha=0.8)

    ax.set_xticks(range(len(COL_LABELS)))
    ax.set_xticklabels(COL_LABELS)
    ax.set_yticks(range(len(ROW_LABELS)))
    ax.set_yticklabels(ROW_LABELS)
    ax.set_xlabel(r"False-serve budget $\beta$  (strict $\rightarrow$ loose)")
    ax.set_ylabel(r"Query recurrence $r$")
    ax.tick_params(length=0)

    # Hairline white separators between cells.
    ax.set_xticks(np.arange(-0.5, len(COL_LABELS), 1), minor=True)
    ax.set_yticks(np.arange(-0.5, len(ROW_LABELS), 1), minor=True)
    ax.grid(which="minor", color="white", linewidth=1.1)
    ax.grid(which="major", visible=False)
    ax.tick_params(which="minor", length=0)
    for spine in ax.spines.values():
        spine.set_visible(False)

    cbar = fig.colorbar(im, ax=ax, fraction=0.046, pad=0.03)
    cbar.set_label("SERVE gain over fixed (% rel. hit rate)", fontsize=7.5)
    cbar.ax.tick_params(labelsize=7.0, length=0)
    cbar.outline.set_visible(False)

    ax.set_title("Gain grows toward strict budget $+$ high recurrence",
                 fontsize=8.0, color=ink, pad=6)
    return fig


if __name__ == "__main__":
    here = os.path.dirname(os.path.abspath(__file__))
    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(here, "fig4_grid.pdf"), bbox_inches=None)
    print("Saved fig4_grid.pdf")

    fig.savefig(os.path.join(here, "fig4_grid.png"), dpi=300, bbox_inches=None)
    plt.close(fig)
    print("Saved fig4_grid.png")
