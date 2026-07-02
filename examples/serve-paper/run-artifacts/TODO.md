- Provide the results/results.json file referenced by all figure scripts; it is not present in input/. This file contains per-seed tradeoff curves, margin mechanism decile data, recurrence sweep arrays, and grid cell data. Without it, all quantitative claims in the paper cannot be independently reproduced.
- Provide the data/qqp_minilm.npz file referenced by fig6_decision_boundary.py; it is not present in input/. This file contains the embedded QQP dataset used to train the gate and compute the decision boundary visualization.
- Resolve the minor numerical discrepancy between the recurrence x budget grid numbers in main.tex (+214.8% at r=0.8, beta<=0.25%, with fixed=0.0223, SERVE=0.0701) and those in fig4_best_regime_grid.py (+205.3% at r=0.8, beta<=0.25%, with fixed=0.0231, SERVE=0.0707), which both claim to represent the same experiment but differ.
- Resolve minor numerical discrepancies in the ablation table between main.tex (s1+m=0.0363, total=0.0378) and fig1_sota_ablation.py (s1+m=0.0364, total=0.0379), which affect the derived percentage increments.
- Provide per-seed raw values and standard deviations for the PAWS results, which are currently from a single held-out split with no seed averaging.
- Run experiments on additional embedding families and domains (code search, multilingual, RAG corpora) to test whether the margin advantage transfers beyond the two encoders (MiniLM, mpnet) tested.
- Measure production traffic directly (FAQ/customer-support caches) to validate that the high-recurrence, strict-budget regime characterization holds outside the artificially controlled recurrence sweeps.
- Run statistical significance tests (not just 4-seed means with spread) on the primary SOTA comparison at beta <= 1%.
- Study online gate refresh for live, drifting cache deployments rather than the current one-time-train-per-configuration protocol.
- Compare against the original vCache codebase (not only a fair reimplementation) to validate that the reimplementation faithfully reproduces vCache behavior.

## Bibliography items deferred (could not verify)

- Related work on query recurrence in caching systems: No standalone paper found that specifically studies query recurrence patterns in LLM semantic caches beyond the three foundational systems already cited (GPTCache, MeanCache, vCache). The concept of query recurrence is discussed within those papers themselves. If a specific paper on temporal locality or query repetition patterns in LLM serving is desired, a more targeted search (e.g., for "query locality LLM serving" or "cache hit ratio LLM traffic patterns") may yield results.
- Prior work on using tree ensembles for hit/miss prediction in caches: Explicitly noted as "none found" per the original needs list — this is a novelty claim of the paper. No reference to add.
