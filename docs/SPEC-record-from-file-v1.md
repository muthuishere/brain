# SPEC-record-from-file-v1

## Motivation

Two huddle plans (independent Plan/critic agents) converged on the same
finding: `libs/go/engine` — the shield/recall/conviction/consolidation core —
ships with **zero test files**, despite `README.md` calling it the
safety-critical, deterministic heart of the brain. That's the harden target.

For the "highest-leverage improvement" the plans diverged. Plan A proposed
wiring the already-built, already-tested `libs/go/ingest` package into the CLI
(`record --from-file`). Plan B proposed `objective --history` +
`brain forget`/`forgetStale` CLI surfacing. Both are safe; only ingest-wiring
closes a gap `README.md`'s own status table calls out by name:

> `libs/go/ingest` ... ⚠️ Library-only. `Fetch`/`Crawl`/`ChunkFile` are
> implemented and tested, but no CLI command wires them into `record` yet.

The mission's own instruction ("audit ⏳ stubs vs README ... THEN implement
the improvement") points at this gap specifically. Going with ingest-wiring.

## Goals

1. Harden `libs/go/engine` with real test coverage before touching anything
   else — shield veto/penalty/alarm, conviction confidence, consolidation
   clustering/conflict, recall gate/abstention/conflict, objective
   dormancy/revival, file-store round-trip.
2. Ship `brain record --from-file PATH`: chunk a local file
   (`ingest.ChunkFile`) into N episodes and record each one, deterministically.
3. Update `README.md`'s status table to flip `libs/go/ingest` (file path) from
   ⚠️ to ✅, and keep the CLI help/examples in sync.

## Non-goals (v1)

- **No `record --from-url`.** Wiring `ingest.Fetch`/`Crawl` into the CLI
  means the CLI would reach the network by default the moment a user tries
  ingest — that's a bigger default-posture change than "wire an existing
  library call," and the endpoints precedent (`endpoints.json`, explicit
  opt-in file) suggests any network-reaching CLI surface needs its own
  opt-in design. Deferred; flag for a follow-up SPEC if wanted.
- **No dedup/upsert-by-content.** Re-ingesting the same file (or overlapping
  chunk windows) creates additional episodes; `Store` has no content-addressed
  upsert. This matches the existing append-only, supersede-don't-delete
  philosophy (`Episode.SupersededBy`), so it's treated as by-design rather
  than a bug. Documented here so it isn't rediscovered as a "surprise."
- **No per-chunk reward inference.** `--reward`/`--label`/`--dimension` (if
  passed) apply uniformly to every chunk recorded from one file. This is a
  modeling shortcut, not a claim that every chunk of a document deserves the
  same outcome. Fine for v1: an agent that wants per-chunk rewards can still
  `record` chunks individually, or `reappraise` afterward.
- **No `--store s3` / new `engine.Store` backend.** Out of scope for this
  cycle; also flagged by Plan B as a bigger, harder-to-reverse change (breaks
  git-native diffability, needs a concurrency/locking story) that deserves its
  own huddle if ever pursued.
- **The shield/veto logic itself is not touched.** Hardening means tests only.
  Any test that seems to "want" different `Evaluate`/`Guaranteed` behavior is
  a finding to report, not a license to change the guarantee.

## Engine test plan (Goal 1)

New files under `libs/go/engine/`, `package engine` (white-box, matching the
existing single-package layout):

- **`shield_test.go`** — table-driven over `Constraint{Kind, Threshold,
  Weight, Check|Signal}`:
  - hard constraint violated → `Allowed=false`, `VetoedBy` contains its name,
    `Fallback` set to the caller's fallback string — *regardless* of
    `objectiveReward` (a hard veto is not reward-sensitive).
  - soft constraint violated → `Allowed=true`, `PenalizedBy` contains its
    name, `AdjustedReward < ObjectiveReward` by `Weight*(cost-threshold)`.
  - alarm is the conjunction: 2×2 over {reward high/low} × {violation
    yes/no} — `Alarm=true` in exactly the high-reward+violation cell.
  - `Guaranteed()` is `false` the moment any evaluated constraint's cost came
    from `Signal` rather than `Check`; `true` when all evaluated constraints
    are `Check`-sourced (or there are none).
  - mixed hard+soft in one `Evaluate` call — veto wins for `Allowed`, but
    penalty still accrues into `AdjustedReward`, and both names show up in
    their respective lists.

