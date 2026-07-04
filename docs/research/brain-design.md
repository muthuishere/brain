# CiteNexus Brain — design (research-backed)

> Second research pass: 108 agents, 25 sources, 105 claims → 23 verified.
> Companion to `memory-brain-research.md` (biology + first pass). This file is
> the **design decision** for the `brain` capability.

## The one-line shape you asked for

```python
from citenexus import create_brain

brain = create_brain(
    cite_client=rag,                 # an existing CiteNexus (storage + vector+BM25+RRF + faithfulness gate)
    llm=small_llm_endpoint,          # any OpenAI-compatible small model (extraction + reflection)
    namespace="crypto-desk",         # PartitionPath scope — one brain per desk, shared across its agents
)

brain.record(raw_text)               # feed ANY raw experience; brain extracts + stores + reconciles
answer = brain.ask("what have I learned about shorting into funding spikes?")
```

`record` and `ask` do **everything internally**. No CLI, no schema for the caller
to fill in. That is the whole public surface (plus `consolidate()` and
`forget()`, below, which most callers never touch — a scheduler does).

## What the research settled

The biology and the production AI systems **converge on the same pipeline**, which
is why we can build it confidently:

1. **Two stores, not one (Complementary Learning Systems).** Brains keep a *fast
   episodic* store (hippocampus) and a *slow semantic* store (neocortex).
   Verified, top-tier primary sources, unanimous. → We keep **raw episodes** AND
   **distilled lessons** as two tiers, not one blob.

2. **Consolidation is an active offline job, not passive decay.** Sleep *replays*
   overlapping episodes and distills them into schemas, strengthening shared
   structure and **forgetting the weakly-encoded**. → We need a scheduled
   `consolidate()` pass (the "sleep" job), and **forgetting is first-class**.

3. **The canonical write path is four ops: ADD / UPDATE / DELETE / NOOP** (Mem0,
   arXiv:2504.19413, verified 3-0). Raw text → small-LLM extracts candidate facts
   → semantic dedup against what's stored → emit exactly one op. Superseded facts
   are **soft-deleted (marked obsolete), never physically removed** — which is
   exactly the evidence-first / audit stance CiteNexus already takes.

4. **Do NOT let the LLM decide which fact is current.** The single highest-value
   finding (arXiv:2606.01435, verified 3-0): move freshness/contradiction
   resolution **out of the LLM into deterministic Python**. The LLM only
   *extracts candidates*; a version/timestamp `max()` picks the winner. Worth
   **+10.8pp average, up to +21pp** at long context. → Every memory is
   version-stamped; conflicts resolve in code, not by asking the model "which is
   true now?"

5. **Retrieval is multi-signal hybrid fusion** — semantic + keyword + entity in
   parallel, fused (Mem0, verified 3-0). This is **1:1 our existing
   vector + BM25 + structure retrievers with RRF (k=60)**. We add Generative-Agents
   **recency + importance + relevance** scoring on top (recency decay × salience).

