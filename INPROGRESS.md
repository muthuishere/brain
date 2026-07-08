# brain — in progress

## NOW: self-evolving brain (2026-07-08) — owner working directly here

Full spec: **`docs/SPEC-self-evolving-v1.md`** · research digest:
`toolnexus/docs/references/self-evolving-agents-2026-07-08.md` · OS doctrine:
`deemwar-one-os/doctrine/self-improving-harness.md`.
Principle: **build on top of the existing core** (engine + CLI + skill) — extend, never rebuild.
Division: **brain learns, toolnexus acts** (agents load playbook/skills at runtime).

### Phase A — self-improving harness (experiences → evolving playbook) — DONE 2026-07-08
- [x] engine: `libs/go/engine/playbook.go` (Delta/PlaybookEntry/EvolvePolicy; Reflect/Regress/
      Curate pure funcs; content-dedup, supersession, genealogy; tests in playbook_test.go)
- [x] `brain reflect [--since TS]` — queues fresh deltas to `pending-deltas.ndjson`; idempotent
      (skips deltas already pending/rejected/in playbook SourceDeltas)
- [x] `brain curate [--apply]` — regress-gates each pending delta against validated convictions,
      merges survivors into `playbook.json`; rejections → `rejected-candidates.ndjson`
- [x] `brain playbook [--topic T] [--json]`
- [x] `brain regress <delta-id>` — standalone gate inspection
- [x] `consolidate --evolve` = consolidate→reflect→gate→curate --apply in one pass
- [x] thresholds in `evolve-policy.json` (owner-owned, defaults in DefaultEvolvePolicy)
- CLI wiring in `clis/go/brain/evolve.go`; smoke-tested end-to-end; all tests green.

### Phase B — self-evolving skills (SkillOpt, arXiv 2605.23904) — DONE 2026-07-08
- [x] engine: `libs/go/engine/skilllib.go` (SkillLib over `skills-lib/{primitives,composites,
      archived}/` + `metadata.json` + `usage.ndjson`; versions, lineage, rationale mandatory,
      metrics recomputed from the usage log; tests in skilllib_test.go)
- [x] `brain skill register --id ID --from FILE --rationale "why"` — v1 becomes current; later
      versions are CANDIDATES until validated (agent synthesizes content; CLI is bookkeeping only)
- [x] `brain skill validate ID --test-data DIR` — arithmetic gate, promote only ≥ min_improvement
      (default +5%, `min_improvement` in evolve-policy.json); rejected → archived with verdict
- [x] `brain skill log ID --outcome ok|fail [--task T] [--cost C]` — per-use feedback
- [x] `brain skill search / metrics / deprecate / rollback` — active-only search, windowed
      metrics, retirement with lineage kept, rollback refuses gate-rejected versions
- CLI wiring in `clis/go/brain/skill_cmds.go`; smoke-tested full lifecycle; all tests green.

### Phase C — everyone uses it
- [x] `skills/brain/SKILL.md`: reflect/curate/playbook + skill-lifecycle how-to (2026-07-08)
- [x] both skills updated (skills/brain + embedded copy synced; `install-skills` ships it)
- [x] toolnexus consumption — RESOLVED BY DESIGN (owner, 2026-07-08): no code edge in either
      direction. toolnexus/brain/citenexus stay independent libraries (only real edge:
      brain → citenexus). Agents compose them at the SKILL layer: any agent that wants
      memory invokes the globally-installed brain agent skill (`brain install-skills`),
      loads `brain playbook` / `brain skill search` before acting, records outcomes after.
      Opt-in per use; nothing to build in toolnexus.

