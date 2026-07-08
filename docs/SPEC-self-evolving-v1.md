# SPEC — brain self-improvement + self-evolving skills (v1)

Owner directive 2026-07-08: the brain is the OS's self-improvement engine — **build on top of
the existing core** (`libs/go/engine` + `clis/go/brain` + `skills/brain`), do NOT rebuild.
Sources digested in `toolnexus/docs/references/self-evolving-agents-2026-07-08.md`
(Lilian Weng "Agent Harness" 2026-07-04 + arXiv 2605.23904 "SkillOpt").

Division of labor: **brain learns, toolnexus acts** — toolnexus/agents load the playbook +
skill library at runtime; the brain owns the learning lifecycle.

## A. Self-improving harness (experiences → evolving playbook)

Extends the existing episodic store + consolidation. New engine funcs in `libs/go/engine`,
thin CLI verbs in `clis/go/brain`, prompts/how-to in `skills/brain/SKILL.md`.

- `brain reflect [--since TS] [--json]` — distill recent episodes (successes AND failures)
  into candidate **playbook deltas**: {rule, evidence episode-ids, verifier-grounded root
  cause, reward}. Read-only; proposes only. Deterministic core selects/structures; the
  agent (skill layer) writes the prose — same split as record/recall today.
- `brain curate [--apply] [--json]` — merge accepted deltas into `playbook.json` (structured,
  itemized entries — never a growing blob). Dedup by CONTENT, supersede stale rules, keep
  genealogy. `--apply` writes + relies on the folder's git commit as audit trail.
- `brain playbook [--topic T] [--json]` — print the current itemized playbook (what an agent
  loads before acting).
- `brain regress <delta-id> [--json]` — **regression-gate**: check a proposed delta against
  held-in episodes that already succeeded; reject if it contradicts a validated conviction or
  a previously-winning setup. Rejections logged to `rejected-candidates.ndjson`, never applied.

`consolidate` becomes the scheduled curator pass (reflect→curate→regress in one run behind
`consolidate --evolve`).

## B. Self-evolving skills (SkillOpt lifecycle)

A versioned **skill library** inside the brain folder: `skills-lib/{primitives,composites,archived}/`
+ `skills-lib/metadata.json` (versions, lineage, metrics). A skill = executable procedure spec:
id, description, preconditions, success_criteria, cost {api_calls, latency_ms}, metrics
{success_rate, invocation_count}.

- `brain skill register --from <trace|file> [--validation SET]` — failure-driven synthesis:
  a repeated failure pattern from `reflect` proposes a new/modified skill version.
- `brain skill validate <id> --test-data DIR [--json]` — run against held-out cases, compare to
  prior version; accept only on ≥ threshold improvement (default +5%), else archive.
- `brain skill search --domain D [--min-success R]` · `brain skill metrics <id> [--window 7d]`
- `brain skill deprecate <id>` — retire low-usage/degraded skills; lineage kept, rollback possible.
- `brain skill log <id> --task T --outcome ok|fail [--cost C]` — the per-use feedback record
  that feeds metrics (called by agents after each invocation).
- Composites chain primitives; every change stores its rationale (explainability).

## Guardrails (both)
- Evaluator OUTSIDE the loop: acceptance gates (regress/validate thresholds) are config the
  producer can't edit in the same run; CEO/owner own the thresholds.
- Anti-collapse: periodically probe an initially-poor-looking path on PAPER before promoting.
- Negative results first-class: failures recorded honestly; rejected candidates kept for learning.
- Deterministic core / agent reasoning split preserved — no model endpoint in the engine.
- Nothing outward/irreversible auto-applies.

## Rollout
1. Implement A then B (engine → CLI → skill docs), tests per verb, demo in README.
2. `skills/brain/SKILL.md` gains the reflect/curate/skill-lifecycle how-to so EVERY agent
   (fleet lines, desks, CEO) runs the loop: reflect+curate at each work-done/session-end;
   skill lifecycle at daily retro.
3. toolnexus consumes: agents load `brain playbook` + `brain skill search` at spawn.
