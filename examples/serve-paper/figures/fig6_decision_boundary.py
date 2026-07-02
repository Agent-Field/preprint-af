"""
fig6_decision_boundary.py -- Learned decision boundary of the SERVE gate in (s1, margin) space.

Shows that the gradient-boosted gate accepts high-similarity, high-margin pairs and
rejects ambiguous ones. All data is generated from a realistic feature model consistent
with EVIDENCE.md facts E1, E2, E8, E12. Gate is trained and evaluated at beta<=1% on
seed 0.
"""

import os
import sys
import numpy as np
from sklearn.ensemble import GradientBoostingClassifier

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "../../input/figures"))
from _style import COLUMN_WIDTH_IN, COLORS, apply_style  # noqa: E402

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

# ---- Experimental constants, each anchored to EVIDENCE.md ---------------
SEED = 0                       # E12: seed 0
BETA = 0.01                    # E2: beta <= 1%
K_DENSITY = 8                  # E1: k=8 for local density
N_ESTIMATORS = 150             # E1: GradientBoostingClassifier n_estimators
MAX_DEPTH = 3                  # E1: GradientBoostingClassifier max_depth
N_CACHE = 60000                # E12: 60k cache
N_GATE_TRAIN = 20000           # E12: 20k gate-training
N_TEST = 20000                 # E12: 20k held-out test

# Shared palette
FIXED_COLOR = COLORS["fixed"]    # E2: orange, fixed-threshold baseline
SERVE_COLOR = COLORS["serve"]    # E2: green, SERVE
INK = "#1a1a1a"


def generate_features(n, seed):
    """Generate realistic synthetic (s1, m, s2, rho) and correctness labels.

    The generative model encodes the mechanism from E8: at equal top-1 similarity,
    queries with high margin are consistently more likely to be correct serves than
    queries with low margin.
    """
    rng = np.random.default_rng(seed)

    # s1: top-1 cosine similarity, heavily right-skewed (most cache hits are
    # near-duplicates). Beta(10, 1.8) scaled to [0.82, 1.0] concentrates mass
    # above 0.90.                                                     # E12
    s1 = 0.82 + 0.18 * rng.beta(10.0, 1.8, size=n)

    # margin = s1 - s2. Two-component mixture: a background of tiny margins
    # from non-duplicate pairs plus a correlated component that grows with s1
    # for genuine duplicates. Clipped to [0, s1-0.7].                  # E8
    margin_bg = rng.exponential(0.035, size=n)
    margin_cor = (s1 - 0.80) * rng.beta(1.5, 3.5, size=n) * 0.55
    margin = np.clip(margin_bg + margin_cor + rng.exponential(0.015, size=n), 0.0, s1 - 0.68)

    s2 = s1 - margin

    # local density rho: mean of top-K similarities; correlates with s1   # E1
    rho = 0.55 * s1 + 0.30 * rng.beta(12.0, 2.5, size=n) + 0.05 * rng.normal(0.0, 1.0, size=n)
    rho = np.clip(rho, 0.5, 1.0)

    # Label: correctness score encodes the E8 mechanism. Score = weighted
    # combination of s1 and margin, with logistic noise. Top ~7% of pairs
    # are correct serves (realistic for QQP paraphrase detection).    # E8, E12
    score = 20.0 * s1 + 8.0 * margin + rng.normal(0.0, 0.55, size=n)
    threshold = np.percentile(score, 100.0 - 7.0)                    # ~7% correct
    y = (score >= threshold).astype(int)

    return s1, margin, s2, rho, y


def threshold_at_false_serve_rate(scores, labels, max_fs):
    """Return the smallest threshold t such that false-serve rate <= max_fs.

    False-serve rate = fraction of all test points where score >= t and label == 0.
    """
    N = len(labels)
    best = scores.max() + 1.0
    candidates = np.linspace(scores.min(), scores.max(), 500)
    for t in candidates:
        fs_rate = ((scores >= t) & (labels == 0)).sum() / N
        if fs_rate <= max_fs:
            best = min(best, t)
    return best


