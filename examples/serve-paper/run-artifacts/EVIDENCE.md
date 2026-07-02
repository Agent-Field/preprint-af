# EVIDENCE.md — Fact Ledger for "Similarity Is Not Enough: Margin-Aware Reuse Decisions for Semantic LLM Caches"

## Facts

### E1
- **Statement:** SERVE uses a gradient-boosted gate (GradientBoostingClassifier, 150 estimators, max depth 3) over a 4-dimensional feature vector: top-1 similarity (s1), margin (m = s1 - s2), runner-up similarity (s2), and local density (ρ, mean similarity over k=8 nearest cache entries).
- **Numbers:** 4 features, 150 estimators, max depth 3, k=8.
- **Source:** `input/main.tex` (lines 275–293).
- **Derivation:** Direct read from the main.tex problem-formulation section.

### E2
- **Statement:** On 100k MiniLM QQP embeddings (60k cache, 20k gate-train, 20k test, 4 seeds), SERVE improves hit-rate at matched false-serve budgets over the fixed-threshold and vCache baselines.
- **Numbers:**
  - β ≤ 0.5%: fixed=0.0202, vCache=0.0216, SERVE=0.0238 (rel. +17.9%, +9.9%)
  - β ≤ 1%: fixed=0.0338 (std 0.0019), vCache=0.0355 (std 0.0005), SERVE=0.0378 (std 0.0017) (rel. +11.9%, +6.5%)
  - β ≤ 2%: fixed=0.0553, vCache=0.0564, SERVE=0.0590 (rel. +6.6%, +4.6%)
  - β ≤ 5%: fixed=0.0983, vCache=0.0993, SERVE=0.0993 (rel. +1.0%, +0.1%)
- **Source:** `input/main.tex` (Table tab:sota, lines 444–458; prose lines 409–414, 421–422).
- **Derivation:** Direct read from the paper's SOTA comparison table and surrounding prose.

### E3
- **Statement:** Aggregate ROC-AUC is nearly identical across all three methods: fixed=0.8948, vCache=0.8963, SERVE=0.8967.
- **Numbers:** 0.8948, 0.8963, 0.8967.
- **Source:** `input/main.tex` (line 429).
- **Derivation:** Direct read from the prose discussion of AUC.

### E4
- **Statement:** Feature ablation at β ≤ 1% shows that the margin alone recovers 83% of SERVE's total improvement over a similarity-only gate.
- **Numbers:**
  - s1 only: 0.0292
  - s1 + m: 0.0363 (+24.4%)
  - s1 + m + s2: 0.0374 (+2.9%)
  - s1 + m + s2 + ρ: 0.0378 (+1.1%)
  - Margin recovery: (0.0363 − 0.0292) / (0.0378 − 0.0292) ≈ 0.83
- **Source:** `input/main.tex` (Table tab:ablation, lines 604–614; prose lines 588–595).
- **Derivation:** Direct read from the ablation table and prose; the 83% ratio is explicitly stated and verified by arithmetic on the table values.

### E5
- **Statement:** Gate overhead is 0.8 μs per query in batched inference (std 0.0 across 4 runs) on an Apple M-series CPU.
- **Numbers:** 0.8 μs, std 0.0.
- **Source:** `input/main.tex` (line 396).
- **Derivation:** Direct read from the experimental setup section.

### E6
- **Statement:** SERVE's hit-rate gain over the fixed threshold at β ≤ 1% grows monotonically with query recurrence r from +21.3% at r=0.2 to +81.0% at r=0.9.
- **Numbers:**
  - r=0.2: fixed=0.0233, SERVE=0.0283 (+21.3%)
  - r=0.3: +32.8%
  - r=0.4: +36.1%
  - r=0.5: fixed=0.0520, SERVE=0.0750 (+44.3%)
  - r=0.6: +53.3%
  - r=0.7: fixed=0.0654, SERVE=0.1110 (+69.9%)
  - r=0.8: +77.0%
  - r=0.9: fixed=0.0789, SERVE=0.1428 (+81.0%)
- **Source:** `input/main.tex` (lines 491–496).
- **Derivation:** Direct read from the recurrence sweep prose.

### E7
- **Statement:** In the recurrence×budget grid, the strictest-budget highest-recurrence cells show SERVE serving roughly 2.6–3.15× the hits of the fixed threshold.
- **Numbers:**
  - r=0.8, β ≤ 0.25%: fixed=0.0223, SERVE=0.0701 (+214.8%, 3.15× ratio)
  - r=0.9, β ≤ 0.25%: fixed=0.0222, SERVE=0.0692 (+211.8%)
  - r=0.5, β ≤ 0.25% through β ≤ 1% range: gain from +44.4% to +214.8%
