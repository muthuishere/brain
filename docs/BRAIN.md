# CiteNexus Brain — architecture & build spec

> A brain, not a memory store. It has a **point of view** (convictions), a
> **direction** (goals), and the machinery to keep them honest against experience
> and against the things it must never do. The RAG (this repo) is the substrate;
> the brain is the thin active layer that gives it a self.
>
> Research basis: `docs/research/memory-brain-research.md` (biology),
> `docs/research/brain-design.md` (memory systems + competitors). This file is
> what we build.

## 1. The one belief that defines it

**The brain's only job is signal-to-noise.** A single experience is noise — a win
may be edge or luck, a loss may be process or variance. The *signal* — what
"defines you" — is the pattern that survives repetition **and** survives
reappraisal. A conviction is a belief that kept being right after every chance to
be wrong. Everything else is held but never acted on.

You don't lose money by forgetting. You lose money by **acting on noise you
mistook for signal** (the "experience-following" failure). So the brain's core
discipline is: *refuse to elevate noise to conviction, and stay silent on the
noise.* Cite-or-abstain, applied to beliefs — the same guarantee CiteNexus already
ships, pointed at experience instead of documents.

Signal is only definable relative to a goal. So the brain is organised around
**goals**, and goals come in two kinds, exactly as in a human:

- **Foreground objectives** — fast, streamed in continuously ("what I want now"),
  replace-on-arrival. Signal/noise is measured against the *current* objective.
