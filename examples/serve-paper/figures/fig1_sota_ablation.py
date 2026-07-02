"""
fig1_sota_ablation.py -- SOTA comparison and feature ablation for SERVE.

Two-panel figure:
  Left:  hit-rate at matched false-serve budget across methods.
  Right: cumulative feature ablation at beta<=1% showing margin recovery.

All hardcoded numeric literals trace to EVIDENCE.md fact ids (E<n> comments).
"""

import sys
import os

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "../../input/figures"))

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

from _style import FULL_WIDTH_IN, COLORS, apply_style, savefig

apply_style()

# ---- SOTA comparison data ---------------------------------------------------
# E2: On 100k MiniLM QQP (60k cache, 4 seeds)
#   beta<=0.5%: fixed=0.0202, vCache=0.0216, SERVE=0.0238 (rel. +17.9%, +9.9%)
#   beta<=1%:   fixed=0.0338, vCache=0.0355, SERVE=0.0378 (rel. +11.9%, +6.5%)
#   beta<=2%:   fixed=0.0553, vCache=0.0564, SERVE=0.0590 (rel. +6.6%, +4.6%)
#   beta<=5%:   fixed=0.0983, vCache=0.0993, SERVE=0.0993 (rel. +1.0%, +0.1%)

BUDGETS = ["0.5%", "1%", "2%", "5%"]
HIT_RATE_FIXED  = [0.0202, 0.0338, 0.0553, 0.0983]  # E2
HIT_RATE_VCACHE = [0.0216, 0.0355, 0.0564, 0.0993]  # E2
HIT_RATE_SERVE  = [0.0238, 0.0378, 0.0590, 0.0993]  # E2

# ---- Ablation data at beta<=1% ----------------------------------------------
# E4: s1 only=0.0292, s1+m=0.0363 (+24.4%), s1+m+s2=0.0374 (+2.9%),
#     s1+m+s2+rho=0.0378 (+1.1%)
#     Margin recovery: (0.0363-0.0292)/(0.0378-0.0292) ~= 0.83

ABLATION_LABELS = [
    "s1\n(sim only)",
    "+ margin\n(top1-top2)",
    "+ sim2",
    "+ density\n($\\rho$)",
]
ABLATION_VALUES = [0.0292, 0.0363, 0.0374, 0.0378]  # E4
MARGIN_RECOVERY = 0.83  # E4: (0.0363-0.0292)/(0.0378-0.0292)

# Relative gain of margin step
MARGIN_REL_GAIN = (ABLATION_VALUES[1] - ABLATION_VALUES[0]) / ABLATION_VALUES[0]  # E4: +24.4%

# SOTA relative gain at beta<=1%  (E2)
SOTA_REL_GAIN = (HIT_RATE_SERVE[1] - HIT_RATE_FIXED[1]) / HIT_RATE_FIXED[1]  # E2: +11.9%