- **`conviction_test.go`**:
  - `ConfidenceOf(0, c)==0`, `ConfidenceOf(1, c)==0` for any `c` — the
    never-act-on-n=1 guard.
  - `ConfidenceOf(n, consistency)` increases monotonically in both `n>=2` and
    `consistency`.
  - `ConvictionID` is stable for the same `(namespace, statement,
    supportingIDs)` regardless of input slice order (sorted internally), and
    differs when any input differs.

- **`consolidate_test.go`**:
  - `ClusterByTokens`: a chain of episodes pairwise sharing >= `minShared`
    tokens merges into one cluster via union-find; a disjoint pair stays two
    clusters; episodes with no `Outcome` or inactive (`SupersededBy != ""`)
    are excluded entirely.
  - `AssessCluster`: `IsConviction` requires both `SupportCount>=minSupport`
    and `Consistency>=minConsistency`; a 3-vs-2 valence split under a
    `minConsistency` that neither side clears yields `Conflicted=true`,
    `IsConviction=false` — surfacing disagreement instead of picking a side.

- **`brain_test.go`** (using `fakes.FakeEmbedding{}` / `fakes.FakeLLM{}` +
  `NewInMemoryStore()`, the project's existing offline-test pattern):
  - `Ask` abstains (`Grounded=false`, a `Reason`) on an empty store and on a
    populated store with no relevance overlap.
  - `Ask` returns a grounded answer when an episode overlaps and (with an LLM)
    the draft is faithfully supported (`gate.IsSupported`); abstains instead
    of hallucinating when the draft is not.
  - `valenceConflict`: co-recalled episodes with opposite valence set
    `Recall.Conflict=true` without suppressing the grounded answer.
  - `Reappraise` appends the old `Outcome` to `PriorOutcomes` and swaps in the
    new one — nothing is overwritten/lost.
  - `SetObjective` dormancy round-trip: switching A→B dormants only
    A-framed convictions; switching back to A revives exactly those, leaving
    B-framed convictions dormant.
  - `Consolidate` end-to-end: repeated consistent episodes → a conviction is
    formed and counted in `ConsolidationReport`; `forgetStale` decays only
    episodes that are unsupported, past the age threshold, and
    non-positive-salience.
  - `FileStore`/`InMemoryStore` round-trip: episodes/convictions/objective
    written then reloaded (`NewFileStore` on the same dir) preserve version
    counters and content — the file-store persistence path currently has
    zero coverage either.

CI (`go vet && go build && go test ./...`) must stay green throughout; these
are additive test files, no production-code behavior changes.

## `record --from-file` surface (Goal 2)

- New flag on `record`: `--from-file PATH`. When present, the positional TEXT
  argument is not required (mutually exclusive — error if both given).
- Optional `--max-tokens N` (default 450) / `--overlap N` (default 60),
  matching `chunker.ChunkText`'s pinned defaults.
- `--reward`/`--label`/`--dimension`, if passed, apply identically to every
  chunk's `Outcome` (see non-goals).
- Implementation: `ingest.ChunkFile(path, maxTokens, overlap)` →
  for each `ingest.Chunk`, call `b.Record(chunk.Text, outcome)` in file order;
  print one `recorded ep_...` line per episode plus a `recorded N episodes
  from PATH` summary line (JSON mode: an array of episode IDs).
- Deterministic throughout: chunking is a pure function of file bytes, and
  episode IDs are `sha1(namespace, version, text)` — no model call in this
  path. Zero-network: reading a local file never touches the network.
- Test coverage: an integration-style test in `clis/go/brain` (mirroring the
  existing `endpoints_test.go` hermetic style) that inits a temp repo, writes
  a temp multi-paragraph file, runs the `record --from-file` path, and asserts
  N episodes land in `episodes.ndjson` with matching text/order.

## Rollout

1. Land engine tests, confirm CI green.
2. Land `record --from-file` + its test, confirm CI green.
3. Flip the README status-table row for `libs/go/ingest` (file path only —
   web/crawl stays ⚠️ per non-goals) and refresh the "Quick use" example.
4. Open PR; report `PR-OPEN` to the work queue.
5. Dogfood: point at least one real fleet brain's onboarding doc / usage at
   `record --from-file` for bulk-ingesting a doc into episodic memory.

## Open questions (not blocking this cycle)

- Should `--from-url` get its own opt-in design (a `--allow-network` flag, or
  requiring `endpoints.json`-style config) in a future SPEC?
- Is unbounded append-only duplication on re-ingest actually fine long-term,
  or does `Consolidate`/salience math need a content-addressed dedup pass at
  scale? Flag for a future huddle if it becomes a real problem.
