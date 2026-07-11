# SPEC: Orientation Layer (state · fact · wake · ritual) — v1

**Status:** open / v1 partially built. **Author:** brain team. **Date:** 2026-07-11.

**Validation changelog (2026-07-11):** two independent adversarial validators + a
blind, clean-context (OpenAI, no CLAUDE.md) re-run of the wake ablation converged:
(1) the **STATE layer is the non-substitutable win** (blind: wake block 6/8 correct,
0 fabrications, and 3/3 on live-state incl. the stale-desk trap; log-dump 0/8 + 1
fabrication; a truly clean no-memory baseline scored 0 — confirming the earlier
baseline was CLAUDE.md leakage); (2) stable **FACTS/CHARTER largely live in a
maintained CLAUDE.md** — the dynamic `fact` layer is lower-value; (3) a no-TTL
state value is **fail-dangerous** (served as current forever after its writer
dies). **Therefore v1 as-built = `state` + `wake` only, with TTL REQUIRED and the
STATE block gated on heartbeat freshness. `fact` and `ritual` are deferred** until
the state layer proves out on a live desk. See scratchpad spike results
(spike1real-ablation, spike4-shield) and the two validator verdicts.

## Why (evidence, not opinion)

Across 7,286 real operator prompts over 5 autonomous-agent projects, the same
four things are re-asked/re-told every session because each session cold-starts
blind. Measured recurrence (share of a desk's *active days* on which a bucket is
raised): FACTS up to **100%**, CHARTER **80–86%**, STATE up to **78%**. The
daily-rhythm demand is one of the strongest signals in the corpus: `restart/fresh`
**364**, `retro` **178**, `daily/everyday` **169**, `morning/pre-open` **140**.

The research (CoALA, ICML 2024) locates the root cause as **model statelessness**
— durable behaviour must come from **external** memory + a read/write contract,
not a bigger context (Lost-in-the-Middle / context-rot refute the "just dump the
log" alternative; a validation spike reproduced this — a 30k-token log dump scored
2.5/7 *with a dangerous stale-as-live hallucination*, while a 600-token curated
wake block scored 7/7). Fine-tuning facts into weights is the wrong lever
(catastrophic forgetting).

CoALA's memory taxonomy maps our buckets 1:1 and is the backbone of this spec:

| Bucket | CoALA type | brain object (this spec) |
|---|---|---|
| volatile STATE (running? P&L? positions?) | **working memory** | `state.ndjson` (NEW) |
| stable FACTS (ports/paths/stack/personas) | **semantic memory** | `facts.json` (NEW) |
| CHARTER / learned beliefs | **semantic via reflection** | `objective` + convictions (EXISTS) |
| RISK envelope | **procedural + deterministic shield** | `constraints.json` + `check` (EXISTS) |
| past decisions/outcomes | **episodic** | `episodes.ndjson` (EXISTS) |

The operating loop CoALA prescribes — *"retrieval reads long-term memory into
working memory; learning writes back"* — is implemented here as **`wake` (read)**
+ **`state set`/`fact set`/`record` (write)**.

## Scope of v1

Add two storage files and four CLI verbs, reusing the existing engine/shield/
consolidate. No changes to episodic/conviction/shield semantics.

New: `state.ndjson`, `facts.json`; verbs `state`, `fact`, `wake`, `ritual`.

---

## 1. `state` — volatile working memory (latest-wins, TTL'd)

Distinct from episodic memory: single-valued, overwritten, **must never be served
stale silently**. Backing file `state.ndjson` (append-only; latest record per key
wins; git log audits how state changed).

Record shape (one JSON object per line):
```json
{"key":"heartbeat.cryptoloop","value":"alive","ts":"2026-07-11T09:00:00Z","ttl_sec":300,"source":"loop"}
```

CLI:
```
brain state set <key> <value> (--ttl DUR | --static) [--source WHO]  # --ttl REQUIRED (5m,900s,2h) unless --static; no silent no-TTL default (fail-safe)
brain state get <key> [--json]                              # -> value + fresh|stale|unknown + age
brain state list [--stale] [--json]                         # all keys; --stale shows only past-TTL
brain heartbeat <name> [--ttl DUR] [--note N]               # sugar for state set heartbeat.<name> alive
```

Semantics (proven by spike): `get`/`list` compute `age = now - ts`; status =
`fresh` if `age <= ttl_sec` (or no ttl), else `stale`; missing key = `unknown`.
A stale or unknown key is **flagged**, never rendered as a confident current value
— this is what stops "is it running?" from being answered with a hallucinated yes.

Standard key conventions (not enforced, documented): `heartbeat.<loop>`,
`pnl.<desk>.{today,cum}`, `positions.<desk>`, `conn.<svc>`,
`runstate.<loop>` (`working|resting-at-gate:X|stuck`), `last_inbound.<channel>`,
`last_ritual_date`.

## 2. `fact` — authoritative semantic memory (single value, supersedes, abstains)

The opposite of an episode: a settled, owner-declared truth that must **never be
re-litigated** and must **abstain** when unknown (not confabulate). Backing file
`facts.json` (a map). Where `recall` says "episodes suggest… or I abstain", a fact
is the decision.

```json
{
  "stack.lang": {"value":"go","why":"single static binary, packageable","supersedes":"python/bun","ts":"..."},
  "svc.messenger.port": {"value":"14310","why":"one global hub, CLI-only"}
}
```

