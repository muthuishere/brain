# brain

A Go library and CLI that turns a **folder (a git repo) into a persistent,
evidence-first brain**. It records experiences, recalls grounded
cite-or-abstain answers, distils repeated experience into validated convictions,
and runs a **deterministic constraint shield** that vetoes "profitable but
ruinous" decisions.

No network required. No model endpoint required. The engine is the
deterministic core; an **agent** (a real LLM, via the bundled
[skill](skills/brain/SKILL.md)) is the reasoning layer, and **git history is
the audit trail**.

- **Library**: `github.com/muthuishere/brain/libs/go/engine` — the brain
  engine (episodic memory, shield, consolidation). Also `libs/go/storage`
  (local + S3-compatible object storage), `libs/go/ingest` (web crawl + file
  chunking), and `libs/go/modelclients` (optional HTTP embedding/reranker/LLM
  clients).
- **CLI**: `github.com/muthuishere/brain/clis/go/brain` — a thin binary over
  the library.
- Chunking/tokenizing are delegated to the tested
  [`citenexus/golang`](https://github.com/muthuishere/citenexus) module
  rather than reimplemented.
- Design + research: [`docs/BRAIN.md`](docs/BRAIN.md). Feature specs land as
  `docs/SPEC-<slug>-v1.md` — see [`docs/SPEC-record-from-file-v1.md`](docs/SPEC-record-from-file-v1.md).
- `libs/go/engine` (the shield veto, conviction confidence, consolidation
  clustering, and recall gating) is covered by tests under
  `libs/go/engine/*_test.go` — the safety-critical core is proven, not just
  asserted.

## Status — what's ready today

| Capability | Status |
|---|---|
| **Git/file storage** | ✅ Ready, default and only storage path the CLI uses. `engine.FileStore` persists `episodes.ndjson`/`convictions.json`/`objective.json` as plain files in a folder — commit that folder yourself, `git log` is the audit trail. |
| **Agent skill** | ✅ Ready. `brain install-skills` embeds and installs the skill (below) into `~/.claude/skills/brain`. |
| **Offline recall (hash embedding)** | ✅ Ready, zero network by default. |
| **Optional embedding/reranker/LLM endpoints** | ✅ Ready (`libs/go/modelclients`), opt-in via `endpoints.json`. |
| **S3 / object storage** (`libs/go/storage`) | ⚠️ Library-only. `S3Backend`/`LocalFsBackend` implement a generic bytes/JSON blob interface and are usable from Go code, but there is **no `engine.Store` implementation on S3 yet** and **no `--store s3` CLI flag** — `brain --repo` always uses the local file store today. |
| **File ingest** (`libs/go/ingest`) | ✅ Ready. `brain record --from-file PATH` chunks a local file (`ingest.ChunkFile`, deterministic, no network) and records one episode per chunk. |
| **Web crawl** (`libs/go/ingest`) | ⚠️ Library-only. `Fetch`/`Crawl` are implemented and tested, but not wired into the CLI — reaching the network from `record` needs its own opt-in design (see [`docs/SPEC-record-from-file-v1.md`](docs/SPEC-record-from-file-v1.md)'s non-goals). Call them from Go if you want web ingest today. |

## Install

Prebuilt binaries from GitHub Releases — no Go toolchain needed. The installer
also drops the agent skill into `~/.claude/skills/brain` and seeds
`~/.config/brain/config.json`.

**macOS / Linux**
```sh
curl -fsSL https://raw.githubusercontent.com/muthuishere/brain/main/install.sh | bash
```

**Windows** (PowerShell / cmd) — download `install.cmd` from the repo and run it, or:
```cmd
curl -fsSL -o install.cmd https://raw.githubusercontent.com/muthuishere/brain/main/install.cmd && install.cmd
```

Already have Go? `go install github.com/muthuishere/brain/clis/go/brain@latest && brain install-skills` also works.

Release binaries are built for darwin/linux/windows (amd64 + arm64) by
[`.github/workflows/release.yml`](.github/workflows/release.yml) on every `v*` tag.

## Use directly

```sh
brain --repo ./mybrain init
brain --repo ./mybrain objective "preserve capital"
brain --repo ./mybrain record "Overtrading in chop bled the account" --reward -1
brain --repo ./mybrain record "Overtrading in chop bled the account again" --reward -1
brain --repo ./mybrain consolidate          # → distils a validated conviction
brain --repo ./mybrain recall "overtrading" --json
brain --repo ./mybrain check "bet the account" --reward 0.95 --signal ruin_risk=1 --json
#   → allowed:false  alarm:true  vetoed_by:[never-ruin]   ("winning is actually losing")

# bulk-ingest a doc into episodic memory — one episode per chunk, no network:
brain --repo ./mybrain record --from-file postmortem.md --reward -1 --label "postmortem"
```

## Endpoints (optional — works fully offline without them)

By default the CLI runs with **no network**: deterministic hashing recall, no
reranker, no LLM. To upgrade, drop an `endpoints.json` in the brain repo
(`init` writes an `endpoints.example.json` to copy):

```json
{
  "embedding": {"base_url": "http://localhost:11434/v1", "model": "bge-m3", "api_key_env": "OLLAMA_API_KEY"},
  "reranker":  {"base_url": "http://localhost:11434", "model": "bge-reranker-v2-m3", "api_key_env": ""},
  "llm":       {"base_url": "http://localhost:11434/v1", "model": "qwen2.5", "api_key_env": ""}
}
```

Each block is independent. `api_key_env` names an env var — the key is read at
runtime and never stored or printed. Env overrides: `BRAIN_EMBED_URL`,
`BRAIN_RERANK_URL`, `BRAIN_LLM_URL` (+ `_MODEL`, `_KEY_ENV`).

## Agent skill

`brain install-skills` installs [`skills/brain/SKILL.md`](skills/brain/SKILL.md)
into `~/.claude/skills/brain/SKILL.md` and seeds `~/.config/brain/config.json`
(a `{default, brains: {name: repoPath}}` map so an agent can resolve "the
crypto brain" → a repo folder). Once installed, an agent should:

1. **Resolve the brain**: look up the named brain in `~/.config/brain/config.json`
   (or the default), and always pass `--repo "$REPO" --json`.
2. **Use the command table**: `objective` (set the goal), `record` (log an
   experience with `--reward`), `reappraise` (flip a past judgment),
   `recall`/`learn` (grounded, cite-or-abstain answers), `check` (the
   constraint shield — pass `--signal name=value` for measured risk signals),
   `consolidate` (distil repeated experience into convictions), `convictions`
   (list current beliefs).
3. **Commit after every mutation** — `objective`/`record`/`reappraise`/`consolidate`
   change the repo; `git add -A && git commit` right after so git history stays
   the audit trail. `recall`/`learn`/`check`/`convictions` are read-only, no commit.
4. **Honor `check`'s verdict literally**: `"allowed": false` → don't take the
   decision, offer `"fallback"` instead. `"alarm": true` → "profitable but
   ruinous" — the loudest signal, surface it to the human rather than
   proceeding quietly. `"guaranteed": true` → every constraint was
   self-evaluated in code; if false, some cost came from outside and the
   verdict is advisory.
5. **Keep multiple brains isolated** — each is its own git repo; never merge
   two brains' folders. To share one brain across agents, share the same repo
   (commit/pull) and let conflicting experiences surface as conflicts instead
   of being averaged away.

Full behavioral spec (constraints format, endpoint config, reading raw memory
directly): [`skills/brain/SKILL.md`](skills/brain/SKILL.md).

## The brain on disk (all git-friendly plain text)

```
mybrain/
├── constraints.json   the slow layer: what this brain will never do
├── objective.json     current foreground goal + retired history
├── episodes.ndjson    raw experiences, one JSON line each (append-only)
└── convictions.json   distilled, validated beliefs
```

`git log` over the folder is the reappraisal history — the brain never overwrites
its past, unlike a human.

## Design in one line

Signal-to-noise, made honest: it states a belief only when experience *repeats and
stays consistent*, abstains on the noise, surfaces conflict instead of flattening
it, and hard-vetoes decisions that win on the objective but break a standing
constraint. Four published ideas (goal-conditioned objectives, reflection-formed
convictions, truth-maintenance dormancy, and a constrained-RL shield) in a
composition nobody ships.
