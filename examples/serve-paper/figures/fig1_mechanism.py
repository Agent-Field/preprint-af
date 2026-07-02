"""
fig1_mechanism.py -- SERVE caching pipeline with margin-aware gate.

Single \\columnwidth schematic figure illustrating how SERVE intercepts a query,
performs a top-k nearest-neighbour lookup in the cache, extracts four features
(s1, m, s2, local density rho) as zero-cost byproducts, and feeds them to a
gradient-boosted gate that decides between reuse and LLM call.
"""

import os
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, FancyArrowPatch

COLUMN_WIDTH_IN = 3.5       # EVIDENCE.md Figure candidates section; _style.py

COLORS = {
    "serve":  "#009E73",
    "fixed":  "#E69F00",
    "chance": "#555555",
}

INK = "#1a1a1a"

plt.rcParams.update({
    "font.size": 8,
    "axes.titlesize": 8,
    "axes.labelsize": 8,
    "xtick.labelsize": 7,
    "ytick.labelsize": 7,
    "legend.fontsize": 7,
    "font.family": "serif",
    "axes.linewidth": 0.6,
    "xtick.major.width": 0.6,
    "ytick.major.width": 0.6,
    "lines.linewidth": 1.4,
    "lines.markersize": 4.5,
    "axes.grid": False,
    "legend.frameon": False,
    "savefig.dpi": 300,
    "pdf.fonttype": 42,
    "ps.fonttype": 42,
})


def draw_box(ax, x, y, w, h, text, color=INK, fill="#f7f7f7", fontsize=7.5,
             textcolor=INK, bold=False, subtext=None, subtext_color=INK):
    """Draw a rounded box with optional subtext."""
    box = FancyBboxPatch((x - w / 2, y - h / 2), w, h,
                         boxstyle="round,pad=0.08",
                         facecolor=fill, edgecolor=color, linewidth=0.8,
                         zorder=3)
    ax.add_patch(box)
    weight = "bold" if bold else "normal"
    ax.text(x, y, text, ha="center", va="center", fontsize=fontsize,
            color=textcolor, fontweight=weight, zorder=4)
    if subtext is not None:
        ax.text(x, y - 0.055, subtext, ha="center", va="top", fontsize=6.5,
                color=subtext_color, style="italic", zorder=4)


def draw_arrow(ax, x1, y1, x2, y2, color=INK, lw=1.0, zorder=2):
    """Draw a downward arrow from (x1,y1) to (x2,y2)."""
    arrow = FancyArrowPatch((x1, y1), (x2, y2),
                            arrowstyle="-|>", mutation_scale=10,
                            lw=lw, color=color, zorder=zorder,
                            shrinkA=0, shrinkB=0.5)
    ax.add_patch(arrow)


def draw_split_arrow(ax, x_center, y_top, y_bottom, x_left, x_right,
                     color_left=COLORS["serve"], color_right=COLORS["fixed"]):
    """Draw a split: center -> left (reuse) and center -> right (LLM)."""
    ax.plot([x_center, x_left], [y_top, y_bottom], color=color_left, lw=1.1,
            zorder=2, solid_capstyle="round")
    ax.plot([x_center, x_right], [y_top, y_bottom], color=color_right, lw=1.1,
            zorder=2, solid_capstyle="round")


