# Migrating flat memory files into a brain

The CEO previously ran on **255 flat `.md` memory files** (frontmatter + body) with
no brain. `record --from-file` lifts them into episodes so they become recallable,
consolidatable evidence.

## Frontmatter-aware ingestion

Each memory file looks like:

```markdown
---
name: recall-first
description: Recall prior experience before deciding, so you never rebuild what the brain already knows.
metadata:
  type: feedback
---

<body: the actual lesson>
```

`record --from-file`:

- **strips the leading YAML frontmatter block** so `name:`/`metadata:` never become
  episode text (before this fix they were recorded verbatim and polluted recall);
- **uses the frontmatter `description:` (falling back to `name:`) as the episode
  cue**, so `recall` keys on the lesson's summary, not on metadata;
- leaves files **without** frontmatter byte-for-byte unchanged.

Pinned by `libs/go/ingest/frontmatter_test.go` and
`clis/go/brain/migration_test.go` (`TestMigrateFlatFilesIntoRecallableEpisodes`).

## Batch migration

One `record --from-file` per file:

```sh
brain --repo ~/.config/deemwar-one-os/ceo-brain init   # if not already a brain
for f in ~/muthu/.../deemwar-one-os/memory/*.md; do
  brain --repo ~/.config/deemwar-one-os/ceo-brain record --from-file "$f"
done
brain --repo ~/.config/deemwar-one-os/ceo-brain consolidate   # distil repeated lessons
```

Notes:

- Import is **additive and idempotent-ish by content**: episode IDs are derived
  from `namespace + text + version`, so re-importing an unchanged file re-adds
  episodes — migrate once, or clear the brain before a re-run.
- A file chunks into one episode per blank-line-separated block (long blocks split
  by `--max-tokens` / `--overlap`).
- After import, run `consolidate` so repeated lessons across files promote into
  convictions; run `reflect` + `curate --apply` to seed the playbook.
- Keep the brain folder in **git** for a free audit trail (`init` does not do this
  for you — see README).

## Verified on real files

Migrating the first three `deemwar-one-os/memory/*.md` files produced 3/3/6
episodes with the frontmatter stripped (first episode is body text, not
`---\nname:…`), and `recall` on each file's description returned `grounded: true`.
Command output is in `doctrine/organs/reports/brain.md`.
