# Title: Cosine Margin Predicts Semantic Cache Reliability

## Final abstract

Semantic caches reduce large language model serving costs by reusing responses to similar past queries, but their practical adoption is limited by the risk of serving incorrect answers. Existing gating methods rely on absolute similarity scores, which are unreliable predictors of semantic equivalence in high-dimensional embedding spaces. We argue that the decision-theoretic margin—the gap between the top-1 and runner-up match similarities—captures classification confidence and is a more robust signal for cache admission. We propose a lightweight gradient-boosted gate operating on the top-1 similarity, cosine margin, runner-up similarity, and local embedding density. Trained on QQP, the gate runs in 0.8 µs and achieves a hit rate of 0.0378 at a strict 1% false-serve budget, an 11.9% improvement over a similarity-only threshold and outperforming the vCache baseline. Ablation shows that the margin alone recovers 83% of the total gain. The result demonstrates that incorporating a simple, geometrically motivated signal can substantially tighten the hit-rate–reliability frontier for LLM caching.

## Opening thesis

In high-dimensional embedding spaces, similarity to a nearest neighbor is a poor proxy for semantic equivalence—what matters is whether that neighbor stands clearly apart from the rest.

## Contribution order

1. Identifying cosine margin as a first-order signal for semantic cache reliability, grounded in decision-theoretic nearest-neighbor classification.
2. Proposing a four-feature, sub-microsecond gradient-boosted gate that explicitly controls false-serve rate.
3. Demonstrating consistent hit-rate improvements over fixed-threshold and vCache baselines at every reliability budget, with margin contributing 83% of the gain.

## Winning frame: Mechanism-Insight

The Mechanism-Insight frame wins because it offers the most defensible contribution within the crowded reliability-caching space. The core observation—that the cosine margin between top-1 and runner-up similarities reflects decision confidence in a way that absolute similarity does not—is a genuine conceptual insight unoccupied in prior work. This decision-theoretic framing is not merely a new knob; it recharacterizes the cache gating problem. Coupled with the gradient-boosted gate that runs in under a microsecond, the paper delivers both an explanatory mechanism and a practical artifact. Critically, this framing avoids the synthetic-reliability trap of Frame 1 and the triviality charge against Frame 2. While it carries medium overlap risk with vCache, the margin signal itself is uncontested and the allocation-of-credit ablation (83% of gain from margin) provides compelling evidence for its primacy. The low-confidence score reflects the risk that reviewers still see the overall architecture as incremental over vCache despite the distinct decision-theoretic lens.

## Judgments

| Frame | Persona | Comprehension | Excitement | Credibility | Naturalness |
| --- | --- | --- | --- | --- | --- |
| Reliability-First | Busy area chair judging placement and defensibility at a high-impact ML systems venue | 0.80 | 0.50 | 0.70 | 0.40 |
| Reliability-First | Interdisciplinary editor evaluating broad import for a high-impact ML systems venue | 0.90 | 0.40 | 0.60 | 0.30 |
| Reliability-First | Senior scientist in ML systems | 0.90 | 0.75 | 0.80 | 0.70 |
| Reliability-First | skeptical methods reviewer | 0.90 | 0.50 | 0.30 | 0.35 |
| Methodology-Simplicity | I'm a senior systems researcher who has reviewed dozens of ML caching/optimization papers. I care about real latency numbers | 0.95 | 0.60 | 0.55 | 0.40 |
| Methodology-Simplicity | Interdisciplinary editor at a high-impact ML systems venue. I judge whether the first page signals broad importance to readers outside the immediate subfield. I value clear | 0.90 | 0.50 | 0.80 | 0.60 |
| Methodology-Simplicity | Senior Area Chair | 0.90 | 0.65 | 0.85 | 0.70 |
| Methodology-Simplicity | skeptical methods reviewer | 0.80 | 0.40 | 0.60 | 0.30 |
| Mechanism-Insight | Reviewer 2 | 0.80 | 0.40 | 0.30 | 0.70 |
| Mechanism-Insight | Senior editor at a high-impact ML systems venue | 0.90 | 0.70 | 0.65 | 0.50 |
| Mechanism-Insight | busy area chair | 0.80 | 0.50 | 0.40 | 0.20 |
| Mechanism-Insight | skeptical methods reviewer | 0.85 | 0.55 | 0.40 | 0.50 |
| Recurrence-Robustness | An interdisciplinary editor judging whether the first page signals broad importance to readers outside the immediate subfield. | 0.65 | 0.70 | 0.35 | 0.25 |
| Recurrence-Robustness | Senior domain expert in ML systems and semantic caching | 0.40 | 0.50 | 0.30 | 0.30 |
| Recurrence-Robustness | busy area chair judging placement and defensibility | 0.70 | 0.50 | 0.60 | 0.20 |
| Recurrence-Robustness | skeptical-methods-reviewer | 0.70 | 0.30 | 0.20 | 0.10 |

