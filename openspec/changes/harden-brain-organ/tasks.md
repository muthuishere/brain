## 1. Conviction visibility (fix #1)

- [x] 1.1 Add a conviction-count helper to the engine reporting active vs dormant counts
- [x] 1.2 `status` prints `convictions: N active (M dormant)` keeping the existing token readable
- [x] 1.3 `convictions --all` lists dormant convictions labeled `[dormant]`; default stays active-only
- [x] 1.4 Test: objective edit → status shows the dormant count; `convictions --all` surfaces it; default hides it; re-editing the objective back revives it
- [x] 1.5 Test: a hard-constraint `check` still vetoes when all convictions are dormant (shield independence)

## 2. Side-effect-free help (fix #2)

- [x] 2.1 `install-skills` short-circuits on `-h`/`--help`, printing usage without installing
- [x] 2.2 Test: `install-skills --help` writes nothing and exits 0

## 3. Frontmatter-aware migration (fix #3)

- [x] 3.1 Add a pure helper that strips a balanced leading YAML frontmatter block and returns body + description/name cue
- [x] 3.2 Wire it into the `record --from-file` path; plain files unchanged
- [x] 3.3 Test: frontmatter not stored as episode text; description becomes the cue; plain files byte-for-byte identical to prior behavior
- [x] 3.4 Migrate 3 read-only sample memory `.md` files from the one-os `memory/` dir into a fresh brain and assert each is recallable

## 4. Cross-organ conformance (fix #4)

- [x] 4.1 Add a Go conformance test pinning the CEO loop: recall-before-decision, check-before-action (veto + fail-closed), record-after-outcome
- [x] 4.2 Same test asserts a citenexus-style verdict recorded as an episode is recallable, and `playbook` output is loadable by a worker
- [x] 4.3 Write `docs/` cross-organ contract + migration guide; reconcile the git-audit-trail claim in README/INPROGRESS with reality

## 5. Verify

- [x] 5.1 `go test ./...` green (paste output)
- [x] 5.2 Re-exercise the live CEO-brain copy for the fixed behaviors (paste output)
- [x] 5.3 Huddle verify the diff; keep transcript path
- [x] 5.4 `openspec validate harden-brain-organ` passes
