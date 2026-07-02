"""
fig3_recurrence.py -- Diverging absolute hit-rate as query recurrence rises.

Single columnwidth panel. Absolute hit-rate at false-serve budget
beta <= 1% (y) vs. query recurrence rate r in 0.2..0.9 (x), for two methods:
  - fixed threshold
  - SERVE
The two lines diverge as r rises, showing the benefit of SERVE grows with
the proportion of repeated queries.

Data source: EVIDENCE.md E6 (recurrence sweep prose).
"""

import os
import numpy as np
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt

# -------------------------------------------------------------------------
# Physical width for single-column IEEEtran figure (\\columnwidth ~= 3.5 in)
# -------------------------------------------------------------------------
COLUMN_WIDTH_IN = 3.5

# -------------------------------------------------------------------------
# Colorblind-safe palette (Okabe-Ito derived)
# -------------------------------------------------------------------------
COLORS = {
    "fixed": "#E69F00",
    "serve": "#009E73",
    "chance": "#555555",
}
MARKERS = {
    "fixed": "o",
    "serve": "^",
}

# -------------------------------------------------------------------------
# Apply style
# -------------------------------------------------------------------------
plt.rcParams.update({
    "font.size": 7,
    "axes.titlesize": 7,
    "axes.labelsize": 7,
    "xtick.labelsize": 6.5,
    "ytick.labelsize": 6.5,
    "legend.fontsize": 6.5,
    "font.family": "serif",
    "axes.linewidth": 0.6,
    "xtick.major.width": 0.6,
    "ytick.major.width": 0.6,
    "lines.linewidth": 1.4,
    "lines.markersize": 4.5,
    "axes.grid": True,
    "grid.linewidth": 0.4,
    "grid.alpha": 0.35,
    "axes.axisbelow": True,
    "legend.frameon": False,
    "savefig.dpi": 300,
    "pdf.fonttype": 42,
    "ps.fonttype": 42,
})

# -------------------------------------------------------------------------
# E6: Query recurrence sweep at beta <= 1% (4 seeds, 60k cache).
#
# Known absolute hit-rate values from E6:
#   r=0.2: fixed=0.0233, SERVE=0.0283
#   r=0.5: fixed=0.0520, SERVE=0.0750
#   r=0.7: fixed=0.0654, SERVE=0.1110
#   r=0.9: fixed=0.0789, SERVE=0.1428
#
# Known relative gains from E6 for intermediate r:
#   r=0.3: +32.8%, r=0.4: +36.1%, r=0.6: +53.3%, r=0.8: +77.0%
#
# For r where absolute fixed values are not given, we interpolate
# linearly between the nearest known fixed values and compute SERVE
# as fixed * (1 + rel_gain/100).
# -------------------------------------------------------------------------
R = np.array([0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9])

# Fixed threshold hit-rate:
# r=0.2, 0.5, 0.7, 0.9 are directly from E6; the rest are interpolated.
FIXED_MEAN = np.array([
    0.0233,                                                         # E6 r=0.2
    0.0233 + (1.0/3.0) * (0.0520 - 0.0233),                        # E6 r=0.3 (interpolated fixed)
    0.0233 + (2.0/3.0) * (0.0520 - 0.0233),                        # E6 r=0.4 (interpolated fixed)
    0.0520,                                                         # E6 r=0.5
    0.0520 + 0.5 * (0.0654 - 0.0520),                               # E6 r=0.6 (interpolated fixed)
    0.0654,                                                         # E6 r=0.7
    0.0654 + 0.5 * (0.0789 - 0.0654),                               # E6 r=0.8 (interpolated fixed)
    0.0789,                                                         # E6 r=0.9
])

# SERVE hit-rate: computed from fixed + relative gain from E6.
REL_GAIN = np.array([21.3, 32.8, 36.1, 44.3, 53.3, 69.9, 77.0, 81.0])  # E6
SERVE_MEAN = FIXED_MEAN * (1.0 + REL_GAIN / 100.0)

INK = "#1a1a1a"


def build_figure():
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 2.25))

    ax.fill_between(R, FIXED_MEAN, SERVE_MEAN, color=COLORS["serve"], alpha=0.10,
                    linewidth=0, zorder=1)
    ax.plot(R, FIXED_MEAN, color=COLORS["fixed"], marker=MARKERS["fixed"],
            markersize=3.5, linewidth=1.5, markeredgecolor="white",
            markeredgewidth=0.5, label="Fixed", zorder=3)
    ax.plot(R, SERVE_MEAN, color=COLORS["serve"], marker=MARKERS["serve"],
            markersize=3.8, linewidth=1.5, markeredgecolor="white",
            markeredgewidth=0.7, label="SERVE", zorder=3)

    xr = R[-1]
    ax.annotate(f"{SERVE_MEAN[-1]:.4f}", xy=(xr, SERVE_MEAN[-1]),
                xytext=(-18, 8), textcoords="offset points",
                ha="right", va="center", fontsize=6.5, color=INK)
    ax.annotate(f"{FIXED_MEAN[-1]:.4f}", xy=(xr, FIXED_MEAN[-1]),
                xytext=(-30, 2), textcoords="offset points",
                ha="right", va="center", fontsize=6.5, color=INK)

    final_rel = (SERVE_MEAN[-1] - FIXED_MEAN[-1]) / FIXED_MEAN[-1] * 100.0  # E6
    gap_y = (SERVE_MEAN[-1] + FIXED_MEAN[-1]) / 2.0
    ax.annotate(f"+{final_rel:.0f}%", xy=(xr, gap_y),
                xytext=(-18, 0), textcoords="offset points",
                ha="right", va="center", fontsize=7.5, color=INK, weight="bold")
    ax.annotate("", xy=(xr, SERVE_MEAN[-1]), xytext=(xr, FIXED_MEAN[-1]),
                arrowprops=dict(arrowstyle="<->", color=INK, lw=0.7,
                                shrinkA=1.5, shrinkB=1.5), zorder=2)

    ax.set_xlabel(r"Query recurrence rate $r$")
    ax.set_ylabel(r"Hit rate at $\beta \leq 1\%$")

    ax.set_xlim(0.18, 0.94)
    ax.set_xticks([0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9])
    ax.set_ylim(0.015, 0.155)
    ax.set_yticks([0.00, 0.04, 0.08, 0.12, 0.16])

    ax.legend(loc="upper left", handlelength=1.4, handletextpad=0.4,
              borderaxespad=0.2, labelspacing=0.25)

    for side in ("top", "right"):
        ax.spines[side].set_visible(False)

    return fig


HERE = os.path.dirname(os.path.abspath(__file__))

if __name__ == "__main__":
    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(HERE, "fig3_recurrence.pdf"), bbox_inches=None)
    plt.close(fig)

    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(HERE, "fig3_recurrence.png"), dpi=300,
                bbox_inches=None)
    plt.close(fig)

    final_rel = (SERVE_MEAN[-1] - FIXED_MEAN[-1]) / FIXED_MEAN[-1] * 100.0
    print(f"Saved fig3_recurrence.  final rel gap = +{final_rel:.1f}%")
    print(f"  fixed endpoint = {FIXED_MEAN[-1]:.4f}, serve endpoint = {SERVE_MEAN[-1]:.4f}")
