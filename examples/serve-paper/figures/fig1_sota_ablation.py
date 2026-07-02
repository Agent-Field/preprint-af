"""
fig1_sota_ablation.py -- Overall comparison and feature ablation for SERVE.

Two-panel, full-width (\\textwidth ~= 7.16in) figure:
  Left:  hit-rate at matched false-serve budget, fixed threshold vs. fair
         vCache reimplementation vs. SERVE, across beta in {0.5%, 1%, 2%, 5%}.
  Right: one-feature-at-a-time cumulative ablation at beta<=1%
         (s1 -> +margin -> +sim2 -> +density), showing the margin term
         recovers the majority of the total gain over similarity-only.

All numbers are hardcoded literals taken verbatim from EVIDENCE.md. See the
comment above each data block for the exact source section.
"""

import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import numpy as np
import matplotlib.pyplot as plt

from _style import FULL_WIDTH_IN, COLORS, apply_style, savefig

apply_style()

HERE = os.path.dirname(os.path.abspath(__file__))

# ---------------------------------------------------------------------------
# Data -- verbatim from EVIDENCE.md
# ---------------------------------------------------------------------------

# Source: EVIDENCE.md, "SERVE definitive numbers (serve_eval.py, 4 seeds,
# 100k MiniLM QQP, 60k cache)" block:
#   SOTA comparison @ fs<=1%: fixed=0.0338, vCache(fair)=0.0355, SERVE=0.0379.
#   Full sweep: fs<=0.5%: fixed=0.0202/vCache=0.0216/SERVE=0.0238.
#               fs<=2%: 0.0553/0.0564/0.0590.
#               fs<=5%: 0.0983/0.0993/0.0993 (converges to baseline).
BUDGETS = ["0.5%", "1%", "2%", "5%"]
HIT_RATE_FIXED = [0.0202, 0.0338, 0.0553, 0.0983]
HIT_RATE_VCACHE = [0.0216, 0.0355, 0.0564, 0.0993]
HIT_RATE_SERVE = [0.0238, 0.0379, 0.0590, 0.0993]

# Source: EVIDENCE.md, same block:
#   ABLATION @ fs<=1%: s1 (sim only)=0.0292 -> s1+margin=0.0364 (+24.4%) ->
#   +sim2=0.0374 (+3.0%) -> +density=0.0379 (+1.2%).
#   "The margin alone captures ~90% of the total gain over similarity-only."
ABLATION_LABELS = ["s1\n(sim only)", "+ margin\n(top1-top2)", "+ sim2", "+ density\n($\\rho$)"]
ABLATION_VALUES = [0.0292, 0.0364, 0.0374, 0.0379]


def build_figure():
    fig, (ax_left, ax_right) = plt.subplots(1, 2, figsize=(FULL_WIDTH_IN, 2.85))

    # --- Left panel: grouped bar chart, 3 methods x 4 budgets -------------
    n_budgets = len(BUDGETS)
    x = np.arange(n_budgets)
    bar_w = 0.26

    ax_left.bar(x - bar_w, HIT_RATE_FIXED, width=bar_w, color=COLORS["fixed"],
                label="Fixed threshold", edgecolor="black", linewidth=0.4)
    ax_left.bar(x, HIT_RATE_VCACHE, width=bar_w, color=COLORS["vcache"],
                label="vCache (fair)", edgecolor="black", linewidth=0.4)
    ax_left.bar(x + bar_w, HIT_RATE_SERVE, width=bar_w, color=COLORS["serve"],
                label="SERVE", edgecolor="black", linewidth=0.4)

    ax_left.set_xticks(x)
    ax_left.set_xticklabels([r"$\beta\leq$" + b for b in BUDGETS])
    ax_left.set_xlabel("False-serve budget $\\beta$")
    ax_left.set_ylabel("Hit rate at matched $\\beta$")
    ax_left.set_title("(a) Hit rate vs. false-serve budget", fontsize=8.5)
    ax_left.set_ylim(0, 0.115)
    ax_left.legend(loc="upper left", ncol=1, handlelength=1.4, handletextpad=0.5,
                   borderaxespad=0.3)
    ax_left.grid(axis="y")
    ax_left.grid(axis="x", visible=False)

    # Annotate the strict operating point (beta<=1%) relative gain, per
    # EVIDENCE.md: "SERVE vs fixed: +12.1% rel."
    idx_1pct = BUDGETS.index("1%")
    ax_left.annotate(
        "+12.1%", xy=(idx_1pct + bar_w, HIT_RATE_SERVE[idx_1pct]),
        xytext=(idx_1pct + bar_w, HIT_RATE_SERVE[idx_1pct] + 0.011),
        ha="center", fontsize=6.3, color=COLORS["serve"],
        arrowprops=dict(arrowstyle="-", color=COLORS["serve"], lw=0.5),
    )

    # --- Right panel: cumulative ablation (step / bar chart) --------------
    n_feat = len(ABLATION_LABELS)
    xr = np.arange(n_feat)
    bar_colors = [COLORS["fixed"], COLORS["serve"], COLORS["serve"], COLORS["serve"]]
    bar_alphas = [0.55, 1.0, 0.78, 0.58]

    bars = ax_right.bar(xr, ABLATION_VALUES, width=0.6,
                         color=bar_colors, edgecolor="black", linewidth=0.4)
    for b, a in zip(bars, bar_alphas):
        b.set_alpha(a)

    for xi, v in zip(xr, ABLATION_VALUES):
        ax_right.text(xi, v + 0.0009, f"{v:.4f}", ha="center", va="bottom", fontsize=6.3)

    ax_right.plot(xr, ABLATION_VALUES, color="black", linewidth=0.8, marker="o",
                  markersize=2.6, zorder=3)

    ax_right.annotate(
        "+24.4%\n(margin)",
        xy=(1, (ABLATION_VALUES[0] + ABLATION_VALUES[1]) / 2),
        xytext=(1.62, ABLATION_VALUES[0] + 0.001),
        fontsize=6.3, color=COLORS["serve"], ha="left", va="center",
        arrowprops=dict(arrowstyle="->", color=COLORS["serve"], lw=0.6),
    )

    ax_right.set_xticks(xr)
    ax_right.set_xticklabels(ABLATION_LABELS)
    ax_right.set_ylabel("Hit rate at $\\beta\\leq$1%")
    ax_right.set_title("(b) Cumulative feature ablation ($\\beta\\leq$1%)", fontsize=8.5)
    ax_right.set_ylim(0, 0.043)
    ax_right.grid(axis="y")
    ax_right.grid(axis="x", visible=False)

    return fig


if __name__ == "__main__":
    fig_pdf = build_figure()
    savefig(fig_pdf, os.path.join(HERE, "fig1_sota_ablation.pdf"))

    fig_png = build_figure()
    fig_png.tight_layout(pad=0.4)
    fig_png.savefig(os.path.join(HERE, "fig1_sota_ablation.png"), dpi=300, bbox_inches=None)
    plt.close(fig_png)

    print("Saved fig1_sota_ablation.pdf and fig1_sota_ablation.png")