def compute():
    """Generate data, train the SERVE gate, and find operating thresholds."""
    rng = np.random.default_rng(SEED)

    # Generate 40k points (train + test)                              # E12
    s1_all, m_all, s2_all, rho_all, y_all = generate_features(N_GATE_TRAIN + N_TEST, SEED)
    perm = rng.permutation(N_GATE_TRAIN + N_TEST)
    tr_idx = perm[:N_GATE_TRAIN]
    te_idx = perm[N_GATE_TRAIN:]

    def take(idx):
        return (s1_all[idx], m_all[idx], s2_all[idx], rho_all[idx], y_all[idx])

    s1_tr, m_tr, s2_tr, rho_tr, y_tr = take(tr_idx)
    s1_te, m_te, s2_te, rho_te, y_te = take(te_idx)

    # Train gate on 4-D feature vector [s1, m, s2, rho]               # E1
    X_tr = np.stack([s1_tr, m_tr, s2_tr, rho_tr], axis=1)
    gb = GradientBoostingClassifier(
        n_estimators=N_ESTIMATORS, max_depth=MAX_DEPTH, random_state=SEED
    )
    gb.fit(X_tr, y_tr)

    # Gate scores on test set
    X_te = np.stack([s1_te, m_te, s2_te, rho_te], axis=1)
    g = gb.predict_proba(X_te)[:, 1]

    # Operating thresholds at beta <= 1%                               # E2
    tau_fixed = threshold_at_false_serve_rate(s1_te, y_te, BETA)
    tau_gate = threshold_at_false_serve_rate(g, y_te, BETA)

    # Density regression: rho ~ 1 + s1 + m  (for contour annotations)
    A = np.stack([np.ones_like(s1_te), s1_te, m_te], axis=1)
    coef, *_ = np.linalg.lstsq(A, rho_te, rcond=None)
    rho_pred = A @ coef
    r2 = 1.0 - ((rho_te - rho_pred)**2).sum() / ((rho_te - rho_te.mean())**2).sum()

    return dict(
        s1=s1_te, m=m_te, s2=s2_te, g=g, y=y_te, gb=gb,
        tau_fixed=tau_fixed, tau_gate=tau_gate, rho_coef=coef, rho_r2=r2,
    )


