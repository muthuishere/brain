# SPEC-shield-conformance-v1

## Motivation

Work item asked to "polyglot-parity the shield fail-closed fix... to the
py/ts/rust ports with shared conformance fixtures." Auditing `rag-cite-nexus`
(the polyglot home of the sibling CiteNexus project, which brain's engine
already depends on for `gate`/`chunker`/`fakes`) turned up **no existing
brain/shield module in Go, Python, TypeScript, or Rust there** — not in
`golang/`, `python/src/citenexus/`, `js/src/`, or `rust/src/`. Brain's own
`INPROGRESS.md` claims the engine is "a vendored copy of `golang/brain`" and
`shield.go`'s header comment says it "mirrors `src/citenexus/brain/shield.py`"
— neither exists. `SPEC-shield-signal-provenance-v1.md` itself already
flagged this honestly: "the provenance tri-state should enter the shared
SPEC-PORTS conformance contract... **filed as a follow-on**; v1 ships the Go
semantics + Go tests that the golden vectors will *later* be generated from."

There is nothing to "port the fix to" — no py/ts/rust shield exists to drift.
Building three new language runtimes from scratch, this cycle, on a mission
whose premise assumed they already existed, is a much bigger and more
hard-to-reverse commitment than the ticket implies. **This spec ships the
part that is unambiguously right regardless of that decision**: a
language-neutral, versioned set of golden decision vectors — so that the
moment any port *does* get built (Go, Python, TS, Rust, or otherwise), it has
something concrete and mechanical to conform to, and "the bet the account
regression breaks any drifting port" becomes a literal, runnable test rather
than an aspiration. Whether/when to actually build additional-language
runtimes is a separate, larger decision (see the accompanying CYCLE-HUDDLE).

## Goals

1. A stable, versioned JSON fixture format for a shield decision: inputs
   (declarative constraints + a decision's signals + objective reward) →
   expected `Verdict` fields. Declarative-only (no `Check` funcs — those
   aren't serializable, and the whole point of `constraints.json`'s
   `signal`-based constraints is that they *are* portable).
2. A canonical fixture file, `conformance/cases/shield.json`, covering the
   fail-closed fix's full behavior space: present-and-safe,
   present-and-violating, the **bet-the-account regression** (the exact
   pre-fix bug), the `assume_safe` opt-in, soft-constraint defaults, Unbound,
   penalty math, mixed hard+soft, and the alarm boundary.
3. A Go conformance-runner test (`libs/go/engine/conformance_test.go`) that
   loads the fixture and asserts the *existing* engine matches every vector
   exactly — proving the fixtures are correct and byte-accurate to the
   reference implementation, and giving a template any future port's own
   runner can copy.

## Non-goals (this SPEC, as originally written)

- ~~**No Python/TypeScript/Rust shield implementation.**~~ **Superseded** —
  see Rollout below: the CEO huddle resolved to build them, and they're now
  landed in this same repo (`libs/python`, `libs/js`, `libs/rust`). Left here
  for history: at the time this SPEC was first written, that was correctly
  identified as a real, separate, materially larger commitment not to be
  guessed on solo — which is exactly why it was raised as a CYCLE-HUDDLE
  instead of built silently.
- **No change to `libs/go/engine/shield.go`'s behavior.** This SPEC only adds
  a fixture format and tests that read the engine (Go and, now, the ports),
  never writes to its behavior.
- **No new conformance infrastructure/runner framework.** One JSON file, one
  Go test. If/when other-language ports exist, each gets its own thin runner
  reading the same JSON — no shared code is presumed across languages beyond
  the file format itself (matching `rag-cite-nexus`'s own `conformance/cases/`
  precedent: plain JSON, one runner per language).

## Fixture format

`conformance/cases/shield.json` is a JSON array. Each element:

```json
{
  "name": "bet-the-account-regression",
  "constraints": [
    {"name": "never-ruin", "kind": "hard", "signal": "ruin_risk", "threshold": 0.5, "weight": 1},
    {"name": "never-n1", "kind": "hard", "signal": "unrepeated", "threshold": 0.5, "weight": 1}
  ],
  "signals": {},
  "objective_reward": 0.95,
  "high_reward": 0.5,
  "fallback": "safe-fallback",
  "expect": {
    "allowed": false,
    "alarm": true,
    "undetermined": true,
    "undetermined_by": ["never-n1", "never-ruin"],
    "vetoed_by": [],
    "penalized_by": [],
    "guaranteed": false
  }
}
```

- `constraints[].kind` is `"hard"` or `"soft"`. `when_absent` is optional
  (`"veto"` | `"abstain"` | `"assume_safe"`), matching `constraints.json`;
  omitted means the kind default (veto for hard, assume_safe for soft).
- Omitting `signal` entirely means the constraint is **Unbound** (declares no
  cost source) — the fixture format has no way to express a `Check`-sourced
  constraint (Computed provenance) since a Go closure isn't serializable;
  Unbound stands in for it in cross-language fixtures — a constraint any port
  can't evaluate at all behaves identically to one whose Go-only `Check` a
  port doesn't have.
- `signals` is the decision's supplied measurements (a missing key ==
  genuinely omitted, distinct from a present key with value 0 — the entire
  point of the fix).
- `expect.adjusted_reward` is optional; present only for cases where a soft
  penalty changes it (omitted means "equal to `objective_reward`", the
  common case).
- Every list field (`vetoed_by`, `penalized_by`, `undetermined_by`) is
  **sorted** — the engine sorts them; a conformant port must too.

## Rollout

1. ~~Land `conformance/cases/shield.json` + the Go runner (this SPEC).~~ Done.
2. ~~CYCLE-HUDDLE: does the CEO want a real Python/TS/Rust shield built
   now...~~ Resolved: yes, build now. Repo-home: inside the `brain` repo
   itself, mirroring the existing `libs/go/` convention (`libs/python/`,
   `libs/js/`, `libs/rust/`) — not `rag-cite-nexus`, and without presupposing
   #35559's still-open standalone-extraction-repo decision.
3. **Landed**: `libs/python/brain_shield`, `libs/js/brain-shield`,
   `libs/rust/brain-shield` — each a line-for-line port of `shield.go`'s
   `Provenance`/`WhenAbsent`/fail-closed logic, each independently verified
   against every vector in `conformance/cases/shield.json` (including
   `bet-the-account-regression`), each wired into CI
   (`.github/workflows/ci.yml`: `shield-conformance-{python,ts,rust}` jobs)
   so a drifting port fails its own build. Only the shield ported — not the
   full engine (recall/consolidation/convictions stay Go-only; there is no
   cross-language episodic memory today).
4. Follow-on, not done here: wiring these into receipts-V3's actual
   re-execution path (this SPEC proves parity exists; using it from
   receipts-V3 is separate integration work).