- **Source:** `input/main.tex` (lines 507–521, Fig.~\ref{fig:grid}).
- **Derivation:** Direct read from the grid discussion section. Also referenced in the abstract (lines 46–49) with the same numbers.

### E8
- **Statement:** At equal top-1 similarity, queries with high margin are consistently more likely to be correct serves than queries with low margin, demonstrated by decile-level conditional-correctness analysis at seed 0.
- **Numbers:**
  - Decile midpoint 0.723: P(correct) = 0.071 (high margin) vs. 0.024 (low margin)
  - Decile midpoint 0.823: 0.270 vs. 0.176
  - Decile midpoint 0.921: 0.592 vs. 0.481
  - Highest decile: 0.796 vs. 0.642 (seed 0)
  - Highest-decile gap averaged over 4 seeds: +17 pp
  - Same ordering in every seed.
- **Source:** `input/main.tex` (lines 566–574; figure caption lines 155–166). Also `input/figures/fig1_mechanism.py` (lines 58–66) which loads per-seed decile data from `results/results.json` key `margin_mech.seeds[*]`.
- **Derivation:** Direct read from the paper's conditional-correctness analysis prose and Fig. 1 caption. The figure script confirms the per-seed data source.

### E9
- **Statement:** On the PAWS external adversarial benchmark (single held-out test split, n=8,000), the fixed similarity rule inverts below chance (AUC=0.1408) with zero achievable hits at any false-serve budget up to 5%, while SERVE trained normally reaches AUC=0.9025.
- **Numbers:**
  - Fixed AUC: 0.1408 (below chance 0.5)
  - SERVE AUC: 0.9025
  - Sign-corrected oracle AUC: 1 − 0.1408 = 0.8592
  - Fixed hit-rate at every β ∈ {0.5%, 1%, 2%, 5%}: 0.0000
  - SERVE hit-rates: 0.0025 (β ≤ 0.5%), 0.0053 (β ≤ 1%), 0.0100 (β ≤ 2%), 0.0215 (β ≤ 5%)
  - Test split: n=8,000; 44.20% paraphrases; 5.78% correct-to-serve
- **Source:** `input/main.tex` (lines 616–658, Fig.~\ref{fig:paws}). `input/figures/fig5_paws.py` (lines 35–36) contains the same AUC values hardcoded.
- **Derivation:** Direct read from the PAWS section of main.tex. The sign-corrected oracle bound is explicitly derived as 1 − AUC_fixed by ROC symmetry.

### E10
- **Statement:** PAWS ablation at β ≤ 1% shows similarity alone=0.0020, adding margin raises to 0.0045 (+125.0%); s2 contributes −8.3% (to 0.0041), and ρ contributes +24.2% (to 0.0051).
- **Numbers:** 0.0020 → 0.0045 → 0.0041 → 0.0051.
- **Source:** `input/main.tex` (lines 653–656).
- **Derivation:** Direct read from the PAWS section.

### E11
- **Statement:** Embedding robustness test with mpnet (768d) vs MiniLM (384d) at β ≤ 1% shows SERVE maintains a +10.6% relative gain over the fixed threshold, comparable to the +11.8% gain on MiniLM.
- **Numbers:**
  - MiniLM (384d): fixed=0.0338, SERVE=0.0378 (+11.8%)
  - mpnet (768d): fixed=0.0351, SERVE=0.0388 (+10.6%)
- **Source:** `input/main.tex` (Table tab:robustness, lines 729–737). `input/figures/fig2_embedding_quality.py` (lines 38–40) contains slightly different hardcoded values (SERVE=0.0377 for MiniLM, 0.0383 for mpnet, +11.6% and +9.0%) from an independent stand-alone run.
- **Derivation:** Direct read from the robustness table in main.tex.

### E12
- **Statement:** The QQP pipeline embeds 100k questions with sentence-transformers/all-MiniLM-L6-v2 (384d), partitioned into 60k cache, 20k gate-training, and 20k held-out test splits over 4 random seeds.
- **Numbers:** 100k embeddings, 60k cache, 20k train, 20k test, 4 seeds.
- **Source:** `input/main.tex` (lines 362–366).
- **Derivation:** Direct read from the experimental setup section.

