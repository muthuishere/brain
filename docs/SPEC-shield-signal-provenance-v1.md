# SPEC — Shield signal provenance: absence of evidence is never evidence of safety (v1)

> Status: proposed · Scope: `libs/go/engine` (shield) + `clis/go/brain` (check/init)
> + skill/docs · Style: deterministic core, zero-network, git-auditable, no new deps.
> One-line: make the shield's `guaranteed` claim *true* by never letting a
> **missing** required signal masquerade as a **satisfied** constraint.

## 1. Problem

The constraint shield is brain's killer feature: a deterministic veto that the
creative component (the agent) cannot reason around, firing loudest on
"profitable-but-ruinous" — high objective reward **and** a violated constraint
(`shield.go:125`). Its whole promise is that the arbiter is *code*, not the model
(BRAIN.md §4).

That promise is currently **hollow at its boundary**. Declarative constraints
read their cost from a named signal the agent supplies at check time
(SKILL.md:84-87 — *"You supply those numbers; the CLI does the veto"*). But the
shield cannot tell a signal that was **provided and is 0 (safe)** from one that
was **never provided (unknown)**:

```go
// libs/go/engine/shield.go:45-53  — Constraint.cost()
if c.Signal != "" {
    return ctx.Signals[c.Signal], true   // BUG: missing key → 0, and "true" (deterministic)
}
```

A Go map miss yields the zero value `0`. So a **hard** `never-ruin` constraint
declared to read `ruin_risk`, checked with that signal simply omitted, evaluates
to cost `0 ≤ threshold` → **not violated**, and `cost()` returns
`deterministic=true`, so `Verdict.Guaranteed()` (`shield.go:80-87`) also returns
**true**. Concretely, today:

```
brain check "bet the account" --reward 0.95 --json         # ruin_risk NOT passed
→ { "allowed": true, "alarm": false, "guaranteed": true, "vetoed_by": [] }
```

The shield reports a **clean, guaranteed pass** on the exact decision it exists to
veto. The failure direction is the worst possible one: silent false-negative on a
hard safety constraint. And it is reachable precisely by the mechanism the shield
was built to defend against — the agent under reward pressure omits (or forgets,
or is nudged past) the very signal that would have vetoed it. `check` with no
signals at all returns `guaranteed:true` for *every* signal-based constraint.

This is higher-leverage than conviction-threshold tuning (that is *calibration*
of a sound mechanism; this is *soundness* of the core mechanism) and than a
cross-language conformance harness (which would only certify the current unsound
semantics identically in four languages). Fix the semantics first; certify after.

## 2. The invariant this must NOT break — and the one it establishes

**Preserve (existing):**

- **INV-DET** The shield never consults a model. Every cost is a Go `CostFn` or a
  named signal read from the decision context. Pure, deterministic, offline.
- **INV-SEP** Constraints remain a *separate cost channel* with hard compare, never
  blended into the objective (BRAIN.md §4, Alshiekh 2018).
- **INV-AUDIT** State stays git-friendly plain text; `constraints.json` stays a
  declarative, human-authored contract.

**Establish (new, INV-SOUND):**

> **A hard constraint may never be reported as *satisfied* unless its cost was
> actually determined from a provided input.** Absence of evidence is not evidence
> of safety. An unknown required signal fails **closed**, never open.

Corollary for `guaranteed`: `Guaranteed()` must mean *"every constraint that
mattered had its cost determined from a real input"* — not merely *"the code path
ran."* Today it conflates the two.

## 3. Mechanism (deterministic; the information is already present)

The fix needs **no new inputs** — the provenance is already knowable and the
current code throws it away by discarding the map comma-ok. We make it first-class.

### 3.1 Provenance of a constraint's cost

Introduce a four-value provenance, computed with zero ambiguity:

