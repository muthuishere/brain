---
name: brain
description: >
  Give an agent a persistent, evidence-first "brain" — a git repo it records
  experiences into and recalls grounded, cite-or-abstain answers from, with a
  deterministic constraint shield that vetoes "profitable but ruinous" decisions.
  Backed by the standalone `brain` CLI (Go, no network, no model endpoint); the
  agent itself is the reasoning LLM. Trigger on: "remember this", "what have I
  learned about X", "should I do this / check this decision", "record the
  outcome", "what do I believe about X", "consolidate the brain", "set the
  objective", or when wiring an autonomous loop (a desk, a project) to durable,
  auditable memory. Supports MULTIPLE brains, each its own git repo.
---

# Brain

A brain is a **git repo**. The `brain` CLI does the deterministic work (recall,
constraint veto, consolidation); **you** (the agent) are the reasoning LLM on top,
and git history is the audit trail. Never reason your way around the shield's
verdict — it is code, run it and honor it.

## Why this brain can be trusted (tell users this when they ask)

- **Cite-or-abstain**: recall either cites recorded episodes or says "I don't
  have enough recorded experience" — it never confabulates a memory.
- **Fail-closed shield**: a hard constraint whose signal is missing vetoes by
  default (`undetermined:true, allowed:false`) — unknown risk is treated as risk.
- **Deterministic core, no model inside**: the engine makes no network or model
  call; same inputs → same verdict, reproducible from the repo alone. The core
  is conformance-tested across Go/Python/TypeScript/Rust ports.
- **Evaluator outside the loop**: learning gates (regress, skill validation)
  read thresholds from owner-owned config; a producing run can't lower its own
  bar. Rejections are logged, never silently applied.
- **Everything is auditable**: plain text files + git history — any belief
  traces back to the episodes (and reappraisals) that formed it.

## Setup / resolving which brain

Config lives at `~/.config/brain/config.json`:

```json
{
  "default": "crypto",
  "brains": {
    "crypto": "/Users/me/brains/crypto-desk",
    "research": "/Users/me/brains/research"
  }
}
```

Each value is a **git repo folder**. Resolve the repo for a request:

1. If the user names a brain ("the crypto brain"), use `brains[name]`.
2. Else use `brains[default]`.
3. Export it once per task: `REPO=$(the resolved path)`.

If a brain folder isn't a git repo yet: `git -C "$REPO" init` then `brain --repo "$REPO" init`.

## The commands (always pass `--repo "$REPO"` and `--json` for machine output)