## Rejected alternatives

- Reliability-First: The reliability-guarantee framing is crowded by vCache and GroundedCache; the benchmark-derived false-serve rate does not convincingly map to real-world deployment risk.
- Methodology-Simplicity: The four-feature gate, while practical, reads as an incremental application of XGBoost rather than a research contribution with mechanistic insight.
- System-Throughput: The discussion of popularity skew is a fresh angle but unsupported by end-to-end system measurements; the claim of high-throughput impact is not substantiated by the available evidence.

## Novelty scan

Confident: True

Closest related titles:
- vCache: Verified Semantic Prompt Caching
- Grounded Cache Routing for Retrieval-Augmented Generation: When Is It Safe to Reuse an Answer?
- Category-Aware Semantic Caching for Heterogeneous LLM Workloads
- MeanCache: User-Centric Semantic Caching for LLM Web Services
- GPT Semantic Cache: Reducing LLM Costs and Latency via Semantic Embedding Caching
- From Exact Hits to Close Enough: Semantic Caching for LLM Embeddings
- Semantic Caching for Low-Cost LLM Serving: From Offline Learning to Online Adaptation
- SCALM: Towards Semantic Caching for Automated Chat Services with Large Language Models
- Krites: Asynchronous Verified Semantic Caching for Tiered LLM Architectures
- Proximity: Leveraging Approximate Caching for Faster Retrieval-Augmented Generation
- When Classic Cache Policies Fail: Learning-Augmented Replacement for Semantic Retrieval Buffers
- A Generative Caching System for Large Language Models
- ContextCache: Context-Aware Semantic Cache for Multi-Turn Queries in Large Language Models
- LLMs for Test Input Generation for Semantic Caches

Collision risks:
- HIGH: Frame 1 (Margin-Aware Gating for Reliable Caching) directly overlaps with vCache (ICLR 2026), which also proposes user-defined error-rate guarantees via per-prompt threshold learning. GroundedCache additionally proposes controlled unsafe-served rate as a first-order metric. The reliability-guaranteed caching framing is increasingly crowded.
- MEDIUM-HIGH: Frame 2 (4D Margin-Aware Classifier) names vCache as its explicit baseline, making overlap acknowledged. The open question is whether a 4D gradient-boosted gate offers enough improvement over vCache's per-prompt online learning to earn a distinct contribution claim.
- MEDIUM: Frame 3 (Similarity Is Not Enough) shares its core motivation with both vCache and GroundedCache, which explicitly argue static similarity thresholds are unreliable. The novelty must rest entirely on the margin signal rather than the observation itself.
- LOW-MEDIUM: Frame 4 (Cache Gating Loves Popularity) has the least direct competition. Category-Aware caching analyzes query recurrence patterns but does not center the interaction between gating mechanism and recurrence. SOLAR touches on frequency concentration but not gating.

Open positioning angles:
- The cosine margin signal (distance to 1st nearest neighbor minus distance to 2nd nearest neighbor) as a gating primitive is genuinely unoccupied in the LLM cache literature. No existing paper uses this signal for cache decisions despite its roots in decision-theoretic nearest-neighbor classification.
- Decision-theoretic framing of cache reuse decisions is absent. Existing papers frame the problem as engineering reliability or systems optimization, not as a statistical decision problem where the margin captures classification confidence.
- Explicit false-serve-rate vs. hit-rate Pareto frontier as a user-tunable optimization target is underexplored. vCache and GroundedCache control error rate but do not map the full tradeoff curve or treat it as formal multi-objective optimization.
- Gradient-boosted tree classifier as a sub-microsecond cache gate is uncontested. No prior work proposes tree-ensemble classifiers for cache hit/miss decisions, offering a novel engineering contribution orthogonal to online learning or neural approaches.
- Cache gating as an orthogonal, pluggable component decoupled from cache structure is underexplored. Most papers tightly couple gating to the cache implementation; a standalone gate compatible with any cache backend is a distinctive architectural position.
- Query recurrence as a first-order dimension of gating effectiveness analysis. The interaction between how often queries reappear and how well gating captures them is not a central thesis in any existing paper, making it a fresh sub-angle.