### Guardrails (non-negotiable)
Evaluator outside the loop (thresholds are config the producer can't edit in-run) · rejected
candidates logged, never silently applied · negative results first-class · deterministic core /
agent-reasoning split preserved (no model endpoint in engine) · anti-collapse: probe
poor-looking paths on PAPER before promoting.

---

Standalone CLI for the **CiteNexus Brain**: an evidence-first, git-repo-backed
memory an agent records experiences into and recalls grounded, cite-or-abstain
answers from, with a deterministic constraint shield that vetoes "profitable but
ruinous" decisions. GitHub: `muthuishere/brain`.

## Layout (current)

```
brain/                 module github.com/muthuishere/brain (go 1.26)
├─ cmd/brain/                the CLI (command name stays `brain`)
│  ├─ main.go                subcommands + flags + install-skills
│  ├─ endpoints.go           OPTIONAL http embedding / reranker / llm clients
│  ├─ endpoints_test.go      hermetic httptest wire tests
│  └─ skill/                 embedded agent skill (SKILL.md, config + endpoints examples)
├─ engine/                   the brain engine (self-contained, no external deps)
│  ├─ brain.go store.go store_file.go shield.go conviction.go consolidate.go
│  └─ text.go + stopwords.json  (vendored tokenizer/gate/hash-embed + embedded stopwords)
├─ install.sh install.cmd    download prebuilt binary from GitHub Releases
├─ .github/workflows/        ci.yml (vet+build+test) · release.yml (v* tag → cross-compiled binaries)
└─ README.md
```

The engine is a **vendored copy** of the reference Go brain in
`rag-cite-nexus/golang/brain` (kept self-contained so `go install` needs nothing).
The reference brain also exists in Python (`rag-cite-nexus/python`), TypeScript
(`.../js`), and Rust (`.../rust`) — all green against the shared conformance contract.

## What works (verified locally)

- `go build ./cmd/brain`, `go test ./...` green; cross-compiles for
  darwin/linux/windows × amd64/arm64.
- **Offline by default**: no endpoints → deterministic hashing recall, no reranker,
  no LLM. `init · objective · record · reappraise · recall · learn · check ·
  consolidate · convictions · status · install-skills` all work.
- **Optional endpoints** (`endpoints.json` in the brain repo, or `BRAIN_EMBED_URL`
  etc. env): embedding (semantic recall), reranker (reorder), llm (synthesis) — each
  independent. `api_key_env` names an env var; the key is read at runtime, never
  stored/printed. Wire proven by httptest tests.
- A brain repo on disk is git-friendly plain text (`episodes.ndjson`,
  `convictions.json`, `objective.json`, `constraints.json`); `git log` is the
  reappraisal audit trail.

## TODO (to finish shipping)

1. **README refs**: a couple of spots still say `muthuishere/brain` — update to
   `muthuishere/brain`, and the `go install` path to
   `github.com/muthuishere/brain/cmd/brain@latest`. (install.sh / install.cmd /
   release.yml are already updated to brain + `./cmd/brain`.)
2. **Push + tag**: DONE — v0.3.0 released 2026-07-08 (self-evolving Phase A+B, shield
   fail-closed, ingest); binaries for darwin/linux/windows × amd64/arm64 + checksums
   published, install.sh verified end-to-end (new `skill`/`playbook` verbs present).
3. **Verify CI**: DONE — ci and release workflows green on GitHub (2026-07-08).
4. **Seed constraints**: replace the placeholder `never-ruin` / `never-n1` in
   `constraints.json` with the real inviolables for the brains you'll run.
5. (Optional) **Parity**: backport the reranker seam (added here in `engine`) to
   `rag-cite-nexus/golang/brain` so all four language ports stay identical.

## Install (once released)

```sh
curl -fsSL https://raw.githubusercontent.com/muthuishere/brain/main/install.sh | bash
# or: go install github.com/muthuishere/brain/cmd/brain@latest && brain install-skills
```

## Quick use

```sh
brain --repo ./mybrain init
brain --repo ./mybrain record "Overtrading in chop bled the account" --reward -1
brain --repo ./mybrain consolidate
brain --repo ./mybrain check "bet the account" --reward 0.95 --signal ruin_risk=1 --json
#   → allowed:false  alarm:true  vetoed_by:[never-ruin]   ("winning is actually losing")
```
