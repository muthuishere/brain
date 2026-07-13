package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures shaped exactly like the CEO's flat-file memory (.md with YAML
// frontmatter: name/description/metadata, then a body). This is the format of
// the 255 files under the one-os memory/ dir the migration must lift.
var memFixtures = map[string]string{
	"ack-within-10s.md": `---
name: ack-within-10s
description: The CEO must acknowledge every owner message within ten seconds and always delegate work to agents.
metadata:
  type: feedback
---

Grinding in-session instead of delegating made the owner wait and stalled the fleet.

The rule: auto-ack fast, then hand the task to an agent — never block on it yourself.
`,
	"recall-first.md": `---
name: recall-first
description: Recall prior experience before deciding, so you never rebuild what the brain already knows.
metadata:
  type: project
---

A full day was lost rebuilding a component the brain had already learned about.

Query the brain first, then act on what it already knows.
`,
	"never-fake.md": `---
name: never-fake
description: Faking a done or real claim is the cardinal sin; cite proof or abstain.
metadata:
  type: feedback
---

A page claimed data was real over fabricated content — the cardinal sin for a verifiable-autonomy thesis.

If you cannot cite a rendered artifact, a live 200, or a real row, do not claim it is done.
`,
}

// TestMigrateFlatFilesIntoRecallableEpisodes is the migration complementarity
// proof: a directory of legacy memory .md files imported via `record --from-file`
// yields recallable episodes, keyed on the lesson (description) rather than the
// YAML frontmatter. Mirrors the batch guidance documented in docs/.
func TestMigrateFlatFilesIntoRecallableEpisodes(t *testing.T) {
	memDir := t.TempDir()
	for name, body := range memFixtures {
		if err := os.WriteFile(filepath.Join(memDir, name), []byte(body), 0o444); err != nil { // 0o444 = read-only source
			t.Fatal(err)
		}
	}

	repo := t.TempDir()
	cmdInit(repo)

	// Batch pattern: one record --from-file per memory file.
	entries, _ := os.ReadDir(memDir)
	for _, e := range entries {
		cmdRecord(repo, []string{"--from-file", filepath.Join(memDir, e.Name())})
	}

	// No episode may hold raw frontmatter text.
	for _, ep := range mustStore(t, repo).Episodes() {
		if strings.Contains(ep.Text, "description:") || strings.Contains(ep.Text, "metadata:") || strings.HasPrefix(strings.TrimSpace(ep.Text), "---") {
			t.Fatalf("migrated episode still holds frontmatter: %q", ep.Text)
		}
	}

	// Each file's lesson is recallable by words from its description.
	cases := map[string]string{
		"acknowledge every owner message": "ack",
		"rebuild what the brain already":  "recall",
		"cardinal sin":                    "fake",
	}
	for query, label := range cases {
		out := captureStdout(t, func() {
			cmdRecall(repo, []string{query, "--json"})
		})
		var r map[string]any
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatalf("recall --json (%s): %v\n%s", label, err, out)
		}
		if grounded, _ := r["grounded"].(bool); !grounded {
			t.Fatalf("migrated file %q must be recallable by its description; got: %s", label, out)
		}
	}
}
