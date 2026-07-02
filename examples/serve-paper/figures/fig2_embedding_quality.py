"""
fig2_embedding_quality.py

Robustness to embedding quality: fixed-threshold vs. SERVE hit-rate at a
strict false-serve budget (beta <= 1%), compared across a weaker (MiniLM,
384d) and a stronger (mpnet, 768d) sentence embedding.

Source of truth (verbatim numbers, do not edit without re-checking
EVIDENCE.md):

    EVIDENCE.md, section "SERVE strategic axis 1: embedding quality"
    (serve_embedding_quality.py, 4 seeds):
        SERVE vs fixed @ fs<=1%, stand-alone (no vCache in this comparison):
        MiniLM (384d): fixed=0.0338  SERVE=0.0377  (+11.6%)
        mpnet  (768d): fixed=0.0351  SERVE=0.0383  (+9.0%)

No per-seed spread/std is reported in EVIDENCE.md for this block, so no
error bars are drawn (only the 4-seed mean per cell is available).
"""

import os
import sys

import numpy as np

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from _style import COLORS, COLUMN_WIDTH_IN, apply_style, savefig

import matplotlib.pyplot as plt

FIG_DIR = os.path.dirname(os.path.abspath(__file__))

# ---------------------------------------------------------------------------
# Hardcoded verified data (EVIDENCE.md, "SERVE strategic axis 1: embedding
# quality", serve_embedding_quality.py, 4 seeds).
# ---------------------------------------------------------------------------
embeddings = ["MiniLM\n(384d)", "mpnet\n(768d)"]
fixed_vals = [0.0338, 0.0351]
serve_vals = [0.0377, 0.0383]
pct_gain = [11.6, 9.0]  # relative gain of SERVE over fixed, in percent


def build_figure():
    apply_style(base_fontsize=8)
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 2.5))

    x = np.arange(len(embeddings))
    bar_width = 0.32

    bars_fixed = ax.bar(
        x - bar_width / 2,
        fixed_vals,
        bar_width,
        label="Fixed threshold",
        color=COLORS["fixed"],
        edgecolor="black",
        linewidth=0.5,
        zorder=3,
    )
    bars_serve = ax.bar(
        x + bar_width / 2,
        serve_vals,
        bar_width,
        label="SERVE",
        color=COLORS["serve"],
        edgecolor="black",
        linewidth=0.5,
        zorder=3,
    )

    # Value labels on top of each bar.
    for rect, val in zip(bars_fixed, fixed_vals):
        ax.annotate(
            f"{val:.4f}",
            xy=(rect.get_x() + rect.get_width() / 2, val),
            xytext=(0, 2),
            textcoords="offset points",
            ha="center",
            va="bottom",
            fontsize=6.3,
        )
    for rect, val in zip(bars_serve, serve_vals):
        ax.annotate(
            f"{val:.4f}",
            xy=(rect.get_x() + rect.get_width() / 2, val),
            xytext=(0, 2),
            textcoords="offset points",
            ha="center",
            va="bottom",
            fontsize=6.3,
        )

    # Relative-gain annotation above each pair of bars.
    for xi, (fv, sv, pg) in enumerate(zip(fixed_vals, serve_vals, pct_gain)):
        y_top = max(fv, sv) + 0.0048
        ax.annotate(
            f"+{pg:.1f}%",
            xy=(xi, y_top),
            ha="center",
            va="bottom",
            fontsize=6.8,
            color="#333333",
            fontweight="bold",
        )

    ax.set_xticks(x)
    ax.set_xticklabels(embeddings)
    ax.set_ylabel("Hit rate @ false-serve $\\beta \\leq 1\\%$")
    ax.set_xlabel("Embedding model")
    ax.set_ylim(0, 0.052)
    ax.set_yticks(np.arange(0, 0.051, 0.01))

    ax.legend(loc="upper right", ncol=1, handlelength=1.4, handletextpad=0.5,
              bbox_to_anchor=(1.02, 1.08))
    ax.grid(axis="y", zorder=0)
    ax.grid(axis="x", visible=False)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)

    return fig


if __name__ == "__main__":
    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(FIG_DIR, "fig2_embedding_quality.pdf"), bbox_inches=None)
    fig.savefig(os.path.join(FIG_DIR, "fig2_embedding_quality.png"), bbox_inches=None, dpi=300)
    plt.close(fig)
    print("Saved fig2_embedding_quality.pdf and fig2_embedding_quality.png")