| Intent | Command |
|---|---|
| **Orient at session start** (charter+risk+live state) | `brain --repo "$REPO" wake --as DESK --json` |
| Write live desk state (working memory) | `brain --repo "$REPO" state set positions.DESK "0 open" --ttl 12h` |
| Heartbeat a loop (so wake knows it's alive) | `brain --repo "$REPO" state heartbeat DESK --ttl 5m` |
| Read state (never serves stale as fresh) | `brain --repo "$REPO" state get KEY --json` · `state list [--stale]` |
| Set the current goal | `brain --repo "$REPO" objective "preserve capital"` |
| Record an experience | `brain --repo "$REPO" record "TEXT" --reward -1 --label loss` |
| Record a plain fact (no outcome) | `brain --repo "$REPO" record "TEXT"` |
| Re-judge a past decision (flip) | `brain --repo "$REPO" reappraise EP_ID --reward -3 --note "why"` |
| Recall (grounded / abstains) | `brain --repo "$REPO" recall "QUERY" --json` |
| What have I learned about X | `brain --repo "$REPO" learn "TOPIC" --json` |
| **Check a decision (the shield)** | `brain --repo "$REPO" check "DECISION" --reward 0.9 --signal ruin_risk=1 --json` |
| The "sleep" pass (distil beliefs) | `brain --repo "$REPO" consolidate --json` |
| Current beliefs | `brain --repo "$REPO" convictions --json` |
| Propose playbook deltas from recent episodes | `brain --repo "$REPO" reflect --json` |
| Gate + merge deltas into the playbook | `brain --repo "$REPO" curate --apply --json` |
| Load the playbook before acting | `brain --repo "$REPO" playbook [--topic T] --json` |
| Inspect one delta against the gate | `brain --repo "$REPO" regress DELTA_ID --json` |
| Full scheduled learning pass | `brain --repo "$REPO" consolidate --evolve --json` |

`reward` is signed feedback along any axis (money is one — `--dimension correctness`
etc. work too). Losses teach more than equal wins; a single unrepeated result never
becomes a belief.

**Orientation layer (state + wake).** Run `wake` at the start of every session / loop
tick to load the charter, the risk envelope, and live STATE in one read — instead of
re-asking the human. Write volatile state as it changes (`state set …`, `heartbeat …`).
Every state value needs a `--ttl` (or `--static`): a value past its TTL reads back
**STALE**, never fresh, and a stale desk heartbeat makes `wake` mark the whole STATE
block "probably DOWN" — so a lapsed writer degrades to a safe "unknown", never a
confabulated "it's running". Stable facts (ports/paths) still live in your CLAUDE.md.

## Commit after every mutation — git history IS the audit trail

After any command that writes (`objective`, `record`, `reappraise`, `consolidate`,
`reflect`, `curate --apply`),
commit so the change is auditable and shareable:

```sh
git -C "$REPO" add -A && git -C "$REPO" commit -q -m "brain: <what changed>"
```

Recall / learn / check / convictions are read-only — no commit.

## The shield (the important part)

`check` returns JSON. **Honor it literally:**

- `"allowed": false` → do **not** take the decision. Offer `"fallback"` instead.
- `"alarm": true` → "profitable but ruinous": it scores well on the objective but
  violates a standing constraint (or a required constraint was undetermined —
  see below). This is the loudest signal — surface it to the human, don't
  quietly proceed.
- `"guaranteed": true` → every constraint self-evaluated in code from a real,
  supplied input. If false, either a cost came from outside the code (advisory)
  or a required signal was missing (see `undetermined` next) — either way, don't
  read the verdict as a proof.
- `"undetermined": true` → **treat exactly like `allowed:false`: do not take the
  decision, use the fallback.** It means something distinct from a measured
  violation, though: at least one **hard** constraint named a signal (via
  `"signal"` in `constraints.json`) that you never passed with `--signal`, so
  the shield could not tell whether it was safe — it is not claiming the
  decision *is* unsafe, only that it **could not check**. `"undetermined_by"`
  lists the constraint names that were unresolved. This is fail-closed by
  design (see `docs/SPEC-shield-signal-provenance-v1.md`): an omitted signal on
  a hard constraint is never silently treated as "cost 0, safe." If you see
  `undetermined:true`, the fix is almost always to re-run `check` with the
  missing `--signal name=value` supplied, not to proceed anyway.

The constraints live in `"$REPO"/constraints.json` (declarative: `name`, `text`,
`kind` hard|soft, `signal`, `threshold`, `weight`, `when_absent`). To evaluate
them, pass the decision's measured signals: `--signal ruin_risk=0.9 --signal
unrepeated=1`. You supply those numbers from the situation; the CLI does the veto.

`when_absent` governs what it means when a named `signal` is missing from
`--signal` at check time:

| `when_absent` | Effect when the signal is omitted |
|---|---|
| `veto` (default for **hard**) | fail closed: `undetermined:true`, `allowed:false` |
| `abstain` | `undetermined:true`, `allowed:false`, but distinct from a fired veto (nothing was measured to violate) |
| `assume_safe` (default for **soft**; opt-in for hard) | treated as cost 0 (not violated), but `guaranteed:false` — an explicit, auditable escape hatch you set in `constraints.json`, in git |

Leave `when_absent` unset for any hard safety constraint you actually want to
gate on — that is the safe default. Only set `assume_safe` for a constraint
that is genuinely optional to evaluate.

## Multiple brains

Each brain is an independent git repo — isolate desks/domains by keeping separate
repos in `config.json`. Never merge two brains' folders; to share across agents,
share the *same* repo (commit/pull), and let conflicting experiences surface as
conflicts rather than averaging them.

## Optional endpoints (embedding / reranker / LLM)

The brain works **fully offline** by default (deterministic hashing recall, no
model calls) — you need nothing. To upgrade recall, drop an `endpoints.json` in
the brain repo (an `endpoints.example.json` is created by `init`):

```json
{
  "embedding": {"base_url": "http://localhost:11434/v1", "model": "bge-m3", "api_key_env": "OLLAMA_API_KEY"},
  "reranker":  {"base_url": "http://localhost:11434", "model": "bge-reranker-v2-m3", "api_key_env": ""},
  "llm":       {"base_url": "http://localhost:11434/v1", "model": "qwen2.5", "api_key_env": ""}
}
```

- **embedding** → semantic recall instead of keyword-ish hashing.
- **reranker** → reorders recalled episodes by relevance.
- **llm** → synthesizes the grounded answer (still faithfulness-gated; still abstains).

Each block is independent — set any subset. `api_key_env` NAMES an env var; the
CLI reads the key from it at run time and never stores or prints it. Env
overrides also work: `BRAIN_EMBED_URL` / `BRAIN_EMBED_MODEL` / `BRAIN_EMBED_KEY_ENV`
(and `BRAIN_RERANK_*`, `BRAIN_LLM_*`). Commit `endpoints.json` only if it holds no
secrets (it shouldn't — keys live in env).

## The self-improving loop (playbook)

The brain turns experience into an **itemized playbook** agents load before acting.
The core is deterministic; you supply the prose when recording.

- **Before acting**: `brain --repo "$REPO" playbook --topic "the task" --json` and
  honor the `AVOID:` / `DO:` rules (each carries evidence episode ids and reward).
- **At work-done / session-end**: record outcomes (successes AND failures,
  honestly), then `reflect` (proposes deltas — read-only) and `curate --apply`
  (regress-gates each delta against validated convictions, merges survivors,
  dedups by content, supersedes stale rules with lineage). Commit after.
- **At retro / on a schedule**: `consolidate --evolve` runs the whole pass
  (consolidate → reflect → gate → curate --apply) in one go.
- Rejections land in `rejected-candidates.ndjson` — logged, never silently
  applied; read them at retro, they are first-class negative results.
- Thresholds live in `"$REPO"/evolve-policy.json` (min_consistency,
  contradiction_overlap…). They are the OWNER'S dial — never edit them in the
  same run that produces deltas.

## Self-evolving skills (the skill library)

`"$REPO"/skills-lib/` is a versioned library of executable procedure specs. The
CLI is only the bookkeeping and the promotion gate — **you** synthesize the
content, run the held-out cases, and write the rationales. The lifecycle:

- **Register from failure**: when `reflect` keeps surfacing the same failure
  pattern, write a skill spec (a markdown procedure: preconditions, steps,
  success criteria) to a temp file and
  `brain --repo "$REPO" skill register --id ID --from FILE --domain D --rationale "why"`.
  A first version becomes current; later versions are CANDIDATES until validated.
  Use `--parent OTHER_ID` when deriving from an existing skill, `--kind composite`
  when chaining primitives.
- **Validate before promoting**: run the SAME held-out cases against the prior
  and candidate versions yourself, write the counts to `DIR/prior.json` and
  `DIR/candidate.json` (`{"passed":N,"failed":M}`), then
  `brain --repo "$REPO" skill validate ID --test-data DIR`. The gate promotes only
  on ≥ +5% improvement (`min_improvement` in evolve-policy.json — the owner's
  dial); rejected versions are archived with their verdict, never silently current.
- **Log every use**: after each invocation,
  `brain --repo "$REPO" skill log ID --task "..." --outcome ok|fail [--cost C]` —
  this feeds the metrics everything else reads.
- **Search before acting**: `brain --repo "$REPO" skill search --domain D
  [--min-success 0.7] --json`, then read the current spec file it points at.
- **At retro**: check `skill metrics ID`; `skill deprecate ID` what degraded
  (lineage kept), `skill rollback ID --to N` when a promotion regressed in the wild.

Commit after register / validate / log / deprecate / rollback, same as any mutation.

## Reading raw memory

The repo is plain files — inspect or grep directly:
`episodes.ndjson` (one experience per line), `convictions.json`, `objective.json`,
`constraints.json`. `git -C "$REPO" log` is the reappraisal history.
