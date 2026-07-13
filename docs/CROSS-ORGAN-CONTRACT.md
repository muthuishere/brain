# Cross-organ contract — how the brain complements the other organs

> The brain is the deemwar CEO's **memory organ**. An organ that only works alone
> is incomplete (owner mandate 2026-07-13). This doc pins the seams by which the
> other organs (citenexus, ctx-optimize, toolnexus, the fleet) read from and write
> into the brain — and the conformance test that guards them.

## The loop every consumer runs

The brain enforces one discipline, drawn from the agent-memory literature
(memory read → reason → act → observe → memory write; Generative Agents, Park 2023):

1. **recall-before-decision** — `brain recall "<cue>"` before choosing. If the
   answer is not `grounded`, there is no prior experience to lean on; proceed with
   eyes open (or gather evidence first).
2. **check-before-action** — `brain check "<decision>" --reward R --signal k=v …`
   before acting. The deterministic shield vetoes ruinous decisions and is
   **fail-closed**: a hard constraint whose signal you did not supply yields
   `undetermined → not allowed → not guaranteed`. Absence of evidence is not
   evidence of safety.
3. **record-after-outcome** — `brain record "<what happened>" --reward R` after the
   result is known, so the next recall is richer.

This loop is pinned by `clis/go/brain/conformance_ceo_test.go`
(`TestConformanceCEODecisionLoop`). If any leg regresses, that test fails.

## How each organ writes in / reads out

| Organ | Writes into the brain | Reads from the brain |
|---|---|---|
| **citenexus** (verification) | Each verdict is recorded as an episode: `brain record "citenexus ABSTAIN: '<claim>' had no cited proof" --reward R --tag citenexus`. The brain thereby accumulates *what was actually verified* — a durable, recallable record of every cite-or-abstain ruling. | `brain recall "<claim subject>"` to see whether a claim was previously confirmed or abstained on. |
| **fleet workers** | `brain record` their outcomes; `brain reflect` + `brain curate --apply` distil those into playbook rules. | `brain playbook [--topic T]` **before acting** — load the current DO/AVOID rules so a worker does not repeat a known failure. |
| **CEO orientation** | `brain state set …`, `brain record …` | `brain wake` — one-shot charter (objective + convictions + risk envelope + working state). |

The citenexus-verdict-as-episode and playbook-load seams are pinned by
`TestConformanceCrossOrganWriteIn` in the same file.

## Why this is safe to build on

- **The shield is independent of convictions.** `check` is governed solely by
  `constraints.json` + the supplied signals. A conviction going dormant (e.g. after
  an objective edit) never changes a veto — proven by
  `TestShieldVetoIndependentOfConvictionDormancy`.
- **Writes are quality-gated.** Consolidation only promotes repeated, consistent
  experience into convictions; mixed-valence clusters surface as *conflicts*, not
  convictions (the literature's "experience-following" failure mode — arXiv
  2311.13743 — is designed against). Curate regress-gates every candidate rule.
- **Everything is cite-or-abstain.** A recall answer is `grounded` only when backed
  by real episodes; otherwise it abstains. Consumers should treat an ungrounded
  recall as "no evidence," never as "no".