6. **It beats stuffing everything into context — on accuracy *and* cost.**
   Memory-augmented retrieval reports large wins over full-context (~3–4× cheaper,
   ~91% lower p95 latency, big accuracy lift). Treat the *magnitudes* as real; the
   exact percentages are vendor-reported on a contested benchmark (Zep publicly
   disputes Mem0's numbers — one claim was refuted this pass). The direction is
   not in doubt.

## What "ask like a brain does" means concretely

You said `ask()` should behave *like a brain*, not like a database SELECT. A brain
does **reconstructive recall**: it retrieves the relevant traces and *synthesizes*
a coherent answer, coloured by salience and recency — it does not replay rows.
So `ask()`:

- retrieves via vector + BM25 + structure → RRF → rerank,
- re-scores by **recency × importance × relevance** (salient wins/losses surface
  first, like emotional weighting in biology),
- **synthesizes** an answer with the small LLM,
- but passes it through the **faithfulness / cite-or-abstain gate** so every
  claim is grounded in an actual episode — and **abstains** when memory is too
  thin ("I don't have enough experience to say"). That is the honest analogue of
  a brain admitting it doesn't remember, and it's the CiteNexus guarantee.

Return value: a `Recall` object — the synthesized answer **plus** the episodes it
drew on (provenance), so an agent (or you) can audit *why* the brain said it.
This satisfies both "give me the brain's answer" and CiteNexus's no-ungrounded-claim rule.

## Design decisions (the calls the research lets us make)

| Question | Decision | Why |
|---|---|---|
| Store raw or distilled? | **Both** — episodes (raw, verbatim, citeable) + lessons (distilled) | CLS two-store; audit needs the raw |
| Write sync or async? | **`record()` stores the raw episode synchronously (fast, never lost); extraction+reconciliation runs in the same call by default, `record_async()` defers it** | biology encodes immediately, consolidates later; keeps writes cheap |
| Who resolves contradictions? | **Deterministic Python over version stamps — never the LLM** | +10.8–21pp (arXiv:2606.01435) |
| Extraction model? | **The injected small model** (candidate facts + reflection only) | cost; the hard adjudication is deterministic |
| Consolidation? | **Scheduled `consolidate()` "sleep" pass** — replay overlapping episodes → distill lessons via `LLMGraphDistiller` → decay/prune low-salience | active systems consolidation |
| Forgetting? | **First-class `forget()` policy** — decay unretrieved low-importance, protect salient, soft-delete superseded (keep audit row) | "Remembering to Forget"; evidence-first keeps the audit trail |
| Multi-agent sharing? | **One brain per `namespace` (PartitionPath); many agents read/write the same brain; hard isolation between namespaces** | 36.9% of multi-agent failures are shared-state corruption — isolate |
| Salience / wins vs losses? | **`record(..., outcome=)` sets importance = \|reward-prediction-error\|; losses weighted to teach more; credit the *cue* not just the result** | dopamine RPE, asymmetric, TD credit assignment |

## How it reuses what we already shipped

Nothing here is a rewrite — ~70% is existing CiteNexus primitives:

- **Episode = Evidence Unit** + provenance (already have it, already citeable).
- **Retrieval** = vector + BM25 + structure + RRF fusion + rerank (shipped, and
  already ported to Go/TS).
- **Lessons** = `LLMGraphDistiller` / wiki distiller (shipped).
- **`ask()` grounding** = the faithfulness / cite-or-abstain gate (shipped, the
  core guarantee).
- **Namespacing** = `PartitionPath` (shipped).
- **New code** is the thin outcome/reflection layer: the ADD/UPDATE/DELETE/NOOP
  write reconciler, version-stamped deterministic freshness, the recency×importance
  scoring, and the `consolidate()` / `forget()` scheduler.

## What the first pass missed (flagged by this pass)

1. **Deterministic freshness** — don't ask the LLM to track which fact is current.
   This wasn't in the first design; it's now decision #4 and the biggest single lever.
2. **The four-op write protocol** (ADD/UPDATE/DELETE/NOOP) as the concrete write
   contract — first pass had "quality-gated write" but not the op vocabulary.
3. **Benchmark honesty** — LOCOMO/LongMemEval are the standard evals but the
   published numbers are contested (Zep vs Mem0). If we claim performance, we run
   our *own* eval via `evaluate(csv)`, not cite vendor numbers.
4. **Bitemporal audit rows** (TOKI, arXiv:2606.06240, medium confidence) — model
   contradiction resolution as write-time concurrency control keeping the losing
   fact in an audit row. Fits evidence-first; adopt the shape, hold the theory loosely.

## Competitive teardown (2nd research pass — how rivals do the 4 hard capabilities)

Verified findings (3-0 unless noted). The pass was cut short by a session limit
(62/110 agents), so this is the confirmed subset, not exhaustive.

- **Eywa** (arXiv:2605.30771) — *the closest prior to us, and it validates the
  approach.* Deterministic **hard-anchor validation** of LLM-extracted facts
  (dates, money, quoted strings, IDs, URLs, %) against source before commit;
  **immutable evidence store + revisable beliefs** each with a provenance link;
  "**evidence before belief**" (raw evidence never rewritten; beliefs can be
  accepted/repaired/superseded/rejected); contradiction via **lifecycle metadata**
  keeping superseded facts retrievable. → We are NOT first on "LLM + deterministic
  validation + evidence-first." **Our edge must be narrower and real:** we validate
  *distilled lessons* (outcome-conditioned experiential claims), not just extracted
  facts; and we weight by *salience* (win/loss RPE). Eywa does neither.
- **A-Mem** (arXiv:2502.12110) — LLM-only note generation *and* LLM-only "memory
  evolution" that mutates old memories on write. **No validation, trusted blind,
  memories are mutable.** Confirms the white space.
- **Sleep-time compute** (Letta, arXiv:2504.13171) — precomputing offline by
  anticipating queries cuts test-time compute **~5×** for equal accuracy.
  Empirical justification for the consolidation job paying for itself.
- **Dual-buffer consolidation** (survey arXiv:2603.07670, 2-0) — field "oscillates
  between hoarding and amnesia"; fix = a **hot buffer on probation → promoted to
  long-term only after quality checks (re-verification, dedup, importance)**. This
  is our LLM-generates/code-validates gate, named. **Adopted as our consolidation design.**
- **Survey** (arXiv:2512.13564) — names **trustworthiness + memory automation as
  open frontiers.** White space confirmed at field level.

### Where we win (the defensible intersection)

No system found does all of: **(1) cite-or-abstain grounded recall** (everyone
else synthesizes confidently), **(2) deterministic validation of distilled
*lessons*** (Eywa validates facts, not lessons; A-Mem/Mem0 validate nothing),
**(3) outcome/salience weighting for wins vs losses**, **(4) provenance-preserving
soft-delete audit trail**. That four-way intersection is our moat.

### Stage 2, re-specified with the LLM+deterministic split (buildable NOW)

The hybrid dissolves the "unproven" risk — every LLM output passes a deterministic
gate before it's trusted, exactly like the existing faithfulness gate on the answer path:

| Capability | LLM does | Deterministic code does (the gate) |
|---|---|---|
| Distilled lessons | proposes a candidate lesson from clustered episodes | requires n≥threshold supporting episodes, checks lesson is faithful to them (token/entity overlap), rejects if it contradicts a retained lesson |
| Consolidation ("sleep", dual-buffer) | proposes merges/schemas from the hot/probation buffer | promotes to trusted tier only if it preserves provenance, dedups, and passes importance — else stays on probation |
| Forgetting | (nothing) | decay × importance score; soft-delete to audit row; protect salient/high-value |
| Concurrency (within-desk) | (nothing) | version/timestamp stamps; write-time max-serial resolution; per-namespace write serialization |

## Open questions to pin before/while building

- Consolidation **trigger + budget**: event-count, time, or idle-triggered? How
  aggressively can `forget()` prune without breaking auditability?
- Concrete parameters for recency-decay half-life and small-LLM importance elicitation.
- How deterministic-freshness interacts with the faithfulness gate on fuzzy,
  non-serial contradictions (evolving preferences, partial overlaps).