def build_figure():
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 3.8))
    ax.set_xlim(0, 1)
    ax.set_ylim(0, 1)
    ax.axis("off")
    ax.set_aspect("equal")

    cx = 0.5
    y_positions = [
        0.915,   # query
        0.785,   # top-k NN
        0.645,   # features
        0.485,   # gate
        0.325,   # decision (reuse / LLM)
    ]

    # Box dimensions
    bw_main = 0.72
    bh = 0.08

    # ------ 1. Query ------
    draw_box(ax, cx, y_positions[0], bw_main, bh,
             "Incoming query $q$", color=INK, fill="#ffffff",
             bold=True, fontsize=7.5)

    draw_arrow(ax, cx, y_positions[0] - bh / 2, cx, y_positions[1] + bh / 2)

    # ------ 2. Top-k NN Search ------
    draw_box(ax, cx, y_positions[1], bw_main, bh,
             "Top-$k$ nearest-neighbour search in cache",
             color=INK, fill="#f0f0f0", fontsize=7.0,
             subtext="$k = 8$ nearest entries")                       # E1

    draw_arrow(ax, cx, y_positions[1] - bh / 2, cx, y_positions[2] + bh / 2)

    # ------ 3. Feature extraction ------
    draw_box(ax, cx, y_positions[2], bw_main + 0.04, bh + 0.04,
             "Four zero-cost retrieval byproducts",
             color=INK, fill="#f0f0f0", fontsize=7.0)
    ax.text(cx, y_positions[2] - 0.02,
            "$s_1,\\; m = s_1 - s_2,\\; s_2,\\; \\rho$ (local density)",
            ha="center", va="center", fontsize=7.0, color=INK, zorder=4,
            fontweight="bold")
    ax.text(cx, y_positions[2] - 0.064,
            "discarded by GPTCache / MeanCache / vCache",            # E15
            ha="center", va="center", fontsize=6.5, color=COLORS["chance"],
            style="italic", zorder=4)

    draw_arrow(ax, cx, y_positions[2] - (bh + 0.04) / 2,
               cx, y_positions[3] + bh / 2)

    # ------ 4. Gradient-Boosted Gate ------
    # Slightly wider and taller box with SERVE green border
    draw_box(ax, cx, y_positions[3], bw_main + 0.06, bh + 0.03,
             "Gradient-boosted gate (GBDT)",
             color=COLORS["serve"], fill="#e8f5e9", fontsize=7.5,
             textcolor=COLORS["serve"], bold=True,
             subtext="150 trees, max depth 3, 0.8 $\\mu$s/query")    # E1, E5

    # Gate detail annotation
    ax.text(cx, y_positions[3] - 0.063,
            "4-dim input: $[s_1,\\, m,\\, s_2,\\, \\rho]$",
            ha="center", va="center", fontsize=6.8, color=INK, zorder=4,
            fontweight="bold")

    # ------ 5. Decision split ------
    left_x = cx - 0.35
    right_x = cx + 0.35

    draw_split_arrow(ax, cx, y_positions[3] - (bh + 0.03) / 2,
                     y_positions[4] + bh / 2 + 0.02, left_x, right_x)

    draw_box(ax, left_x, y_positions[4], 0.38, bh,
             "Reuse cached answer",
             color=COLORS["serve"], fill="#e8f5e9", fontsize=7.5,
             textcolor=COLORS["serve"], bold=True)
    draw_box(ax, right_x, y_positions[4], 0.38, bh,
             "Call LLM",
             color=COLORS["fixed"], fill="#fff3e0", fontsize=7.5,
             textcolor=COLORS["fixed"], bold=True)

    # ------ Side annotation: margin advantage ------
    ax.annotate(
        "Margin $m = s_1 - s_2$ alone\nrecovers 83% of the total\n"
        "improvement over a\nsimilarity-only gate",                  # E4
        xy=(0.895, 0.51), fontsize=6.8, color=INK,
        ha="center", va="center",
        bbox=dict(boxstyle="round,pad=0.25", facecolor="#fefefe",
                  edgecolor=COLORS["chance"], linewidth=0.5, alpha=0.95),
    )

    # Connecting line from feature box to annotation
    ax.plot([0.85, 0.895], [y_positions[2], 0.56], color=COLORS["chance"],
            lw=0.5, linestyle="--", zorder=1)

    # ------ Title ------
    ax.text(cx, 1.01, "SERVE: margin-aware cache-reuse pipeline",
            ha="center", va="bottom", fontsize=8.5, color=INK, fontweight="bold",
            zorder=5)

    return fig


HERE = os.path.dirname(os.path.abspath(__file__))

if __name__ == "__main__":
    fig = build_figure()
    fig.tight_layout(pad=0.2)
    fig.savefig(os.path.join(HERE, "fig1_mechanism.pdf"), bbox_inches=None)
    plt.close(fig)

    fig_png = build_figure()
    fig_png.tight_layout(pad=0.2)
    fig_png.savefig(os.path.join(HERE, "fig1_mechanism.png"), dpi=300,
                    bbox_inches=None)
    plt.close(fig_png)

    print("Saved fig1_mechanism.pdf and fig1_mechanism.png")