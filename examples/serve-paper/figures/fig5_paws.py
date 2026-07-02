"""
fig5_paws.py -- External adversarial validation on PAWS (ROC-AUC).

Story: on PAWS, an adversarial paraphrase benchmark where embeddings carry
semantic noise, the fixed-threshold rule inverts catastrophically
(AUC = 0.1408, below chance), while SERVE generalizes without any sign
knowledge (AUC = 0.9025), exceeding even a sign-corrected oracle baseline
(AUC = 0.8592). This demonstrates SERVE's robustness to embedding noise.

Data: single held-out PAWS test split (n=8000), sourced from
input/figures/fig5_paws.py and EVIDENCE.md E9.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(
    os.path.abspath(__file__)))), "input", "figures"))

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from _style import COLORS, apply_style, savefig

HERE = os.path.dirname(os.path.abspath(__file__))

CHANCE_AUC = 0.5
# E9: Fixed AUC = 0.1408 (below chance 0.5)
# E9: SERVE AUC = 0.9025
# E9: Sign-corrected oracle AUC = 1 - 0.1408 = 0.8592
AUC_FIXED = 0.1408       # E9
AUC_SERVE = 0.9025       # E9
AUC_ORACLE = 0.8592      # E9 (1 - AUC_fixed, explicitly derived in E9)
N_PAWS = 8000            # E9: test split size

METHODS = [
    "Fixed threshold\n(raw similarity)",
    "Sign-corrected\noracle baseline",
    "SERVE",
]
AUC = [AUC_FIXED, AUC_ORACLE, AUC_SERVE]
BAR_COLORS = [COLORS["fixed"], COLORS["oracle"], COLORS["serve"]]
INK = "#1a1a1a"


def build():
    apply_style()
    fig, ax = plt.subplots(figsize=(3.45, 2.6))

    y = range(len(METHODS))
    ax.barh(y, AUC, height=0.62, color=BAR_COLORS, zorder=3)

    ax.axvline(CHANCE_AUC, color=COLORS["chance"], linewidth=1.0, zorder=2)
    ax.text(
        CHANCE_AUC + 0.012, len(METHODS) - 0.42, "chance",
        ha="left", va="top", fontsize=7, color=COLORS["chance"], style="italic",
    )

    for yi, v, weight in [(0, AUC[0], "bold"), (1, AUC[1], "normal"), (2, AUC[2], "bold")]:
        ax.text(v + 0.015, yi, f"{v:.3f}", ha="left", va="center",
                fontsize=8, color=INK, fontweight=weight)

    ax.text(
        0.36, 0.0,
        "worse than chance:\nzero hits at any false-serve\nbudget $\\leq$ 5%",
        ha="left", va="center", fontsize=7, color=INK, zorder=5,
        bbox=dict(boxstyle="round,pad=0.15", facecolor="white",
                  edgecolor="none", alpha=0.85),
    )

    ax.set_yticks(list(y))
    ax.set_yticklabels(METHODS, fontsize=7.5, color=INK)
    ax.set_xlabel("ROC-AUC on PAWS")
    ax.set_xlim(0, 1.14)
    ax.set_xticks([0.0, 0.2, 0.4, 0.6, 0.8, 1.0])
    ax.set_ylim(-0.6, len(METHODS) - 0.4)

    ax.set_axisbelow(True)
    ax.xaxis.grid(True, color="#EDEDED", linewidth=0.6, zorder=0)
    ax.yaxis.grid(False)
    for spine in ("top", "right", "left"):
        ax.spines[spine].set_visible(False)
    ax.tick_params(axis="y", length=0)

    return fig


fig = build()
savefig(fig, os.path.join(HERE, "fig5_paws.pdf"))

fig = build()
fig.tight_layout(pad=0.4)
fig.savefig(os.path.join(HERE, "fig5_paws.png"), dpi=300, bbox_inches=None)
plt.close(fig)

print("Saved fig5_paws.pdf and fig5_paws.png")
