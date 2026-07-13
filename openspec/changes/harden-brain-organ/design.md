## Context

The brain is a Go CLI (`clis/go/brain`) over an engine (`libs/go/engine`), no network, no model endpoint — the agent is the reasoning LLM. Baseline `go test ./...` is green (5 packages) and the full command surface was exercised against a throwaway copy of the live CEO brain (`~/.config/deemwar-one-os/ceo-brain`). The shield is already correctly fail-closed. Three defects and one missing contract remain (see proposal). This design keeps the deterministic core intact and makes the smallest changes that fix real CEO-facing behavior.

Research anchors (already vendored in `docs/research/memory-brain-research.md`, 24 sources): the episodic→semantic split (CLS; McClelland/O'Reilly), consolidation-as-sleep, and Generative Agents' retrieval (Park 2023, arXiv 2304.03442). The new anchor is **persistence of reflections**: Reflexion (Shinn 2023, arXiv 2303.11366) stores self-reflections as long-term memory reused across episodes — reflections are not discarded when the task is rephrased.

## Goals / Non-Goals

**Goals:**
- A dormant conviction is always observable/revivable from the CLI (fix #1).
- `--help` never mutates the filesystem (fix #2).
- Flat-file → episode migration keys on the lesson, not YAML metadata (fix #3).
- The CEO loop and cross-organ write-in are pinned by tests + docs (fix #4).
- Baseline stays green; shield conformance golden vectors untouched.

**Non-Goals:**
- Changing dormancy *semantics* (objective-scoping of convictions stays as designed). We surface dormant convictions; we do not stop objective edits from retiring frames.
- Re-tuning consolidation/curate thresholds or clustering precision (noted as a known gap, out of scope for this harden).
- Adding a git-init to `init` (we reconcile the docs instead — brains are plain-text dirs, git optional).
- Any network/model dependency in the engine.

## Decisions

**D1 — Surface dormancy, don't change it.** `Brain.Convictions(includeDormant bool)` already exists. Add a count helper the status path uses to report `active` and `dormant` separately, and thread an `--all` flag into the `convictions` command that calls `Convictions(true)` and labels dormant entries. *Alternative considered:* auto-reviving safety convictions across frames — rejected: it silently overrides the deliberate goal-scoping and is fuzzy ("what is a safety conviction?"). Visibility is the honest, minimal fix; the operator (or a higher layer) decides revival. Research fit: Reflexion/Generative Agents keep reflections retrievable — visibility restores that without discarding the goal-frame model.

**D2 — `--help` short-circuits before side effects.** `install-skills` currently installs unconditionally; add an early check: if args contain `-h/--help`, print usage and return. *Alternative:* a global flag parser — larger blast radius; the local guard is surgical.

**D3 — Frontmatter strip lives in the ingest/record path.** Strip a leading `---\n … \n---` block before `ChunkFile` chunks the body; parse `description:`/`name:` for the cue. Keep it a pure, testable helper. Files without a leading `---` are byte-for-byte unchanged (guards the existing `record_fromfile_test.go`). *Alternative:* a full YAML parser dependency — unnecessary; a minimal leading-fence strip + two-key scan covers the memory-file format without new deps.

**D4 — Conformance as a Go integration test, not a JSON golden-vector.** The shield conformance is a pure-function golden-vector file (`conformance/cases/shield.json`); the CEO loop is *stateful* (record→consolidate→recall→check→record), so it belongs in a Go test driving the engine/CLI. Cross-organ write-in (citenexus verdict → episode; fleet loads playbook) is asserted in the same test file. Documented in `docs/`.

## Risks / Trade-offs

- **[Richer status line breaks a scripted parser]** → keep the existing `convictions : N` token present; append the dormant detail so any strict-prefix parse still reads the active count. Verify no test asserts the exact full line.
- **[Frontmatter strip changes output for files that legitimately start with `---`]** → only strip a *balanced* leading fence (`---` … `---`); a lone `---` or a body horizontal rule is left intact. Covered by the "plain files unaffected" scenario.
- **[Scope creep into consolidation thresholds]** → explicitly out of scope; the cardinal-sin-lesson dilution (cluster consistency 0.57 < 0.6) is recorded as a known gap in the report, not fixed here.
- **[Doc reconciliation touches README/INPROGRESS]** → text-only, no behavior; low risk.

## Migration Plan

1. Land engine + CLI fixes behind additive flags; run `go test ./...`.
2. Add per-fix tests + the conformance test; run again.
3. Migrate 3 read-only sample memory files as the batch-guidance proof.
4. Reconcile docs. No rollback complexity — all changes are additive; revert the commit if needed.

## Open Questions

- Should a future change let the operator pin a conviction as **cross-objective** (never dormant)? Deferred — belongs with a broader convictions-lifecycle change, not this harden.
