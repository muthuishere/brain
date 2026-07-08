# SPEC-record-from-url-v1

## Motivation

`docs/SPEC-record-from-file-v1.md`'s own open questions flagged this as the
natural follow-on: "Should `--from-url` get its own opt-in design ... in a
future SPEC?" README's status table still listed web ingest as ⚠️
library-only — `ingest.Fetch`/`Crawl` implemented and tested, but nothing in
the CLI called them. This closes that gap the same way `record --from-file`
closed the file-ingest gap: wire already-tested library code into `record`,
opt-in, no new library logic.

## Goals

1. Ship `brain record --from-url URL`: fetch exactly one URL, chunk its body
   the same way `record --from-file` chunks a local file, and record one
   episode per chunk.
2. Strip HTML markup before chunking when the response is HTML — otherwise
   chunk text would be full of tags, defeating recall relevance.
3. Flip README's status-table row for single-URL ingest from ⚠️ to ✅, keep
   `Crawl` (multi-page) explicitly ⚠️/deferred.

## Non-goals (v1)

- **No crawl wiring.** `ingest.Crawl` (same-host BFS, depth/page-capped)
  stays library-only. A crawl's network footprint (potentially dozens of
  pages per invocation, no per-call user confirmation) is a materially
  bigger opt-in-policy decision than fetching the one URL the caller
  explicitly named — that's a separate SPEC/huddle if pursued.
- **No JS rendering / readability extraction.** `StripHTML` is a
  dependency-free regex-based tag stripper (drops `<script>`/`<style>`
  bodies whole, strips remaining tags, unescapes the 4 common HTML
  entities), not a full HTML parser or a "main content" extractor. Good
  enough to keep chunk text readable; not a claim of parity with
  Readability/Trafilatura-grade extraction.
- **No retry/backoff/redirect-following beyond `net/http`'s default.**
  `ingest.Fetch` already existed with a 30s timeout and no retry; unchanged.
- **No content-type allowlist beyond the HTML/non-HTML branch.** A PDF or
  binary response is chunked as raw bytes-as-string today (same as
  `Crawl`/`Fetch`'s existing behavior) — not a new failure mode introduced
  here, just not specially handled either.
- **Same append-only/no-dedup/uniform-outcome tradeoffs as
  `SPEC-record-from-file-v1.md`** — re-fetching the same URL creates new
  episodes, `--reward`/`--label`/`--dimension` apply uniformly across all
  chunks from one fetch.

## Surface

- New flag on `record`: `--from-url URL`. Mutually exclusive with a
  positional TEXT argument and with `--from-file` (all three checked
  pairwise in `cmdRecord`).
- Optional `--max-tokens N` (default 450) / `--overlap N` (default 60),
  same defaults as `--from-file`.
- `--reward`/`--label`/`--dimension`, if passed, apply identically to every
  chunk's `Outcome`.
- Implementation: `ingest.FetchAndChunk(url, maxTokens, overlap)` —
  `Fetch(url)`, then `StripHTML` if the response `Content-Type` contains
  `html`, then `ChunkBytes(url, body, maxTokens, overlap)` (the same
  block-then-`chunker.ChunkText` split `ChunkFile` uses, refactored so both
  callers share it) — for each `ingest.Chunk`, `b.Record(chunk.Text,
  outcome)` in fetch order; prints one `recorded ep_...` line per episode
  plus a `recorded N episodes from URL` summary (JSON mode: array of
  episode IDs). Chunk IDs are addressed under the URL itself
  (`url::block::chunk`), mirroring `path::block::chunk` for files.
- The only network call this repo's CLI makes anywhere, and only when
  `--from-url` is explicitly passed — the offline-by-default guarantee
  (`README.md`'s "No network required") is preserved; this is opt-in,
  exactly like `endpoints.json`.

## Test plan

- `libs/go/ingest`: `StripHTML` strips tags/scripts/styles and unescapes
  entities while preserving visible text; `FetchAndChunk` against an
  `httptest` server for (a) an HTML response — chunks contain no markup,
  (b) a plain-text response — body passed through unmodified, (c) a 404 —
  error propagates, nothing recorded.
- `clis/go/brain`: hermetic `httptest`-backed integration tests mirroring
  `record_fromfile_test.go` — multi-chunk HTML page records the expected
  episode count/order/text; `--reward`/`--label`/`--dimension` propagate to
  every chunk's `Outcome`; flag-parsing pins `--from-url`/`--from-file`
  co-presence so `cmdRecord`'s mutual-exclusion checks have something to
  check against.
- `go vet && go build && go test ./...` stays green throughout — additive
  only, no change to existing `record`/`record --from-file` behavior.

## Rollout

1. Land `ingest.ChunkBytes` (extracted from `ChunkFile`) + `StripHTML` +
   `FetchAndChunk`, with tests.
2. Land `record --from-url` CLI wiring + its tests, confirm CI green.
3. Flip the README status-table row (single-URL ✅, multi-page crawl stays
   ⚠️) and add a `record --from-url` line to "Use directly".
4. Open/update PR; report `PR-OPEN` to the work queue.
5. Dogfood: point a research-flavored fleet brain at `record --from-url` for
   ingesting a source page directly, instead of manual copy/paste into
   `record --from-file`.

## Open questions (not blocking this cycle)

- If `Crawl` is ever wired in, should it require a separate opt-in (e.g.
  `--allow-crawl`) beyond `--from-url` given the larger network footprint?
- Is a content-type allowlist (reject obviously-binary responses before
  chunking) worth adding once real usage surfaces a bad case?