### E13
- **Statement:** An additional candidate feature, cache-entry hubness (nearest-neighbor in-degree over the query population), was implemented and ablated identically but found neutral to slightly negative; it was dropped from the final feature vector.
- **Numbers:** Effect: neutral to slightly negative on hit-rate at matched false-serve budget.
- **Source:** `input/main.tex` (lines 761–774).
- **Derivation:** Direct read from the Discussion section.

### E14
- **Statement:** SERVE's features (s1, m, s2, ρ) are all byproducts of the top-k nearest-neighbor search already performed by the cache; none require additional embedding calls or index lookups.
- **Numbers:** Zero additional embedding calls or index lookups.
- **Source:** `input/main.tex` (lines 98–103, 275–293, 742–751).
- **Derivation:** Direct read from multiple sections of main.tex stating this design property.

### E15
- **Statement:** The margin m = s1 − s2, runner-up similarity s2, and local density ρ are produced and then discarded by every deployed semantic cache (GPTCache, MeanCache, vCache), which all make the reuse decision from s1 alone.
- **Numbers:** None.
- **Source:** `input/main.tex` (lines 85–106, 173–201).
- **Derivation:** Direct read from the Introduction and Related Work sections describing incumbent system architectures.

## Figure candidates

- Hit-rate vs. false-serve rate operating frontier for fixed threshold, vCache, and SERVE on QQP/MiniLM (4 seeds, 60k cache), with strict-budget region shaded. Source: `input/figures/fig2_frontier.py` (loads from `results/results.json` key `tradeoff`).
- SERVE relative gain vs. false-serve budget (line plot with markers for vs-Fixed and vs-vCache). Source: `input/figures/fig2_frontier.py` (hardcoded right-panel numbers from main.tex Table tab:sota).
- Conditional correctness P(top-1 correct) vs. top-1 similarity decile midpoint, split by above-median/below-median margin, mean over 4 seeds with min-max bands, plus mechanism schematic. Source: `input/figures/fig1_mechanism.py` (loads from `results/results.json` key `margin_mech`).
- Grouped bar chart: hit-rate at matched β for fixed, vCache, SERVE across β ∈ {0.5%, 1%, 2%, 5%} plus cumulative feature ablation bar chart at β ≤ 1%. Source: `input/figures/fig1_sota_ablation.py` (hardcoded numbers from main.tex).
- Diverging absolute hit-rate vs. query recurrence r at β ≤ 1% for fixed threshold and SERVE, 4 seeds with min-max bands. Source: `input/figures/fig3_recurrence.py` (loads from `results/results.json` key `recurrence`).
- Heatmap of SERVE relative gain over fixed threshold across recurrence×budget grid (r ∈ {0.5, 0.7, 0.8, 0.9}, β ∈ {0.25%, 0.5%, 1%}). Source: `input/figures/fig4_best_regime_grid.py` (hardcoded grid values) and `input/figures/fig4_grid.py` (loads from `results/results.json` key `grid`).
- Horizontal bar chart: ROC-AUC on PAWS for fixed threshold (raw similarity), sign-corrected oracle bound, and SERVE. Source: `input/figures/fig5_paws.py` (hardcoded AUC values from main.tex).
- Decision-boundary scatter in (s1, m) space showing SERVE-gain points (high margin, s1 below τ) and SERVE-drop points (low margin, s1 above τ), computed from a trained gate at β ≤ 1%, seed 0. Source: `input/figures/fig6_decision_boundary.py` (loads `data/qqp_minilm.npz` and trains gate from scratch).
- Grouped bar chart: hit-rate at β ≤ 1% for fixed vs. SERVE across MiniLM (384d) and mpnet (768d) embeddings. Source: `input/figures/fig2_embedding_quality.py` (hardcoded numbers).

## Gaps

