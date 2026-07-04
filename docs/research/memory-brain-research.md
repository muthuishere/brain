# The Agent Brain — memory research → a CiteNexus design

> Research brief for a persistent, shared, outcome-aware **memory layer** ("brain")
> that agents like the crypto desk plug into: save every decision + outcome,
> retrieve the right past experience, and answer *"what have I learned."*
> Synthesized from a deep-research pass (24 sources, 109 extracted claims,
> adversarially verified — 0 refuted). Sources cited inline by name.

---

## Part 1 — How biological memory actually works (the parts that matter for design)

**Two complementary systems, not one store.** The brain deliberately splits memory
into a *fast* learner (hippocampus) that captures specific episodes on one exposure,
and a *slow* learner (neocortex) that extracts generalized/semantic structure across
many experiences. This **Complementary Learning Systems (CLS)** split exists
specifically to avoid *catastrophic interference* — a single fast distributed network
would overwrite prior knowledge every time it learns something new (McClelland,
O'Reilly CLS; Cell/Neuron consolidation reviews). **Design consequence: keep a fast
episodic store separate from a slow, distilled semantic store.**

**Consolidation = replay + transfer, mostly during sleep.** Freshly encoded engrams
decay unless stabilized. During NREM slow-wave sleep the hippocampus *replays*
recent firing patterns offline (with sharp-wave ripples, thalamic spindles,
neocortical slow oscillations) and gradually teaches them to the neocortex; each
reinstatement nudges neocortical synapses, so remote memory accumulates cortically
and is *transformed* into schema-like semantic representations. Removing those
oscillations degrades consolidation; alternating NREM/REM protects old memories from
being overwritten. Consolidation reshapes representation geometry — it increases
*within-category* similarity while leaving across-category similarity unchanged, i.e.
it **extracts shared structure** (biorxiv/PMC systems-consolidation studies).
**Design consequence: an offline "sleep" job that replays recent episodes and
distills them into semantic lessons is biologically the whole game.**

**Salience decides what sticks — and reward-prediction-error is the tag.** Dopamine
neurons encode a three-state **reward prediction error (RPE)**: activated by
better-than-predicted outcomes, baseline for predicted ones, depressed by
worse-than-predicted (Schultz). This RPE is a *teaching signal* that drives
plasticity in striatum/frontal cortex/amygdala — it is literally how reward
experience gets written. Critically for us:

- **Positive RPE (surprise), not reward magnitude, strengthens episodic encoding** of
  co-occurring information — and only when the prediction error coincides *in time*
  with the item. The boost is incidental (no intent to memorize needed) and fast
  (minutes, not consolidation-dependent).
- Positive vs negative errors are coded **asymmetrically** (excitation above baseline
  vs depression below) — two mechanistically distinct directions.
- The dopamine response transfers *backward* from the reward to the earliest
  predicting cue — exactly **temporal-difference credit assignment** (PMC dopamine
  reviews).

**Design consequence: weight and index memories by surprise/|RPE| (how far the
outcome deviated from expectation), tag the *cue that preceded* the outcome — not
just the outcome — and treat wins and losses as distinct signals.**

---

## Part 2 — The historical/theoretical canon (and the one idea we steal from each)

| Model | Core idea | What we steal |
|---|---|---|
| **Art of Memory / method of loci** (Simonides → Cicero → Yates) | Pair vivid images with stable spatial *places*; retrieve by walking the places in order. Encoding works best when images are **emotionally striking/distinctive**, not vague. | A **stable index (places) + written content (episodes)** separation, an explicit *store then retrieve* two-step API, and salience-boosted encoding — a 2,000-year-old anticipation of vector-keyed episodic memory. |
| **Ebbinghaus** (1885) | The **forgetting curve**: retention decays exponentially with time. | Model recency as **exponential decay**, not binary deletion. |
| **Atkinson–Shiffrin multi-store** (1968) | Sensory → short-term (~7±2 items, 15–30s) → long-term (unlimited, semantic-coded); rehearsal transfers STM→LTM. | The **tiered store** shape (working / long-term) and STM=recent-verbatim vs LTM=semantic-meaning coding. Criticized as oversimplified (rehearsal isn't strictly required) — so don't take the tiers as rigid. |
| **Baddeley & Hitch working memory** (1974, +episodic buffer 2000) | STM isn't one box: a **central executive** coordinates a phonological loop, visuospatial sketchpad, and an **episodic buffer** that binds modalities into unified episodes. | A **working-memory controller** that orchestrates sub-stores, and an **episodic buffer** that binds heterogeneous signals (price, text, chart) into one episode. |
| **Tulving episodic vs semantic** | Time-indexed *events* vs timeless *facts* — distinct systems. | The episodic/semantic split that every modern agent-memory taxonomy uses. |
| **Hebbian / connectionism** | "Cells that fire together wire together"; distributed learning. | Association by co-occurrence → the graph edges between episodes ("setups that co-occur"). |
| **CLS theory** (McClelland et al.) | Fast hippocampus + slow interleaved neocortex avoids catastrophic interference. | The load-bearing architecture: **fast episodic writes + slow consolidated semantic store.** |

---

## Part 3 — How AI agents do memory today (the proven patterns)

**The taxonomy converged on cognitive science.** Production agent-memory systems use
**episodic** (time-indexed events), **semantic** (facts/rules/preferences), and
**procedural** (skills/workflows) memory — most use all three, with episodic
*consolidated into* semantic over time (Serokell, Redis, O'Reilly practitioner
write-ups). Some add **working** (the context window) and **shared** (cross-agent)
as distinct lifecycles.

**The read/write loop.** Agents run *memory read → reason/plan → act → observe →
memory write*: retrieve before reasoning, extract facts and summarize after acting.

**Retrieval scoring = recency + importance + relevance.** The canonical formula is
**Generative Agents** (Park et al.): each memory in a natural-language "memory
stream" is retrieved by summing three weighted components — **recency** (exponential
decay, factor ~0.995), **importance** (the LLM rates each memory's "poignancy" 1–10),
and **relevance** (embedding similarity). Raw vector similarity alone is not enough;
**hybrid** (dense + term/full-text) beats either alone; add metadata filters
(recency, tags, agent/task scope).

**Consolidation & reflection (episodic→semantic).** Generative Agents periodically
**distill** raw memories into higher-level *reflections* when accumulated importance
crosses a threshold (150). Practitioners implement it by **clustering** related
episodes, **summarizing** them into one semantic unit, and **replacing** the
originals — a direct computational analogue of systems consolidation. Without
consolidation the store grows unbounded and retrieval quality degrades.

**MemGPT-style paging.** Split a fixed primary context (working memory the LLM sees)
from effectively-infinite external memory reached by explicit retrieval — an
OS-virtual-memory analogy. Write-back to persistent storage is triggered by **memory
pressure** (~70% of context), with the LLM deciding what to keep.

**Forgetting is the least-solved part.** Frame it as **budget-constrained
optimization**: under a fixed budget B, keep the subset that **maximizes cumulative
importance** rather than evicting chronologically. A cognitively-grounded value model
(novelty + task-significance + access-frequency + temporal decay; grounded in Craik's
Levels-of-Processing, McGaugh consolidation, Anderson adaptive forgetting)
probabilistically deletes low-value items while **consolidation-protecting**
high-value ones — and a *smaller* well-curated bank can **beat a larger unfiltered
one** on LongMemEval. Uncontrolled retention measurably hurts: on MultiWOZ, keep-all
yields 78.2% accuracy but a 6.8% **false-memory** rate.

**Two failure modes to design against:**
- **Experience-following / error propagation** (arXiv 2311.13743): agents imitate
  the output of whatever memory they retrieve — so a retrieved *bad* decision gets
  replicated and amplified. **The evaluator that gates what enters memory matters
  more than the LLM** (a fine-tuned judge on as few as 300 trajectories beats a
  vanilla LLM judge). Quality-gate writes.
- **Inconsistent shared state** is the dominant multi-agent failure — one study
  attributes **36.9%** of multi-agent failures to it. Scope and isolate shared memory
  hard (per project/task boundaries).

### The trading/RL-specific layer (this is your crypto desk)

- **Episodic memory as (state, action, outcome, time) tuples** — `(Sₜ, Aₜ, Oₜ₊ₖ, τ)`
  — the exact structure for saving a decision *plus its later realized outcome*
  (trading-agent memory surveys). Retrieval applies **exponential time decay**
  `e^(−λ(t_now−t_k))` to prefer recent market regimes.
- **Prioritized experience replay** (Schaul et al., DQN): uniform replay is
  suboptimal because it ignores which transitions matter; replaying *important*
  (high-error) transitions more often beat uniform replay on **41/49 Atari games**.
  → prioritize high-|RPE| episodes.
- **Losses trigger the update, asymmetrically.** **FinCon** learns by *comparing
  consecutive episodes*, distilling insights from more- vs less-profitable ones
  (Conceptual Verbal Reinforcement) — and when performance drops below the prior
  episode it **fires a manager self-reflection that writes corrective text**. It
  keeps working + procedural (per-decision-step action+outcome) + episodic (PnL
  series + updated beliefs, held only by the manager) memory. **FinMem / FinAgent**
  add a tunable "cognitive span," dual-level reflection, and multimodal (numeric +
  text + Kline-chart) event memory; FinAgent reports **>36% avg profit improvement**
  over 9 baselines. FinCon replaces gradient descent with an **action-overlap
  heuristic** as its "belief-update rate."
- **Hindsight relabeling turns losing trajectories into training signal**
  (AgentHER): a failed trajectory is relabeled as a *correct* demo for whatever goal
  it actually achieved — credit assignment at the **goal level** (hold the trajectory
  fixed, rewrite the goal). Mining failures gives +7–12pp on WebArena/ToolBench and
  reaches baseline with **half** the successful demos; a verification judge keeps
  relabels ≥94% precise.
- **Two trading-specific traps.** (1) **Oracle Fallacy / Outcome Embargo**: never let
  a retrieved past episode leak its *future* outcome — bar an episode recorded at `t`
  from exposing its outcome until `t_now ≥ t+k`, or you train on lookahead bias.
  (2) **Reflexion-style pre-act self-critique fits poorly** because market feedback
  is *delayed* — reflection must be outcome-triggered, not immediate.

---

## Part 4 — The CiteNexus "brain": a concrete design

The beautiful part: **most of this maps onto primitives CiteNexus already has.** An
*episode* is essentially an Evidence Unit with an outcome; *distilled lessons* are
exactly what the existing `LLMGraphDistiller` / wiki distillers produce; *retrieval*
is the existing vector + BM25 + RRF fusion; *multi-agent scoping* is the existing
`PartitionPath`; *"setups like this"* is the existing `navigate-not-cite` graph.
The brain is a **new orchestration + an outcome/reflection layer**, not a rewrite.

### What to store — two tiers (CLS)

1. **Episodic store (fast, verbatim, append-only).** One record per decision:
   ```
   Episode {
     agent_id, desk, ts,
     cue:      { situation snapshot — features/observation at decision time },
     decision: { action, size, rationale text },
     outcome:  { realized PnL / win|loss, exit_ts, embargo_until },   # written later
     salience: |RPE| = |realized − expected|,                          # surprise
     embedding, tags, provenance                                       # verbatim, citeable
   }
   ```
   Episodes are Evidence Units → they inherit verbatim citation + provenance, so the
   brain can *answer with evidence* ("last 3 times this setup appeared: …"), not vibes.

2. **Semantic store (slow, distilled lessons).** Consolidation clusters similar
   episodes and distills a **Lesson**: "when {cue pattern}, {action} led to {outcome
   distribution}; prefer/avoid X." This is the answer to *"what have I learned."*
   Reuse the distiller seam; lessons are navigate-not-cite (they raise recall, the
   cited source stays the episode).

### Save / retrieve API (the "separate method" you asked about)

Not the current `MemoryStore` (that's per-conversation Q/A scratch — wrong shape).
A distinct surface, e.g.:

```python
brain = rag.brain(desk="crypto")                 # partition-scoped, shared across agents
ep_id = brain.remember(cue=…, decision=…, agent_id=…)     # write at decision time
brain.record_outcome(ep_id, pnl=…, exit_ts=…)             # write outcome later (sets salience, embargo)
hits  = brain.recall(cue, k=5, scope=…)                   # recency+importance+relevance, embargo-safe
lesson= brain.what_did_i_learn(about=…)                   # distilled semantic answer, cited to episodes
brain.consolidate()                                       # the "sleep" job: cluster → distill → prune
```

### The scoring (steal Generative Agents + Ebbinghaus + RPE)

`score = w_recency·e^(−λ·Δt) + w_importance·salience + w_relevance·sim(cue) [+ w_freq·access]`
— hybrid dense+BM25 retrieval, metadata pre-filter by `agent_id`/`desk`/`tags`,
and an **Outcome Embargo** filter so `recall()` never returns an episode whose
outcome hasn't matured (`t_now < embargo_until`) as a settled lesson.

### Win/loss handling (asymmetric, on purpose)

- **Losses teach more → weight them up.** Salience uses |RPE|, but consolidation
  fires a **corrective reflection on drawdowns** (FinCon), and losing trajectories
  are **hindsight-relabeled** into "what setup would have made this right" lessons
  rather than discarded.
- **Wins reinforce** the responsible cue→action edge.
- **Prioritized replay**: consolidation samples high-|RPE| episodes more often.

### Forgetting & consolidation (the "sleep" job)

Budget-constrained: under memory budget B, keep the subset maximizing cumulative
value `V = novelty + task-significance + access-freq − temporal-decay`;
probabilistically delete low-V episodes, **consolidation-protect** high-V ones and
anything already distilled into a lesson. Run it offline on a schedule (the desk
already has cron slots) — that *is* systems consolidation.

### Multi-agent shared brain

One brain per **desk** (partition), read/write by all its agents; **hard project
isolation** (crypto desk ≠ stock loop) since inconsistent shared state is the #1
multi-agent failure. `recall()` returns an agent's own + siblings' episodes,
pre-filtered by `agent_id`/`memory_type`/`task_id`. Quality-gate every write with an
evaluator (cheap distiller model) — because experience-following means a bad stored
decision propagates.

### Guardrails unique to a *trading* brain

- **Outcome Embargo** on retrieval (no lookahead).
- **Quality gate on write** (don't memorize a bad decision as if it were good).
- **Outcome-triggered reflection**, not pre-act self-critique (delayed feedback).
- Everything stays **evidence-first**: a retrieved lesson cites the episodes it was
  distilled from, so the desk can audit *why* the brain said what it said.

---

## Sources (by research angle)

- **Biological mechanisms:** PMC9636926, Cell/Neuron *S0896-6273(23)00201-5*,
  biorxiv *327445*, PMC4826767 (dopamine RPE / Schultz).
- **Historical canon:** SimplyPsychology (Atkinson–Shiffrin multi-store), Wikipedia
  (Baddeley working memory; Art of Memory), McClelland/O'Reilly CLS (ResearchGate).
- **AI-agent memory:** arXiv 2311.13743 (experience-following), arXiv 2407.06567
  (agent-memory survey), Serokell long-term-memory patterns.
- **RL / trading wins-losses:** arXiv 1511.05952 (Prioritized Experience Replay),
  arXiv 2603.21357, arXiv 2402.18485, arXiv 2505.16067, arXiv 2605.19337 (FinCon /
  FinMem / FinAgent family), Medium (LLM stock-trading memory tiers).
- **Retrieval / forgetting / sharing:** ar5iv 2304.03442 (Generative Agents), AWS S3
  Vectors multi-agent memory, O'Reilly memory-engineering, arXiv 2606.12945, Redis
  long-term-memory architectures, arXiv 2604.02280 (AgentHER hindsight relabeling).

*Method: deep-research harness — 5 angles, 24 sources fetched, 109 claims extracted,
25 sampled for 3-vote adversarial verification (0 refuted). Verdicts and full claim
set in the run journal.*
