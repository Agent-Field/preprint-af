# Narrative review (round 1)

**Score:** 0.850  
**Confident:** True

## Arc assessment

The paper constructs a clear analytical arc: it motivates the problem of unreliable absolute similarity for semantic caching, introduces margin as a decision-theoretic confidence signal, presents a lightweight gate that leverages this signal, demonstrates improved hit-rate/reliability trade-offs, and isolates margin as the dominant contributor through ablation. The sequence builds belief in the main claim efficiently, though a sharper transition from architecture to ablation and from results to regime analysis would tighten the narrative. Overall, the story is coherent and the evidence mounts progressively.

## Promise alignment issues

- The abstract promises 'ablation shows that the margin alone recovers 83% of the total gain,' which the body delivers via the 1% budget ablation statement. However, the title 'Cosine Margin Predicts Semantic Cache Reliability' implies a broad claim beyond the QQP/MiniLM setting that the text does not fully support with evidence beyond the studied datasets and models; the body's limitations section acknowledges this, but the front matter does not qualify the scope, creating mild over-delivery.
- The abstract states 'the gate runs in 0.8 µs,' which is delivered in the body, but the abstract does not mention that this latency is measured under specific conditions (a single run with zero standard deviation) and for a particular model, which the body clarifies. This is a minor over-delivery of precision.
- The abstract mentions 'outperforming the vCache baseline' without specifying that vCache is used as a fixed-encoder, count-based baseline in the same feature space; the body clarifies this but the front matter implies a direct, unqualified comparison to the full vCache system as published, which may be over-delivering on the comparison scope.

## Transition issues

| Section | Severity | Issue | Fix hint |
|---|---|---|---|
| sections/04_architecture.tex | minor | The section ends by introducing the edge over similarity-only gating but does not logically bridge to the ablation experiments that begin section 05. The reader expects a statement like 'to quantify how much each feature contributes, we conduct an ablation study' to connect the architecture to the experimental design. | Add a sentence at the end of section 04 that explicitly announces the ablation-motivated decomposition and links to section 05. |
| sections/06_analysis.tex | minor | The analysis section opens by describing the high-recurrence regime without first recapitulating how the previous section established the overall performance edge. The transition from section 05's main results to the regime analysis is abrupt. | Open section 06 with a sentence that summarizes the main result from section 05 and states the goal of characterizing when that gain is largest. |

