# brain — in progress

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
2. **Push + tag**: push to `muthuishere/brain`, then `git tag v0.1.0 && git
   push --tags` to trigger `release.yml` → publishes cross-compiled binaries that
   `install.sh` / `install.cmd` download.
3. **Verify CI**: confirm the Actions run green on GitHub (runners must have Go 1.26,
   as the rag-cite-nexus workflows already assume).
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
