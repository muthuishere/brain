package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A memory .md file as the CEO's flat-file store actually shapes them.
const memFile = `---
name: recall-before-deciding
description: Always recall prior experience before making a decision, to avoid rebuilding what the brain already knows.
metadata:
  type: feedback
---

Skipping recall wasted a full day rebuilding a component the brain had already learned.

The fix is a recall-before-decision ritual: query the brain first, then act.
`

func TestSplitFrontmatter_ParsesTopLevelKeysAndBody(t *testing.T) {
	meta, body := SplitFrontmatter([]byte(memFile))
	if meta["name"] != "recall-before-deciding" {
		t.Fatalf("name = %q, want recall-before-deciding", meta["name"])
	}
	if !strings.HasPrefix(meta["description"], "Always recall prior experience") {
		t.Fatalf("description not parsed: %q", meta["description"])
	}
	// Nested/indented keys (under metadata:) must not be captured as top-level.
	if _, ok := meta["type"]; ok {
		t.Fatalf("indented nested key 'type' should not be captured, got %q", meta["type"])
	}
	if strings.Contains(string(body), "description:") || strings.Contains(string(body), "---") {
		t.Fatalf("body still contains frontmatter markers:\n%s", body)
	}
	if !strings.Contains(string(body), "Skipping recall wasted") {
		t.Fatalf("body lost its content:\n%s", body)
	}
}

func TestFrontmatterCue_PrefersDescriptionThenName(t *testing.T) {
	if got := FrontmatterCue(map[string]string{"name": "n", "description": "d"}); got != "d" {
		t.Fatalf("cue = %q, want description 'd'", got)
	}
	if got := FrontmatterCue(map[string]string{"name": "n"}); got != "n" {
		t.Fatalf("cue = %q, want name 'n'", got)
	}
	if got := FrontmatterCue(nil); got != "" {
		t.Fatalf("cue for nil meta = %q, want empty", got)
	}
}

func TestChunkFileFM_StripsFrontmatterAndReturnsCue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.md")
	if err := os.WriteFile(path, []byte(memFile), 0o644); err != nil {
		t.Fatal(err)
	}
	chunks, cue, err := ChunkFileFM(path, 450, 60)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cue, "Always recall prior experience") {
		t.Fatalf("cue = %q, want the description", cue)
	}
	for _, c := range chunks {
		if strings.Contains(c.Text, "name:") || strings.Contains(c.Text, "description:") {
			t.Fatalf("chunk still holds frontmatter text: %q", c.Text)
		}
	}
}

// Plain files with no leading frontmatter must be byte-for-byte identical to the
// pre-change ChunkFile path (guards the existing record --from-file contract).
func TestChunkFileFM_PlainFileUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	content := "first block of text.\n\nsecond block of text."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	fmChunks, cue, err := ChunkFileFM(path, 450, 60)
	if err != nil {
		t.Fatal(err)
	}
	plainChunks, err := ChunkFile(path, 450, 60)
	if err != nil {
		t.Fatal(err)
	}
	if cue != "" {
		t.Fatalf("plain file should yield an empty cue, got %q", cue)
	}
	if len(fmChunks) != len(plainChunks) {
		t.Fatalf("chunk count differs: FM=%d plain=%d", len(fmChunks), len(plainChunks))
	}
	for i := range plainChunks {
		if fmChunks[i].Text != plainChunks[i].Text || fmChunks[i].ID != plainChunks[i].ID {
			t.Fatalf("chunk %d differs: FM=%+v plain=%+v", i, fmChunks[i], plainChunks[i])
		}
	}
}

// A lone leading "---" with no closing fence is ordinary content, not frontmatter.
func TestSplitFrontmatter_UnbalancedFenceLeftIntact(t *testing.T) {
	in := []byte("---\nnot really frontmatter\nno closing fence here")
	meta, body := SplitFrontmatter(in)
	if meta != nil {
		t.Fatalf("expected nil meta for unbalanced fence, got %v", meta)
	}
	if string(body) != string(in) {
		t.Fatalf("body altered for unbalanced fence:\n%s", body)
	}
}