- Provide the `results/results.json` file referenced by all figure scripts; it is not present in `input/`. This file contains per-seed tradeoff curves, margin mechanism decile data, recurrence sweep arrays, and grid cell data. Without it, all quantitative claims in the paper cannot be independently reproduced.
- Provide the `data/qqp_minilm.npz` file referenced by `fig6_decision_boundary.py`; it is not present in `input/`. This file contains the embedded QQP dataset used to train the gate and compute the decision boundary visualization.
- Resolve the minor numerical discrepancy between the recurrence×budget grid numbers in `main.tex` (+214.8% at r=0.8, β≤0.25%, with fixed=0.0223, SERVE=0.0701) and those in `fig4_best_regime_grid.py` (+205.3% at r=0.8, β≤0.25%, with fixed=0.0231, SERVE=0.0707), which both claim to represent the same experiment but differ.
- Resolve minor numerical discrepancies in the ablation table between main.tex (s1+m=0.0363, total=0.0378) and `fig1_sota_ablation.py` (s1+m=0.0364, total=0.0379), which affect the derived percentage increments.
- Provide per-seed raw values and standard deviations for the PAWS results, which are currently from a single held-out split with no seed averaging.
- Run experiments on additional embedding families and domains (code search, multilingual, RAG corpora) to test whether the margin advantage transfers beyond the two encoders (MiniLM, mpnet) tested.
- Measure production traffic directly (FAQ/customer-support caches) to validate that the high-recurrence, strict-budget regime characterization holds outside the artificially controlled recurrence sweeps.
- Run statistical significance tests (not just 4-seed means with spread) on the primary SOTA comparison at β ≤ 1%.
- Study online gate refresh for live, drifting cache deployments rather than the current one-time-train-per-configuration protocol.
- Compare against the original vCache codebase (not only a fair reimplementation) to validate that the reimplementation faithfully reproduces vCache behavior.

## Existing draft

An existing LaTeX draft is present at `input/main.tex` (830 lines, IEEEtran format). It is a complete paper titled "Similarity Is Not Enough: Margin-Aware Reuse Decisions for Semantic LLM Caches." The draft makes the following claims: (1) deployed semantic caches (GPTCache, MeanCache) and the strongest learned refinement (vCache) make reuse decisions from top-1 similarity alone, discarding the runner-up similarity and the margin between them; (2) SERVE, a gradient-boosted gate over four already-computed retrieval-geometry features (s1, m, s2, ρ), improves hit-rate at matched false-serve budgets, by +11.9% over the fixed threshold at β ≤ 1% on QQP/MiniLM; (3) the gain grows monotonically with query recurrence and compounds with budget strictness, exceeding 3× in the strictest highest-recurrence cells; (4) the margin alone accounts for 83% of the gain; (5) on PAWS, the fixed rule inverts below chance (AUC 0.1408) while SERVE reaches AUC 0.9025; (6) the gate costs 0.8 μs per query and adds no retrieval work. The draft's structure follows a standard pattern: Introduction, Related Work, Problem Formulation and Method, Experimental Setup, Results (4 subsections plus robustness/cost), Discussion, Limitations, Conclusion.

All quantitative claims in the draft trace to numeric values stated in the draft itself, but the underlying raw data files (`results/results.json`, `data/qqp_minilm.npz`) are absent from `input/`. The claims are internally consistent but cannot be independently verified from the files provided. The draft is well-structured and evidence-anchored for each claim; the prose follows a disciplined scientific style. To rebuild: the underlying results data must be located and included; the minor numerical discrepancies between main.tex and the figure scripts should be reconciled; and the PAWS single-split limitation should be addressed with multi-seed runs. The draft's structure, narrative arc, and figure plan are sound and should be retained.

## Citation inventory

- bang2023gptcache (Bang, 2023) — "GPTCache: An Open-Source Semantic Cache for LLM Applications Enabling Faster Answers and Cost Savings"
- gill2025meancache (Gill et al., 2025) — "MeanCache: User-Centric Semantic Caching for LLM Web Services"
- schroeder2026vcache (Schroeder et al., 2026) — "vCache: Verified Semantic Prompt Caching"
- zhang2019paws (Zhang et al., 2019) — "PAWS: Paraphrase Adversaries from Word Scrambling"
- reimers2019sentencebert (Reimers & Gurevych, 2019) — "Sentence-BERT: Sentence Embeddings using Siamese BERT-Networks"
- wang2020minilm (Wang et al., 2020) — "MiniLM: Deep Self-Attention Distillation for Task-Agnostic Compression of Pre-Trained Transformers"
- song2020mpnet (Song et al., 2020) — "MPNet: Masked and Permuted Pre-training for Language Understanding"
- friedman2001greedy (Friedman, 2001) — "Greedy Function Approximation: A Gradient Boosting Machine"
- quora2017qqp (DataCanary et al., 2017) — "Quora Question Pairs"
- chow1970reject (Chow, 1970) — "On Optimum Recognition Error and Reject Tradeoff"
- radovanovic2010hubs (Radovanović et al., 2010) — "Hubs in Space: Popular Nearest Neighbors in High-Dimensional Data"