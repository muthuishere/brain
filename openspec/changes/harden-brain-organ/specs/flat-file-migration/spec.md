## ADDED Requirements

### Requirement: record --from-file is frontmatter-aware
When importing a legacy memory `.md` file, `record --from-file` MUST strip a leading YAML frontmatter block (a `---` fenced block at the very start of the file) before chunking the body into episodes, and MUST use the frontmatter's `description:` value (falling back to `name:`) as the episode cue when present. Files with no frontmatter MUST behave exactly as before.

#### Scenario: frontmatter is not stored as episode text
- **WHEN** `record --from-file` imports a file that begins with a `---` YAML frontmatter block followed by body paragraphs
- **THEN** no recorded episode's text is the raw `---\nname: …` frontmatter block; episodes hold the body content

#### Scenario: description becomes the cue
- **WHEN** the imported file's frontmatter has a `description:` field
- **THEN** at least the first recorded episode carries that description as its cue, so `recall` on words from the description returns the file's content grounded

#### Scenario: plain files are unaffected
- **WHEN** `record --from-file` imports a file with no leading frontmatter
- **THEN** the episodes recorded are identical to the pre-change behavior (same count and text as `ingest.ChunkFile` produces)

### Requirement: Documented batch migration guidance
There SHALL be documented, tested guidance for migrating a directory of legacy memory `.md` files into a brain via repeated `record --from-file`, so the CEO's flat-file memory can be lifted into episodes.

#### Scenario: a directory of memory files migrates into recallable episodes
- **WHEN** three sample memory `.md` files are imported into a fresh brain following the documented batch pattern
- **THEN** each file yields one or more episodes and `recall` on content from each file returns grounded answers