def build_figure():
    fig, (ax_left, ax_right) = plt.subplots(1, 2, figsize=(FULL_WIDTH_IN, 2.85))

    # --- Left panel: grouped bar chart, 3 methods x 4 budgets -----------------
    n_budgets = len(BUDGETS)
    x = np.arange(n_budgets)
    bar_w = 0.27

    ax_left.bar(x - bar_w, HIT_RATE_FIXED, width=bar_w, color=COLORS["fixed"],
                label="Fixed threshold", edgecolor="black", linewidth=0.4)
    ax_left.bar(x, HIT_RATE_VCACHE, width=bar_w, color=COLORS["vcache"],
                label="vCache (fair)", edgecolor="black", linewidth=0.4)
    ax_left.bar(x + bar_w, HIT_RATE_SERVE, width=bar_w, color=COLORS["serve"],
                label="SERVE", edgecolor="black", linewidth=0.4)

    ax_left.set_xticks(x)
    ax_left.set_xticklabels([r"$\beta\leq$" + b for b in BUDGETS])
    ax_left.set_ylabel("Hit rate at matched $\\beta$")
    ax_left.set_title("(a) Hit rate vs. false-serve budget", fontsize=8.5)
    ax_left.set_ylim(0, 0.118)
    ax_left.legend(loc="upper left", ncol=1, handlelength=1.4,
                   handletextpad=0.5, borderaxespad=0.3, fontsize=7)
    ax_left.grid(axis="y")
    ax_left.grid(axis="x", visible=False)

    # Annotate SERVE gain over fixed at beta<=1%
    idx_1 = 1
    serve_val = HIT_RATE_SERVE[idx_1]  # E2: 0.0378
    ax_left.annotate(
        f"+{SOTA_REL_GAIN*100:.1f}\\%",
        xy=(idx_1 + bar_w, serve_val),
        xytext=(idx_1 + bar_w + 0.25, serve_val + 0.008),
        ha="center", fontsize=6.3, color=COLORS["serve"],
        arrowprops=dict(arrowstyle="->", color=COLORS["serve"], lw=0.6),
    )

    # --- Right panel: cumulative ablation (step / bar chart) ------------------
    n_feat = len(ABLATION_LABELS)
    xr = np.arange(n_feat)
    bar_colors = [COLORS["fixed"], COLORS["serve"], COLORS["serve"], COLORS["serve"]]
    bar_alphas = [0.55, 1.0, 0.78, 0.58]

    bars = ax_right.bar(xr, ABLATION_VALUES, width=0.6,
                        color=bar_colors, edgecolor="black", linewidth=0.4)
    for b, a in zip(bars, bar_alphas):
        b.set_alpha(a)

    for xi, v in zip(xr, ABLATION_VALUES):
        ax_right.text(xi, v + 0.0009, f"{v:.4f}", ha="center",
                      va="bottom", fontsize=6.3)

    ax_right.plot(xr, ABLATION_VALUES, color="black", linewidth=0.8,
                  marker="o", markersize=2.6, zorder=3)

    # Annotate the margin step gain
    mid_y = (ABLATION_VALUES[0] + ABLATION_VALUES[1]) / 2  # E4
    ax_right.annotate(
        f"+{MARGIN_REL_GAIN*100:.1f}\\%\n(margin)",
        xy=(1, mid_y),
        xytext=(-0.16, ABLATION_VALUES[3] + 0.005),
        fontsize=6.3, color="black", ha="left", va="center",
        arrowprops=dict(arrowstyle="->", color="black", lw=0.6),
    )

    # Annotate the 83% recovery
    ax_right.annotate(
        f"{MARGIN_RECOVERY*100:.0f}\\% of\ntotal gain",  # E4: 83%
        xy=(1, ABLATION_VALUES[1]),
        xytext=(1.55, ABLATION_VALUES[1] + 0.003),
        fontsize=6.3, color="black", ha="center", va="bottom",
        arrowprops=dict(arrowstyle="->", color="black", lw=0.6),
    )

    ax_right.set_xticks(xr)
    ax_right.set_xticklabels(ABLATION_LABELS)
    ax_right.set_ylabel("Hit rate at $\\beta\\leq$1%")
    ax_right.set_title("(b) Cumulative feature ablation ($\\beta\\leq$1%)",
                       fontsize=8.5)
    ax_right.set_ylim(0, 0.054)
    ax_right.grid(axis="y")
    ax_right.grid(axis="x", visible=False)

    return fig


if __name__ == "__main__":
    fig = build_figure()
    savefig(fig, os.path.join(HERE, "fig1_sota_ablation.pdf"))

    fig2 = build_figure()
    fig2.savefig(os.path.join(HERE, "fig1_sota_ablation.png"), dpi=300,
                 bbox_inches=None)
    plt.close(fig2)

    print("Saved fig1_sota_ablation.pdf and fig1_sota_ablation.png")
