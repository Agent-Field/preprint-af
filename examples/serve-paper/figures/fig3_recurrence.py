"""
fig3_recurrence.py -- Robustness to query recurrence.

Line plot of SERVE's relative hit-rate gain over the fixed-threshold
baseline (at false-serve budget fs<=1%) as a function of query recurrence
rate r, swept from 0.2 to 0.9. Shows monotonic growth as recurrence rises.

Data source (hardcoded, verbatim):
  EVIDENCE.md, section "SERVE recurrence sweep, extended
  (serve_metric_sweep.py + serve_recurrence.py, 4 seeds)":
    "Full range r=0.2 to 0.9 @ fs<=1%: r=0.2:+20.9% r=0.3:+32.9%
     r=0.4:+36.1% r=0.5:+44.3% r=0.6:+54.7% r=0.7:+64.4% r=0.8:+73.3%
     r=0.9:+77.8%. Monotonic growth confirmed to the high-recurrence end
     (r=0.9), the realistic regime for FAQ/support-bot caches (a small
     set of questions dominates traffic)."

No per-seed spread (std/CI) is reported in EVIDENCE.md for this sweep,
so no error bars are drawn (never fabricate error bars from a mean).
Single series -> no legend needed (title already states what's plotted).
"""

import os

import matplotlib.pyplot as plt

from _style import COLUMN_WIDTH_IN, COLORS, apply_style, savefig

# ---------------------------------------------------------------------------
# Hardcoded data, verbatim from EVIDENCE.md "SERVE recurrence sweep,
# extended" block (serve_metric_sweep.py + serve_recurrence.py, 4 seeds).
# ---------------------------------------------------------------------------
recurrence_r = [0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9]
relative_gain_pct = [20.9, 32.9, 36.1, 44.3, 54.7, 64.4, 73.3, 77.8]

HERE = os.path.dirname(os.path.abspath(__file__))


def build_figure():
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 2.5))

    ax.plot(
        recurrence_r,
        relative_gain_pct,
        color=COLORS["serve"],
        marker="^",
        markersize=5,
        linewidth=1.6,
        zorder=3,
    )

    ax.set_xlabel("Query recurrence rate $r$")
    ax.set_ylabel("Relative hit-rate gain (%)")
    ax.set_title(r"SERVE gain over fixed threshold ($\beta \leq 1\%$)")

    ax.set_xlim(0.15, 0.95)
    ax.set_xticks([0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9])
    ax.set_ylim(0, 85)
    ax.set_yticks([0, 20, 40, 60, 80])

    return fig, ax


apply_style(base_fontsize=8)

# PDF (vector, for \includegraphics in the paper).
fig, ax = build_figure()
savefig(fig, os.path.join(HERE, "fig3_recurrence.pdf"))

# PNG (300dpi raster, identical content, for visual review).
fig, ax = build_figure()
fig.tight_layout(pad=0.4)
fig.savefig(os.path.join(HERE, "fig3_recurrence.png"), dpi=300, bbox_inches=None)
plt.close(fig)

print("Saved fig3_recurrence.pdf and fig3_recurrence.png")
