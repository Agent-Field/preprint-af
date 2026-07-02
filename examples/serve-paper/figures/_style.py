"""
Shared matplotlib style module for SERVE paper figures.

IEEEtran two-column journal layout reference widths:
  - \\columnwidth  ~= 3.5 in   (single-column figures)
  - \\textwidth    ~= 7.16 in  (full-width, two-column \\figure* figures)

Figures MUST be authored at (or very near) these physical widths so that
\\includegraphics{...} does not need to rescale the figure. Rescaling a
figure saved at some other size shrinks/enlarges its embedded font size
relative to the surrounding body text, which is the "looks fine alone,
wrong once placed" failure this module exists to avoid.

Usage:
    from _style import (
        COLUMN_WIDTH_IN, FULL_WIDTH_IN, COLUMN_FIGSIZE, FULL_FIGSIZE,
        COLORS, apply_style, savefig
    )

    apply_style()
    fig, ax = plt.subplots(figsize=COLUMN_FIGSIZE)
    ...
    savefig(fig, "fig2_embedding_quality.pdf")
"""

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt

# ---------------------------------------------------------------------------
# Reference physical widths (inches), matching IEEEtran journal-class layout.
# ---------------------------------------------------------------------------
COLUMN_WIDTH_IN = 3.5   # \columnwidth for a two-column IEEEtran journal page
FULL_WIDTH_IN = 7.16    # \textwidth (both columns spanned via figure*)

# Default aspect ratios; individual figures may override the height.
COLUMN_FIGSIZE = (COLUMN_WIDTH_IN, 2.6)
FULL_FIGSIZE = (FULL_WIDTH_IN, 2.75)

# ---------------------------------------------------------------------------
# Colorblind-safe palette (Okabe-Ito derived), one consistent color per
# method across every figure in the paper.
# ---------------------------------------------------------------------------
COLORS = {
    "fixed": "#E69F00",    # orange  -- fixed-threshold baseline (GPTCache/MeanCache-style)
    "vcache": "#0072B2",   # blue    -- fair vCache reimplementation
    "serve": "#009E73",    # green   -- SERVE (this work)
    "oracle": "#CC79A7",   # pink    -- sign-corrected / oracle baseline (PAWS figure only)
    "chance": "#7F7F7F",   # grey    -- chance / reference lines
    "grid_lo": "#F7F7F7",  # heatmap low end
    "grid_hi": "#009E73",  # heatmap high end (reuses SERVE green)
}

MARKERS = {
    "fixed": "o",
    "vcache": "s",
    "serve": "^",
    "oracle": "D",
}


def apply_style(base_fontsize: int = 8):
    """Apply a consistent, body-text-scale style for all SERVE figures.

    base_fontsize ~8-9pt matches IEEEtran two-column body text so that
    figures placed at their native physical width (see COLUMN_WIDTH_IN /
    FULL_WIDTH_IN) read at the same visual scale as the surrounding prose.
    """
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


def savefig(fig, filename, tight=True):
    """Save a figure to the figures/ directory at its authored physical size.

    Uses bbox_inches=None (NOT 'tight') by default reasoning override: we
    still allow a small pad via tight_layout before saving, but we do NOT
    pass bbox_inches='tight' to savefig, because that can alter the final
    physical dimensions of the saved artifact relative to the figsize the
    caller chose to match \\columnwidth / \\textwidth exactly.
    """
    if tight:
        fig.tight_layout(pad=0.4)
    fig.savefig(filename, bbox_inches=None)
    plt.close(fig)
