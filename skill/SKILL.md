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
| Set the current goal | `brain --repo "$REPO" objective "preserve capital"` |
| Record an experience | `brain --repo "$REPO" record "TEXT" --reward -1 --label loss` |
| Record a plain fact (no outcome) | `brain --repo "$REPO" record "TEXT"` |
| Re-judge a past decision (flip) | `brain --repo "$REPO" reappraise EP_ID --reward -3 --note "why"` |
| Recall (grounded / abstains) | `brain --repo "$REPO" recall "QUERY" --json` |
| What have I learned about X | `brain --repo "$REPO" learn "TOPIC" --json` |
| **Check a decision (the shield)** | `brain --repo "$REPO" check "DECISION" --reward 0.9 --signal ruin_risk=1 --json` |
| The "sleep" pass (distil beliefs) | `brain --repo "$REPO" consolidate --json` |
| Current beliefs | `brain --repo "$REPO" convictions --json` |

`reward` is signed feedback along any axis (money is one — `--dimension correctness`
etc. work too). Losses teach more than equal wins; a single unrepeated result never
becomes a belief.

## Commit after every mutation — git history IS the audit trail

After any command that writes (`objective`, `record`, `reappraise`, `consolidate`),
commit so the change is auditable and shareable:

```sh
git -C "$REPO" add -A && git -C "$REPO" commit -q -m "brain: <what changed>"
```

Recall / learn / check / convictions are read-only — no commit.

## The shield (the important part)

`check` returns JSON. **Honor it literally:**

- `"allowed": false` → do **not** take the decision. Offer `"fallback"` instead.
- `"alarm": true` → "profitable but ruinous": it scores well on the objective but
  violates a standing constraint. This is the loudest signal — surface it to the
  human, don't quietly proceed.
- `"guaranteed": true` → every constraint self-evaluated in code. If false, a cost
  came from outside and the verdict is advisory.

The constraints live in `"$REPO"/constraints.json` (declarative: `name`, `text`,
`kind` hard|soft, `signal`, `threshold`, `weight`). To evaluate them, pass the
decision's measured signals: `--signal ruin_risk=0.9 --signal unrepeated=1`. You
supply those numbers from the situation; the CLI does the veto.

## Multiple brains

Each brain is an independent git repo — isolate desks/domains by keeping separate
repos in `config.json`. Never merge two brains' folders; to share across agents,
share the *same* repo (commit/pull), and let conflicting experiences surface as
conflicts rather than averaging them.

## Reading raw memory

The repo is plain files — inspect or grep directly:
`episodes.ndjson` (one experience per line), `convictions.json`, `objective.json`,
`constraints.json`. `git -C "$REPO" log` is the reappraisal history.