- **Background constraints** — slow, few, always-on, **veto power** ("who I am /
  what I'll never do"). Rarely change. Nothing is signal if it breaks these.

The killer behaviour: the brain measures signal against the foreground objective
but silently checks every decision against the backgrounded constraints, and gets
**loud** the moment something scores as signal on the objective while threatening a
constraint — *"yes, this made money; it also violated the thing you said you'd
never do."* That's "winning is actually losing," detected mechanically. No memory
store can say it.

## 2. Grounding in how a human works (and where we beat the human)

- A human holds many goals as a **hierarchy** by rate-of-change: a slow core
  (survive, don't blow up, stay who I am) that holds veto, and a fast foreground
  that rotates. One goal is foregrounded; the rest constrain from the background.
- Reappraisal is mostly **silent and unconscious** (reconsolidation quietly
  rewrites the past); big, high-stakes contradictions **break into awareness** and
  get deliberated. Salience is the switch: quiet on the noise, loud on the
  convictions.
- **We beat the human on one thing:** humans don't keep the audit trail —
  reconsolidation overwrites, which is why hindsight bias exists ("I always knew").
  Our brain quiet-updates on the small stuff and escalates on the big stuff **but
  keeps every prior verdict**, so it can show you *"I used to believe this; here's
  when and why it changed."* A human can't.

## 3. The four pillars (each published; the composition is novel)

| Pillar | What it is | Prior art | Ours |
|---|---|---|---|
| **Foreground objective** | goal-as-input; swap it → re-frame, no relearning; "intention that resists reconsideration" | BDI intentions; goal-conditioned policies | objective is streamed input; signal/noise measured against it |
| **Convictions** | LLM reflects experience → graded belief that strengthens with repetition & survives reappraisal | Generative Agents reflection × Bayesian updating | LLM proposes, deterministic code validates the signal (n≥k, consistent direction, survived contradiction) before it's trusted |
| **Auditable dormancy** | convictions stored with the *assumptions/goal-frame* that support them; retire the goal → convictions go dormant, not deleted; revivable | ATMS/JTMS truth-maintenance; Altmann-Trafton activation decay | goal-frame label = the assumption set; when a goal retires, the label points at exactly what to revisit; dormant-but-revivable |
| **Constraint shield** | constraints are a *separate cost channel with thresholds*, not blended into the objective; deterministic veto outside the reasoning loop | CMDP + shielding (Alshiekh 2018) | hard shields for inviolable constraints, soft penalties for bendable ones; fires on `reward≥high ∧ cost>threshold`; proposes a safe fallback |

**Why nobody has it:** Generative Agents has convictions but no veto; Voyager has
dormant-revivable *skills* but no beliefs/veto; Constitutional AI has the standing
constraint layer but static, uncomposed with a streamed objective; shielded RL has
the veto but no beliefs/LLM/dormancy. Every piece is published; the *composition*
is the moat. Independently confirmed white space: **no shipping memory system does
cite-or-abstain, deterministic lesson-validation, or a declared write-time
constraint contract.**

## 4. LLM-driven vs deterministic-code (the hard rule)

The optimiser/creative component must **never** be its own safety arbiter — the
shielding literature forbids it, and it's the CiteNexus faithfulness-gate principle.
**LLM proposes; a small, verifiable, deterministic layer disposes.**

| Function | LLM | Deterministic code |
|---|---|---|
| Interpret experience as signal/noise vs the objective | ✅ | — |
| Draft/summarise convictions; propose belief updates | ✅ | — |
| Validate a conviction before trust (n≥k, direction, faithful to episodes) | — | ✅ |
| Freshness / "which fact is current" | — | ✅ version/timestamp max (never the LLM) |
| Constraint veto (shield) + `reward≥high ∧ cost>threshold` | — | ✅ separate cost channel, hard compare |
| Justification / assumption tracking (JTMS/ATMS) | — | ✅ symbolic bookkeeping |
| Regime/drift detection (retire stale convictions) | label only | ✅ divergence/distance test triggers |
| Dormancy decay + revival | supplies cue relevance | ✅ activation math + gating |

## 5. Public API (draft — refined during build)

```python
from citenexus.brain import create_brain, Constraint, Objective

brain = create_brain(
    embedder=...,          # any embed(text)->vec (a CiteNexus endpoint, or FakeEmbedding)
    llm=...,               # small model: interpret/reflect/propose (never the arbiter)
    namespace="desk",      # one brain per namespace; many agents may share it
    constraints=[          # the slow layer — the ONLY thing the caller must author
        Constraint.hard("never risk ruin / irrecoverable loss"),
        Constraint.hard("never act on a single unrepeated result (n=1)"),
        Constraint.soft("prefer not to concentrate risk"),
    ],
)

brain.set_objective("catch momentum this week")     # foreground; streamed, replace-on-arrival
ep = brain.record(raw_text, outcome=...)            # store raw experience (verbatim, citeable)
brain.record_outcome(ep.id, reward=..., note=...)   # delayed feedback; reappraisal keeps history

verdict = brain.interpret(raw_text)  # the brain THINKS: signal or noise vs the objective?
                                     # which conviction does it confirm/challenge? does it
                                     # breach a constraint? → grounded judgement or veto/alarm

recall  = brain.ask("what do I believe about shorting into funding spikes?")  # cite-or-abstain
beliefs = brain.convictions(min_confidence=...)     # current point of view, each cited + goal-framed
brain.consolidate()                                 # the "sleep" pass: promote validated signal, decay noise
```

- `interpret(experience)` is the verb that makes it a brain, not a store: goal-and-
  constraint-conditioned judgement, grounded, silent on noise, **loud on
  profitable-but-ruinous**.
- `ask` surfaces **conflict** rather than resolving it when convictions disagree.
- Writes are cheap/synchronous (store the raw episode); the LLM work (interpret,
  reflect, consolidate) is separable and can run async / on a schedule.

## 6. Seed constraints (the bottom layer — confirm / edit)

Domain-general defaults drawn from the design discussion; these become the first
hard shields on day one:

1. **Never risk ruin** — reject any decision with catastrophic/irrecoverable
   downside, regardless of objective score. (hard)
2. **Never act on an unrepeated result** — a single n=1 outcome may be recorded but
   must never be elevated to an acted-on conviction. The experience-following guard.
   (hard)
3. **Never state thin evidence as certain** — abstain instead. Cite-or-abstain for
   beliefs. (hard)

`namespace`-level owners can add domain constraints (e.g. legal/compliance).

## 7. Build plan (staged, test-first, reuses the RAG)

- **B1 — Episodic substrate** *(largely built: `src/citenexus/brain/`)*: `Episode`,
  `Outcome` (general signed feedback + reappraisal history), salience by |RPE| with
  loss-aversion, in-memory store with version stamps, recency×importance×relevance
  recall, cite-or-abstain `ask`. Fix the reappraisal wiring; keep hermetic (fakes).
- **B2 — Goals**: `Objective` (streamed, replace-on-arrival, keeps history/frame) +
  `Constraint` (hard/soft, cost channel + threshold). Signal/noise scored against
  the current objective; convictions carry their goal-frame label.
- **B3 — Constraint shield (killer feature)**: deterministic post-shield; `interpret`
  computes `reward≥high ∧ cost>threshold`, hard-vetoes, soft-penalises, proposes a
  safe fallback. Highest-alarm path. Pure code, no LLM in the veto.
- **B4 — Convictions + validation**: LLM reflects clustered episodes → candidate
  conviction; deterministic gate (n≥k, consistent direction, faithful to episodes,
  not contradicting a retained conviction) before commit. `convictions()`, conflict
  surfacing in `ask`.
- **B5 — Consolidation, dormancy, forgetting**: the "sleep" pass (dual-buffer:
  probation → promote on quality checks); activation decay + revival; retire
  convictions whose goal-frame is gone (dormant, not deleted); regime-change flag.
- **B6 — Examples + eval**: runnable Python and Go examples (hermetic, no network);
  wire to a real small LLM endpoint; prove it beats raw recall with `evaluate`.

## 8. Pitfalls we build against (from the safe-RL literature)

- **Reward hacking / Goodhart** — keep the constraint channel separate and never
  optimised against; treat the specified objective as evidence, not ground truth.
- **Goal misgeneralisation** — brain stays capable but pursues a retired objective;
  mitigate with regime-change detection that retires stale convictions.
- **Constraint gaming** — shield must be deterministic and *outside* the loop.
- **Over-veto / safety paralysis** — pre-shield to prune only the truly unsafe;
  penalise overrides lightly.
- **Catastrophic forgetting** — never delete; decay-and-revive.
- **False regime alarms** — proportional behaviour-divergence detection, not binary
  threshold flips.