CLI:
```
brain fact set <key> <value> [--why TEXT] [--supersedes KEY_OR_NOTE]
brain fact get <key> [--json]        # -> AUTHORITATIVE{value,why,supersedes} | ABSTAIN
brain fact list [--topic PREFIX] [--json]
```

`get` on an unknown key returns `{"status":"ABSTAIN"}` with a note to ask/record,
never a guess. Facts are **owner-written** (integrity: corpus-poisoning research
shows an unguarded store is steerable; git audit + owner-write is the guard).
Facts are NOT auto-inferred by the agent in v1.

## 3. `wake` — the orientation contract (read LTM → working memory)

One command run at session start / loop tick. Composes existing + new stores into
a fixed, compact block (curated, NOT a log dump — the spike showed dumping is
worse). Per-desk via a profile namespace.

```
brain wake [--as DESK] [--json]
```

Output sections, in fixed order:
- **CHARTER** — `objective` + top convictions (the standing posture/beliefs).
- **FACTS** — `facts.json` (optionally filtered to the desk's topic prefixes).
- **RISK** — a summary of `constraints.json` (the envelope; not a live check).
- **STATE** — `state.ndjson` dump with stale/unknown keys explicitly flagged.

`--as DESK` selects a profile (`profiles/<desk>.json`, optional) that scopes which
fact prefixes / state keys / constraints to surface, and maps a trigger word
(e.g. "stockloop") to the desk. With no profile, `wake` surfaces everything.

Target size: a few hundred tokens. This is the block that scored 7/7.

## 4. `ritual` — the daily rhythm (the "brush your teeth" fundamentals)

Wake is time-aware. Three beats:

| beat | trigger | action |
|---|---|---|
| **light orient** | every call | `wake` (sections above) |
| **daily ritual** | first call of a new day (`today != last_ritual_date`) | the heavy re-grounding + yesterday→today carry (below), then stamp `last_ritual_date` |
| **EOD close** | end of day (operator/cron) | `record` outcomes + `consolidate` → convictions become *tomorrow's* brushed fundamentals |

```
brain ritual [--as DESK] [--force] [--json]   # runs daily beat if not yet run today (or --force)
brain ritual close [--as DESK] [--json]        # EOD: consolidate + snapshot the day
```

Daily-ritual actions (v1, deterministic):
1. **Detect first-call-of-day**: compare `date(now)` to state key
   `last_ritual_date` (per desk). If equal and not `--force`, no-op (return the
   light orient).
2. **Carry yesterday → today**: read yesterday's `pnl.<desk>.today` into
   `pnl.<desk>.cum` (accumulate), then reset `pnl.<desk>.today`; carry
   `positions.<desk>`; surface yesterday's newest convictions.
3. **Reset daily counters**: any state key tagged daily (e.g. a daily loss
   tally used by a `daily_loss_cap` constraint) resets to 0 at the day boundary.
4. **Re-ground the fundamentals**: emit the full CHARTER + RISK (re-read the
   philosophy, not because it changed — because skipping it lets drift set in).
5. **Stamp** `last_ritual_date = date(now)`.

"How it knows what's daily": **frequency = fundamentality.** The ritual's *content*
is what recurs (measured); the *cadence* is a deterministic clock (`last_ritual_date`);
net-new fundamentals arrive only via owner declaration (`fact set`/`objective`) or
by crossing the `consolidate` support threshold — never invented. Learning is
auditable: every conviction carries `supporting_ids` + `support_count`.

---

## Data model (files in a brain repo after v1)
```
objective.json     (exists)   constraints.json  (exists)
episodes.ndjson    (exists)   convictions.json  (exists)
state.ndjson       (NEW — working memory, latest-per-key, TTL'd)
facts.json         (NEW — semantic memory, authoritative, supersedes)
profiles/<desk>.json (NEW, optional — desk namespace + trigger word)
```

## Time / determinism
All new verbs must accept an injectable `now` (not call `time.Now()` deep in the
engine) so state TTL, the ritual day-boundary, and tests are deterministic —
matching how episode timestamps are set today.

## Non-goals (v1)
- No agent auto-inference of facts (owner-written only).
- No pull-adapters (kite/mudrex/ledgers) — v1 is push (`state set` from loops);
  a pull adapter is a follow-up.
- No S3 store; local file store only (as today).
- TTL policy per key-type is left to the operator/profile; the engine only
  enforces "stale if age > ttl".

## Acceptance (mirrors the spikes)
1. `state get` on a past-TTL key returns `stale` with age; never `fresh`. (Spike 2)
2. `fact get` returns the settled value on known keys, `ABSTAIN` on unknown. (Spike 3)
3. `wake --as <desk>` renders CHARTER+FACTS+RISK+STATE with stale flags, compact. (Spike 1)
4. `ritual` runs once per day; second same-day call is a no-op without `--force`;
   carries P&L and resets the daily counter across a simulated day boundary.
5. `ritual close` invokes `consolidate`; a repeated-loss pattern surfaces as a
   conviction at the next `wake`. (Spike 5)
6. Shield unchanged: `check` still vetoes real violations / fails closed. (Spike 4)
7. All new engine logic has tests using an injected clock + temp repo.
```