| Provenance | Condition | Meaning |
|---|---|---|
| `Computed` | `c.Check != nil` | cost came from a Go cost function |
| `Provided` | `c.Signal != ""` **and** key present in `ctx.Signals` | agent supplied the measurement |
| `Absent`   | `c.Signal != ""` **and** key **missing** | required input was **not** supplied |
| `Unbound`  | no `Check` and no `Signal` | constraint declares no evaluable cost at all |

`Computed` and `Provided` are *determined* (trustworthy cost). `Absent` and
`Unbound` are *undetermined* (cost is unknown, not zero).

`Constraint.cost` changes from `(float64, bool)` to return provenance:

```go
type Provenance int
const ( Computed Provenance = iota; Provided; Absent; Unbound )

func (p Provenance) Determined() bool { return p == Computed || p == Provided }

func (c Constraint) cost(ctx DecisionContext) (float64, Provenance) {
    if c.Check != nil {
        return c.Check(ctx), Computed
    }
    if c.Signal != "" {
        if v, ok := ctx.Signals[c.Signal]; ok {   // <-- read the comma-ok that is discarded today
            return v, Provided
        }
        return 0, Absent
    }
    return 0, Unbound
}
```

### 3.2 Fail-closed policy — declarative, per constraint

A constraint gains an optional `when_absent` policy governing what an **undetermined**
cost means for it:

| `when_absent` | Undetermined hard constraint → | Undetermined soft constraint → |
|---|---|---|
| `veto` (default for **hard**) | `Undetermined` + fail-closed: **not allowed** | n/a |
| `abstain` | `Undetermined`: `allowed:false`, distinct from a fired veto | penalty skipped, flagged non-det |
| `assume_safe` (default for **soft**; opt-in for hard) | cost 0, but `guaranteed:false` | cost 0, `guaranteed:false` |

Defaults make the safe behavior automatic: a hard constraint with a named signal
you forgot to pass **fails closed**. `assume_safe` is the explicit, auditable
escape hatch for a genuinely optional constraint — the author must opt in, in
`constraints.json`, in git.

`Unbound` (a constraint that declares no cost source) is always `assume_safe` in
effect (it cannot fail closed on nothing) but always marks the verdict
`guaranteed:false` — exactly today's behavior, now surfaced instead of hidden.

### 3.3 Verdict surface — a third state

Today a verdict is binary: allowed / vetoed. Add **undetermined** as a distinct,
honest third state so the agent can tell "code cleared it" from "code couldn't
judge it":

```go
type Verdict struct {
    // ...existing...
    Undetermined   bool     // ≥1 hard constraint could not be evaluated (fail-closed)
    UndeterminedBy []string  // their names
}
```

`Evaluate` (`shield.go:96-141`) changes so that, per constraint:

- `Determined()` → existing violated/penalty/veto logic, unchanged.
- **not** `Determined()` and hard and policy `veto`/`abstain` →
  append to `UndeterminedBy`, set `Undetermined=true`, force `Allowed=false`,
  add reason `"undetermined: required signal '<sig>' for hard constraint '<name>' not provided"`,
  and populate `Fallback`.
- **not** `Determined()` and `assume_safe` → cost 0, no veto/penalty, but the
  eval is recorded with its provenance so `Guaranteed()` returns false.

`Allowed` becomes `len(vetoed)==0 && !Undetermined`. The **alarm** condition
(`objectiveReward ≥ highReward ∧ anyViolation`) also fires when the reward is high
and a required constraint is *undetermined* — high-stakes decision, safety input
missing, is itself alarm-worthy.

`Guaranteed()` (`shield.go:80-87`) redefines: `true` iff **every** eval is
`Determined()`. (A verdict can be `Allowed && !Guaranteed` — everything provided
cleared, but something was `assume_safe`/`Unbound`: advisory, as the SKILL already
says.)

### 3.4 Determinism preserved

Every branch above is a pure function of `(constraints, ctx.Signals)`. No model,
no clock, no network. Map presence is deterministic. The output is a pure
extension of today's — same inputs that already pass still produce the same
`allowed/alarm/vetoed_by`; only the previously-silent cases gain an honest verdict.

