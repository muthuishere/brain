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
- Design + research: [`docs/BRAIN.md`](docs/BRAIN.md).

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
