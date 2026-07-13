## Why

The brain is the CEO's memory organ. Exercising **every** command against a throwaway copy of the live CEO brain (`~/.config/deemwar-one-os/ceo-brain`) proved the core is sound — the constraint shield is correctly **fail-closed** (an absent signal for a hard constraint yields `undetermined → not allowed → not guaranteed`), record/consolidate/reflect/curate/playbook/skill/state/wake all run. But three real defects surface against the CEO's actual use, and the cross-organ contract is undocumented and untested:

1. **An objective edit silently zeroes the brain's convictions.** The CEO's hard-won conviction — *"faking is the cardinal sin; cite-or-abstain"* — went **dormant** the moment the objective text was edited (dormancy is keyed on exact `GoalFrame` string equality, `brain.go:271`). `status` then reports `convictions: 0` and `convictions` prints *"none yet"*, with **no CLI path** to see or revive it. Reflexion (Shinn 2023, arXiv 2303.11366) and Generative Agents (Park 2023, arXiv 2304.03442) both hold that distilled reflections are **persistent long-term memory**, retrieved by relevance/recency — not silently dropped when the current task is rephrased. A safety conviction that disappears on a wording change is a fundamentals bug. *(The shield is constraint-based and independent, so the veto path is NOT compromised — the harm is that the brain's point-of-view becomes invisible to recall-before-decision.)*

2. **`install-skills --help` performs the install** (writes to `~/.claude/skills/…`) instead of printing help — a help flag must never mutate the filesystem.

3. **The flat-file → episode migration ingests YAML frontmatter as episode text.** `record --from-file` on the CEO's 255 memory `.md` files records the `---\nname:…\ndescription:…` block verbatim, so `recall` returns raw frontmatter instead of the lesson.

4. **No cross-organ contract exists in code.** "Organs must work complementarily" (owner mandate 2026-07-13) requires a tested interface: how citenexus verdicts get recorded as episodes, how fleet workers load `playbook` before acting, and a conformance test that pins the CEO decision loop (recall → check → record).

## What Changes

- **Dormant-conviction visibility (fix #1).** `status` reports active **and** dormant conviction counts; `convictions --all` lists dormant convictions, clearly labeled. No change to dormancy *semantics* (objective-scoping stays) — only to the fact that a dormant conviction was previously **unreachable** from the CLI. Non-breaking (additive flag + richer status line).
- **`--help` is side-effect free (fix #2).** `install-skills --help` prints usage and exits 0 without installing.
- **Frontmatter-aware migration (fix #3).** `record --from-file` strips a leading YAML frontmatter block before chunking and uses its `description:` (falling back to `name:`) as the episode **cue**, so recall keys on the lesson, not the metadata. Add batch guidance for migrating a directory of memory files.
- **Cross-organ conformance (fix #4).** A new conformance test that pins the CEO loop (recall-before-decision, check-before-action, record-after-outcome) and the cross-organ write-in (a citenexus-style verdict recorded as an episode is recallable; `playbook` output is loadable by a fleet worker). Documented in `docs/`.

## Capabilities

### New Capabilities
- `conviction-visibility`: dormant convictions remain observable and revivable from the CLI (`status` counts + `convictions --all`), so an objective edit never silently blinds recall-before-decision.
- `flat-file-migration`: `record --from-file` is frontmatter-aware and there is documented + tested batch guidance for migrating a directory of legacy memory `.md` files into episodes.
- `cross-organ-contract`: a conformance test + doc pinning the CEO decision loop and how other organs (citenexus, fleet) write into / read from the brain.

### Modified Capabilities
<!-- No existing openspec/specs/ capabilities (first change in this repo); the shield/record behaviors above are captured as new capabilities. -->

## Impact

- **Code**: `libs/go/engine/brain.go` (status/conviction counts), `clis/go/brain/main.go` + `skill_cmds.go`/`evolve.go` (`convictions --all`, `install-skills --help`), `libs/go/ingest` + `clis/go/brain` record path (frontmatter strip), new `*_test.go` for each fix, a new conformance test (`clis/go/brain` or `libs/go/engine`).
- **Docs**: `docs/` gains a migration guide and a cross-organ contract doc; the git-audit-trail claim in README/INPROGRESS is reconciled with reality (brains are plain-text dirs; `git init` is optional, not automatic).
- **No breaking changes**: all additions are additive flags / richer output / a stricter `--help`. Existing `go test ./...` must stay green; the shield conformance golden vectors are untouched.
- **Consumers**: citenexus (records verdicts as episodes), fleet workers (load `playbook`), the CEO orientation flow (`wake`/`state`).