## 4. Integration anchors (file:line)

| Site | Change |
|---|---|
| `libs/go/engine/shield.go:33-41` (`Constraint`) | add `WhenAbsent string` (`"veto"`/`"abstain"`/`"assume_safe"`, empty = kind default) |
| `libs/go/engine/shield.go:45-53` (`cost`) | return `Provenance` instead of `bool`; **read the comma-ok** |
| `libs/go/engine/shield.go:55-63` (`ConstraintEval`) | replace `Deterministic bool` with `Provenance Provenance` (keep a `Determined()` helper for JSON back-compat) |
| `libs/go/engine/shield.go:66-87` (`Verdict`, `Guaranteed`) | add `Undetermined`, `UndeterminedBy`; redefine `Guaranteed()` over `Provenance.Determined()` |
| `libs/go/engine/shield.go:96-141` (`Evaluate`) | undetermined handling, fail-closed, `Allowed`/`alarm`/`Fallback` updates |
| `libs/go/engine/brain.go:250-253` (`Check`) | passthrough only — no change needed |
| `clis/go/brain/main.go:151-185` (`constraintFile`, `loadConstraints`) | add `WhenAbsent string \`json:"when_absent"\``; map into `engine.Constraint` |
| `clis/go/brain/main.go:193-201` (`cmdInit` starter) | starter `never-ruin`/`never-n1` stay hard; rely on default `veto` (document it) |
| `clis/go/brain/main.go:304-330` (`cmdCheck` + `emit`) | add `"undetermined"`, `"undetermined_by"` to JSON; print an `UNDETERMINED:` line in text mode |
| `skills/brain/SKILL.md:73-94` + mirror `clis/go/brain/skill/SKILL.md` | document the third state + fail-closed + `when_absent` |
| `README.md:63-69` and status table | update the `check` example to show a supplied signal; note fail-closed default |

## 5. Config / API surface + backward compatibility

**`constraints.json` (declarative, additive):**

```json
[
  { "name": "never-ruin", "text": "never risk ruin", "kind": "hard", "signal": "ruin_risk" },
  { "name": "prefer-diversified", "text": "avoid concentration", "kind": "soft",
    "signal": "concentration", "threshold": 0.7, "when_absent": "assume_safe" }
]
```

- `when_absent` is **optional**. Omitted → `veto` for hard, `assume_safe` for soft.
- **Old files keep parsing** (new field ignored if absent) — but their *runtime
  semantics change for hard signal-constraints*: a hard constraint checked without
  its signal now fails closed instead of silently passing. That is the point of the
  change, and it is the safe direction. It is called out in the release notes.

**Migration / escape hatch:**

- To restore legacy "assume safe when absent" for a specific constraint, set
  `"when_absent": "assume_safe"` — explicit, auditable, in git.
- Env `BRAIN_SHIELD_LEGACY_ABSENT=1` (one release only, documented as deprecated)
  makes `Absent` behave as `assume_safe` globally, for teams that need a staged
  rollout. Default off. Removed in the next minor.

**JSON output (`check --json`) — additive keys, existing keys unchanged:**

```json
{ "allowed": false, "alarm": true, "undetermined": true,
  "undetermined_by": ["never-ruin"], "vetoed_by": [], "guaranteed": false,
  "fallback": "...", "reasons": ["undetermined: required signal 'ruin_risk' ..."] }
```

Agents already honor `allowed:false` (SKILL.md:76). `undetermined`/`undetermined_by`
are strictly additive — an agent that ignores them still fails safe because
`allowed` is already false.

## 6. Tests / conformance

The engine currently has **no tests** (only ingest/storage/modelclients/endpoints
do). This change adds the first `libs/go/engine/shield_test.go`, table-driven and
hermetic:

1. **Present-and-safe** — hard `never-ruin`, `ruin_risk=0` → `allowed:true,
   guaranteed:true, undetermined:false`. (Distinguishes 0 from missing.)
