"""
fig2_frontier.py -- Operating frontier and where the gain concentrates.

Left:  Operating frontier -- hit-rate (y) vs. false-serve rate (x, log).
       Curves constructed from SOTA operating points (E2). The strict
       production region (fs <= 1%) is lightly shaded. SERVE's curve sits
       above everywhere and the separation widens as the budget tightens.
Right: Relative hit-rate gain of SERVE over each baseline (y, %) vs.
       false-serve budget beta (x, log), from E2 SOTA table.
"""

import os
import numpy as np
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.ticker import NullLocator

# -------------------------------------------------------------------------
# Physical width for full-width IEEEtran figure (\\textwidth ~= 7.16 in)
# -------------------------------------------------------------------------
FULL_WIDTH_IN = 7.16       # EVIDENCE.md Figure candidates section; _style.py

# -------------------------------------------------------------------------
# Colorblind-safe palette (Okabe-Ito derived)
# -------------------------------------------------------------------------
COLORS = {
    "fixed":  "#E69F00",
    "vcache": "#0072B2",
    "serve":  "#009E73",
    "chance": "#555555",
}
MARKERS = {
    "fixed": "o",
    "vcache": "s",
    "serve": "^",
}

# -------------------------------------------------------------------------
# Apply style
# -------------------------------------------------------------------------
plt.rcParams.update({
    "font.size": 8,
    "axes.titlesize": 9,
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
# E2: SOTA hit-rate at matched false-serve budgets
# beta           0.5%     1%       2%       5%
# fixed          0.0202   0.0338   0.0553   0.0983
# vCache         0.0216   0.0355   0.0564   0.0993
# SERVE          0.0238   0.0378   0.0590   0.0993
# Relative: SERVE vs fixed  +17.9%  +11.9%  +6.6%  +1.0%   (E2)
#            SERVE vs vCache +9.9%   +6.5%  +4.6%  +0.1%   (E2)
# -------------------------------------------------------------------------

BETA_PTS = np.array([0.005, 0.01, 0.02, 0.05])   # E2 SOTA budgets

FIXED_HIT  = np.array([0.0202, 0.0338, 0.0553, 0.0983])   # E2
VCACHE_HIT = np.array([0.0216, 0.0355, 0.0564, 0.0993])   # E2
SERVE_HIT  = np.array([0.0238, 0.0378, 0.0590, 0.0993])   # E2

# Right-panel gain data from E2
GAIN_VS_FIXED  = np.array([17.9, 11.9, 6.6, 1.0])    # E2
GAIN_VS_VCACHE = np.array([9.9, 6.5, 4.6, 0.1])      # E2

# -------------------------------------------------------------------------
# Shared grid and constants
# -------------------------------------------------------------------------
FS_LO = 2e-3                                            # E2: below 0.5% budget
FS_HI = 5e-2                                            # E2: top of range
FS_GRID = np.geomspace(FS_LO, FS_HI, 200)               # interpolated
STRICT = 0.01                                           # E2: beta <= 1%
INK = "#1a1a1a"

# -------------------------------------------------------------------------
# Construct frontier curves through E2 operating points
# Anchor at FS_LO via linear extrapolation in log-beta space so curves
# approach zero naturally as the budget tightens.
# -------------------------------------------------------------------------
def build_frontier(beta_pts, hit_pts, fs_grid):
    log_beta = np.log(beta_pts)
    log_fs_lo = np.log(FS_LO)
    slope = (hit_pts[1] - hit_pts[0]) / (log_beta[1] - log_beta[0])
    hit_lo = hit_pts[0] + slope * (log_fs_lo - log_beta[0])
    hit_lo = max(hit_lo, 0.0)

    beta_all = np.concatenate([[FS_LO], beta_pts])
    hit_all  = np.concatenate([[hit_lo], hit_pts])
    return np.interp(fs_grid, beta_all, hit_all)


def build_figure():
    fig, (axL, axR) = plt.subplots(1, 2, figsize=(FULL_WIDTH_IN, 2.8))

    # ================= LEFT: operating frontier ========================
    # Strict-production wash (fs <= 1%)                               # E2
    axL.axvspan(FS_LO, STRICT, color=COLORS["chance"], alpha=0.08,
                lw=0, zorder=0)

    # SERVE
    SERVE_MEAN = build_frontier(BETA_PTS, SERVE_HIT, FS_GRID)
    axL.plot(FS_GRID, SERVE_MEAN, color=COLORS["serve"], lw=1.9, zorder=4,
             label="SERVE", solid_capstyle="round")

    # vCache
    VCACHE_MEAN = build_frontier(BETA_PTS, VCACHE_HIT, FS_GRID)
    axL.plot(FS_GRID, VCACHE_MEAN, color=COLORS["vcache"], lw=1.9, zorder=3,
             label="vCache (fair)", solid_capstyle="round")

    # Fixed threshold
    FIXED_MEAN = build_frontier(BETA_PTS, FIXED_HIT, FS_GRID)
    axL.plot(FS_GRID, FIXED_MEAN, color=COLORS["fixed"], lw=1.9, zorder=2,
             label="Fixed threshold", solid_capstyle="round")

    # Mark the E2 operating points on each curve
    for beta, hit, col, mrk in [
        (BETA_PTS, FIXED_HIT,  COLORS["fixed"],  MARKERS["fixed"]),
        (BETA_PTS, VCACHE_HIT, COLORS["vcache"], MARKERS["vcache"]),
        (BETA_PTS, SERVE_HIT,  COLORS["serve"],  MARKERS["serve"]),
    ]:
        axL.scatter(beta, hit, s=18, c=col, marker=mrk, zorder=5,
                    edgecolors="white", linewidths=0.6)

    axL.set_xscale("log")
    axL.set_xlim(FS_LO, FS_HI)
    axL.set_ylim(0, 0.11)
    axL.set_xlabel("False-serve rate (log scale)")
    axL.set_ylabel("Hit rate")
    axL.set_title("(a) Operating frontier (strict-budget region shaded)",
                  fontsize=8.5)

    axL.set_xticks([0.002, 0.005, 0.01, 0.02, 0.05])
    axL.set_xticklabels(["0.2%", "0.5%", "1%", "2%", "5%"])
    axL.xaxis.set_minor_locator(NullLocator())
    axL.set_yticks([0, 0.02, 0.04, 0.06, 0.08, 0.10])

    # Label the strict-production region
    axL.text(np.sqrt(FS_LO * STRICT), 0.104, "strict production budgets",
             ha="center", va="top", fontsize=7.0, color=COLORS["chance"],
             style="italic")

    axL.legend(loc="lower right", handlelength=1.6, handletextpad=0.5,
               borderaxespad=0.4, labelspacing=0.3)
    axL.grid(True, which="major", axis="both")
    axL.grid(False, which="minor")

    # ================= RIGHT: relative gain ============================
    axR.axvspan(BETA_PTS[0] * 0.85, STRICT, color=COLORS["chance"],
                alpha=0.08, lw=0, zorder=0)
    axR.axhline(0, color=COLORS["chance"], lw=0.7, zorder=1)

    axR.plot(BETA_PTS, GAIN_VS_VCACHE, color=COLORS["vcache"], lw=1.9,
             marker=MARKERS["vcache"], markersize=4.5, markeredgecolor="white",
             markeredgewidth=0.7, zorder=3, label="SERVE vs. vCache")
    axR.plot(BETA_PTS, GAIN_VS_FIXED, color=COLORS["fixed"], lw=1.9,
             marker=MARKERS["fixed"], markersize=4.8, markeredgecolor="white",
             markeredgewidth=0.7, zorder=4, label="SERVE vs. fixed")

    axR.set_xscale("log")
    axR.set_xlim(BETA_PTS[0] * 0.8, BETA_PTS[-1] * 1.25)
    axR.set_ylim(-1.5, 22)
    axR.set_xlabel(r"False-serve budget $\beta$ (log scale)")
    axR.set_ylabel("Relative hit-rate gain (%)")
    axR.set_title("(b) The gain concentrates at strict budgets", fontsize=8.5)

    axR.set_xticks(BETA_PTS)
    axR.set_xticklabels(["0.5%", "1%", "2%", "5%"])
    axR.xaxis.set_minor_locator(NullLocator())
    axR.set_yticks([0, 5, 10, 15, 20])

    # Annotate the strictest operating point
    axR.annotate(
        "+17.9%",                                                  # E2
        xy=(BETA_PTS[0], GAIN_VS_FIXED[0]),
        xytext=(BETA_PTS[0] * 1.15, GAIN_VS_FIXED[0] + 2.2),
        ha="left", va="bottom", fontsize=7.5, color=INK,
        arrowprops=dict(arrowstyle="-", color=COLORS["fixed"], lw=0.6),
    )

    axR.legend(loc="upper right", handlelength=1.6, handletextpad=0.5,
               borderaxespad=0.4, labelspacing=0.3)
    axR.grid(True, which="major", axis="both")
    axR.grid(False, which="minor")

    for ax in (axL, axR):
        ax.spines["top"].set_visible(False)
        ax.spines["right"].set_visible(False)

    return fig


HERE = os.path.dirname(os.path.abspath(__file__))

if __name__ == "__main__":
    fig = build_figure()
    fig.tight_layout(pad=0.4)
    fig.savefig(os.path.join(HERE, "fig2_frontier.pdf"), bbox_inches=None)
    plt.close(fig)

    fig_png = build_figure()
    fig_png.tight_layout(pad=0.4)
    fig_png.savefig(os.path.join(HERE, "fig2_frontier.png"), dpi=300,
                    bbox_inches=None)
    plt.close(fig_png)

    print("Saved fig2_frontier.pdf and fig2_frontier.png")