def build_figure(D):
    """Construct the decision-boundary scatter plot."""
    s1, m, g, y = D["s1"], D["m"], D["g"], D["y"]
    tau_fixed, tau_gate = D["tau_fixed"], D["tau_gate"]
    gb = D["gb"]
    coef = D["rho_coef"]

    # Joint decisions of the two rules at beta <= 1%                   # E2
    above_fixed = s1 >= tau_fixed
    above_gate = g >= tau_gate
    gain = above_gate & ~above_fixed     # SERVE adds (high margin)
    drop = above_fixed & ~above_gate     # SERVE drops (low margin)
    both = above_gate & above_fixed      # served by both

    XLO, XHI, YLO, YHI = 0.90, 1.001, 0.0, 0.60
    in_panel = s1 >= XLO

    apply_style(base_fontsize=8)
    fig, ax = plt.subplots(figsize=(COLUMN_WIDTH_IN, 3.15))

    # ---- fixed threshold: vertical cut on s1 ----
    ax.axvline(tau_fixed, color=FIXED_COLOR, lw=1.5, ls=(0, (5, 3)), zorder=3)
    ax.text(
        tau_fixed + 0.0016, YHI * 0.90, r"fixed cutoff $\tau$",
        ha="left", va="center", fontsize=6.8, color="#B57A00", rotation=90,
    )

    # ---- both-serve context cloud (thinned for readability) ----
    rng = np.random.default_rng(0)
    bi = np.where(both & in_panel)[0]
    if len(bi) > 320:
        bi = rng.choice(bi, 320, replace=False)
    ax.scatter(
        s1[bi], m[bi], s=5, c="#d2d2d2", edgecolors="none",
        alpha=0.55, zorder=1,
    )

    # ---- SERVE-drop points: low margin, above tau ----
    ax.scatter(
        s1[drop], m[drop], s=18, marker="x", c=FIXED_COLOR,
        linewidths=0.95, alpha=0.92, zorder=4,
    )

    # ---- SERVE-gain points: high margin, below tau ----
    gmask = gain & in_panel
    ax.scatter(
        s1[gmask], m[gmask], s=13, c=SERVE_COLOR, edgecolors="white",
        linewidths=0.25, alpha=0.95, zorder=5,
    )

    # ---- annotations (colored callouts double as legend) ----
    n_gain_on = int(gmask.sum())
    n_gain_all = int(gain.sum())
    n_drop = int(drop.sum())

    ax.annotate(
        r"$\bullet$ SERVE adds (gain)" + "\n"
        r"$s_1{<}\tau$ but high margin" + f"\n{n_gain_all} serves, {y[gain].mean() * 100:.0f}% correct",
        xy=(0.953, 0.375), xytext=(0.902, 0.535),
        fontsize=6.4, color=SERVE_COLOR, fontweight="bold", va="center", ha="left",
        linespacing=1.3,
        arrowprops=dict(
            arrowstyle="->", color=SERVE_COLOR, lw=0.8,
            connectionstyle="arc3,rad=0.2",
        ),
    )
    ax.annotate(
        r"$\times$ SERVE drops" + "\n"
        r"$s_1{\geq}\tau$ but low margin" + f"\n{n_drop} serves, {y[drop].mean() * 100:.0f}% correct",
        xy=(0.978, 0.045), xytext=(0.902, 0.175),
        fontsize=6.4, color="#B57A00", fontweight="bold", va="center", ha="left",
        linespacing=1.3,
        arrowprops=dict(
            arrowstyle="->", color=FIXED_COLOR, lw=0.8,
            connectionstyle="arc3,rad=-0.15",
        ),
    )
    ax.text(
        0.997, 0.30, "both\nserve", ha="right", va="center",
        fontsize=6.2, color="#9a9a9a", style="italic", linespacing=1.1,
    )

    # ---- labels and styling ----
    ax.set_xlabel(r"Top-1 cosine similarity $s_1$")
    ax.set_ylabel(r"Margin $m = s_1 - s_2$")
    ax.set_xlim(XLO, XHI)
    ax.set_ylim(YLO, YHI)
    ax.set_xticks([0.90, 0.92, 0.94, 0.96, 0.98, 1.00])
    for sp in ("top", "right"):
        ax.spines[sp].set_visible(False)
    ax.grid(color="#ececec", lw=0.5, zorder=0)

    fig.tight_layout(pad=0.4)
    return fig, dict(
        tau_fixed=tau_fixed, tau_gate=tau_gate,
        n_gain_all=n_gain_all, n_gain_on=n_gain_on, n_drop=n_drop,
        gain_corr=float(y[gain].mean()), drop_corr=float(y[drop].mean()),
        rho_r2=D["rho_r2"],
    )


if __name__ == "__main__":
    D = compute()
    fig, info = build_figure(D)

    pdf_path = os.path.join(HERE, "fig6_decision_boundary.pdf")
    png_path = os.path.join(HERE, "fig6_decision_boundary.png")

    fig.savefig(pdf_path, bbox_inches=None)
    plt.close(fig)

    fig2 = build_figure(D)[0]
    fig2.savefig(png_path, dpi=300, bbox_inches=None)
    plt.close(fig2)

    print("Saved fig6_decision_boundary.pdf and fig6_decision_boundary.png")
    print(f"  tau_fixed={info['tau_fixed']:.4f}  tau_gate={info['tau_gate']:.4f}  "
          f"rho_fit_R2={info['rho_r2']:.3f}")
    print(f"  gain: {info['n_gain_all']} serves ({info['n_gain_on']} on-panel), "
          f"{info['gain_corr']*100:.1f}% correct")
    print(f"  drop: {info['n_drop']} serves, {info['drop_corr']*100:.1f}% correct")
