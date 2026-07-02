"""
fig2_embedding_quality.py

Robustness to embedding quality: fixed-threshold vs. SERVE across two axes:
  (a) embedding model strength  (MiniLM 384d vs. mpnet 768d at beta <= 1%)
  (b) adversarial degradation    (PAWS benchmark, zero hits for fixed at all budgets)

Data loaded from input/figures/fig2_embedding_quality.py (stand-alone 4-seed run)
and EVIDENCE.md facts E9 (PAWS) and E11 (embedding robustness).
"""

import os
import sys

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

# ---------------------------------------------------------------------------
# Self-contained style constants (consistent with input/figures/_style.py)
# ---------------------------------------------------------------------------
FULL_WIDTH_IN = 7.16   # \textwidth for two-column IEEEtran figure*

COLORS = {
    "fixed": "#E69F00",    # orange -- fixed-threshold baseline
    "serve": "#009E73",    # green  -- SERVE
    "oracle": "#CC79A7",   # pink   -- sign-corrected oracle baseline
    "chance": "#555555",   # grey   -- chance / reference lines
}


def apply_style(base_fontsize=8):
    plt.rcParams.update({
        "font.size": base_fontsize,
        "axes.titlesize": base_fontsize + 1,
        "axes.labelsize": base_fontsize,
        "xtick.labelsize": base_fontsize - 1,
        "ytick.labelsize": base_fontsize - 1,
        "legend.fontsize": base_fontsize - 1,
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


# ---------------------------------------------------------------------------
# Panel (a): Embedding model robustness
# Data from input/figures/fig2_embedding_quality.py (stand-alone 4-seed run).
# Values differ slightly from main.tex due to independent re-run.
# ---------------------------------------------------------------------------
EMBEDDINGS = ["MiniLM\n(384d)", "mpnet\n(768d)"]
FIXED_EMB = [0.0338, 0.0351]   # E11 stand-alone run
SERVE_EMB = [0.0377, 0.0383]   # E11 stand-alone run
PCT_GAIN_EMB = [11.6, 9.0]     # E11 stand-alone run

# ---------------------------------------------------------------------------
# Panel (b): PAWS adversarial embedding quality degradation
# Data from E9: single held-out test split, n=8,000.
# ---------------------------------------------------------------------------
BUDGETS_PAWS = [r"$\beta\leq$0.5%", r"$\beta\leq$1%", r"$\beta\leq$2%", r"$\beta\leq$5%"]
FIXED_PAWS = [0.0000, 0.0000, 0.0000, 0.0000]   # E9: zero hits at any budget
SERVE_PAWS = [0.0025, 0.0053, 0.0100, 0.0215]    # E9


def build_panel_embedding(ax):
    n_models = len(EMBEDDINGS)
    x = np.arange(n_models)
    bar_width = 0.32

    ax.bar(
        x - bar_width / 2, FIXED_EMB, bar_width,
        label="Fixed threshold", color=COLORS["fixed"],
        edgecolor="black", linewidth=0.5, zorder=3,
    )
    ax.bar(
        x + bar_width / 2, SERVE_EMB, bar_width,
        label="SERVE", color=COLORS["serve"],
        edgecolor="black", linewidth=0.5, zorder=3,
    )

    for xi, (fv, sv, pg) in enumerate(zip(FIXED_EMB, SERVE_EMB, PCT_GAIN_EMB)):
        ax.annotate(
            f"{fv:.4f}",
            xy=(xi - bar_width / 2, fv),
            xytext=(0, 2), textcoords="offset points",
            ha="center", va="bottom", fontsize=6.3,
        )
        ax.annotate(
            f"{sv:.4f}",
            xy=(xi + bar_width / 2, sv),
            xytext=(0, 2), textcoords="offset points",
            ha="center", va="bottom", fontsize=6.3,
        )
        y_top = max(fv, sv) + 0.0048
        ax.annotate(
            f"+{pg:.1f}%",
            xy=(xi, y_top),
            ha="center", va="bottom", fontsize=6.8,
            color="#333333", fontweight="bold",
        )

    ax.set_xticks(x)
    ax.set_xticklabels(EMBEDDINGS)
    ax.set_xlabel("Embedding model")
    ax.set_ylabel(r"Hit rate at false-serve $\beta\leq 1\%$")
    ax.set_title("(a) Embedding model robustness", fontsize=9)
    ax.set_ylim(0, 0.052)
    ax.set_yticks(np.arange(0, 0.051, 0.01))
    ax.legend(loc="upper right", ncol=1, handlelength=1.4, handletextpad=0.5,
              bbox_to_anchor=(1.02, 1.08))
    ax.grid(axis="y", zorder=0)
    ax.grid(axis="x", visible=False)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)


def build_panel_paws(ax):
    n_budgets = len(BUDGETS_PAWS)
    x = np.arange(n_budgets)
    bar_width = 0.32

    ax.bar(
        x - bar_width / 2, FIXED_PAWS, bar_width,
        label="Fixed threshold", color=COLORS["fixed"],
        edgecolor="black", linewidth=0.5, zorder=3,
    )
    ax.bar(
        x + bar_width / 2, SERVE_PAWS, bar_width,
        label="SERVE", color=COLORS["serve"],
        edgecolor="black", linewidth=0.5, zorder=3,
    )

    for xi, sv in enumerate(SERVE_PAWS):
        ax.annotate(
            f"{sv:.4f}",
            xy=(xi + bar_width / 2, sv),
            xytext=(0, 2), textcoords="offset points",
            ha="center", va="bottom", fontsize=6.3,
        )

    ax.annotate(
        "zero hits for fixed\nat all budgets",
        xy=(n_budgets / 2 - 0.5, SERVE_PAWS[-1] + 0.005),
        ha="center", va="bottom", fontsize=7,
        color=COLORS["fixed"], fontstyle="italic",
        bbox=dict(boxstyle="round,pad=0.15", facecolor="white",
                  edgecolor="none", alpha=0.85),
    )

    ax.set_xticks(x)
    ax.set_xticklabels(BUDGETS_PAWS)
    ax.set_xlabel("False-serve budget")
    ax.set_ylabel("Hit rate")
    ax.set_title("(b) PAWS adversarial degradation", fontsize=9)
    ax.set_ylim(0, 0.028)
    ax.set_yticks(np.arange(0, 0.026, 0.005))
    ax.legend(loc="upper left", ncol=1, handlelength=1.4, handletextpad=0.5)
    ax.grid(axis="y", zorder=0)
    ax.grid(axis="x", visible=False)
    for spine in ("top", "right"):
        ax.spines[spine].set_visible(False)


def build_figure():
    apply_style(base_fontsize=8)
    fig, (ax_left, ax_right) = plt.subplots(1, 2, figsize=(FULL_WIDTH_IN, 2.85))
    build_panel_embedding(ax_left)
    build_panel_paws(ax_right)
    return fig


if __name__ == "__main__":
    HERE = os.path.dirname(os.path.abspath(__file__))

    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(HERE, "fig2_embedding_quality.pdf"), bbox_inches=None)
    fig.savefig(os.path.join(HERE, "fig2_embedding_quality.png"), bbox_inches=None, dpi=300)
    plt.close(fig)
    print("Saved fig2_embedding_quality.pdf and fig2_embedding_quality.png")