2. **Present-and-violating** — `ruin_risk=1` → `allowed:false, vetoed_by:[never-ruin]`.
3. **Absent required, default veto** — signal omitted → `allowed:false,
   undetermined:true, undetermined_by:[never-ruin], guaranteed:false`. *(the bug's
   regression test)*
4. **Absent + high reward** — reward ≥ high, signal omitted → `alarm:true`.
5. **Absent + assume_safe** — `when_absent:assume_safe`, omitted → `allowed:true,
   guaranteed:false, undetermined:false`.
6. **Soft absent** — default `assume_safe`, no penalty, `guaranteed:false`.
7. **Unbound** — constraint with no `Check`/`Signal` → `allowed:true,
   guaranteed:false`. (Codifies today's behavior, now explicit.)
8. **Computed (Check fn)** — cost from a `CostFn` → `Computed`, `guaranteed:true`.
9. **Determinism** — same inputs → byte-identical `Verdict` across repeated runs
   and across map insertion orders (guard against map-iteration nondeterminism in
   `UndeterminedBy`/`VetoedBy`: assert they are **sorted**).
10. **Back-compat parse** — a `constraints.json` with no `when_absent` yields the
    documented kind defaults.

CLI smoke test in `clis/go/brain`: `check` without a signal exits with the
undetermined verdict and `allowed:false` in `--json`.

**Cross-language note (⏳, out of scope for v1):** the provenance tri-state should
enter the shared SPEC-PORTS conformance contract (golden decision vectors:
`{constraints, signals} → verdict`) so Python/TS/Rust ports adopt fail-closed
identically. Filed as a follow-on; v1 ships the Go semantics + Go tests that the
golden vectors will later be generated from.

## 7. Foundation-first rung plan

- **Rung 0 — provenance plumbing (⏳ this spec).** `cost()` returns `Provenance`;
  `ConstraintEval.Provenance`; read the comma-ok. Pure refactor, no behavior change
  yet if `Absent` is temporarily mapped to old behavior. Lands with tests 1,2,7,8.
  *Foundation: makes the missing information observable.*
- **Rung 1 — fail-closed default (⏳).** `Undetermined` verdict state; hard-absent →
  `allowed:false`; redefine `Guaranteed()`; alarm on high-reward-undetermined.
  Lands regression tests 3,4,9. *This is the soundness fix — INV-SOUND holds.*
- **Rung 2 — `when_absent` policy + CLI/skill surface (⏳).** Declarative field,
  `assume_safe` escape hatch, JSON keys, SKILL/README docs, legacy env flag. Tests
  5,6,10. *Makes the new default configurable and documented.*
- **Rung 3 — conformance golden vectors (⏳, follow-on repo `citenexus`).** Encode
  the tri-state as shared `SPEC-PORTS` decision vectors; port Python/TS/Rust.
  *Certifies the now-sound semantics across languages.*
- **Non-goals (v1):** no change to recall/consolidation/convictions; no new network
  path; no S3/ingest wiring; no per-signal freshness/staleness (a later idea:
  treat a *stale* provided signal as `Absent` — explicitly deferred).

## 8. Decision & rationale (in lieu of an ADR)

The repo has no `docs/adr/` convention today, so this spec carries the decision
record rather than inventing an ADR numbering scheme the project doesn't use.

**Decision:** distinguish *unknown* from *safe* in the shield, and fail hard
constraints **closed** on unknown. **Why this over alternatives:** (a) *tune
conviction thresholds* — calibration of a sound mechanism, not soundness of the
core one; lower leverage. (b) *cross-language conformance harness* — valuable, but
certifying the current semantics would freeze the false-negative into four
languages; it becomes rung 3 *after* the fix. (c) *require the agent to always
pass all signals* — pushes the guarantee back onto the creative component the
shield exists to distrust; violates BRAIN.md §4. The chosen fix keeps the arbiter
in deterministic code, adds zero dependencies and zero network, is fully
declarative and git-auditable, and repairs the one guarantee the whole product is
sold on: that the veto is real.
