package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/muthuishere/brain/libs/go/engine"
	"github.com/muthuishere/brain/libs/go/ingest"
)

// TestCmdRecordFromFile is a hermetic, no-network integration test for the
// `record --from-file` CLI surface: it inits a temp brain repo, writes a
// multi-paragraph fixture file, drives cmdRecord directly (this is package
// main, so unexported functions are callable in-package — matching the
// pattern in endpoints_test.go), then asserts episodes.ndjson holds exactly
// one episode per chunk ingest.ChunkFile would produce, in the same order
// with matching text.
func TestCmdRecordFromFile(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	docPath := filepath.Join(t.TempDir(), "doc.txt")
	content := "Paragraph one has a handful of words to chunk on for the test.\n\n" +
		"Paragraph two is a separate block of text, entirely distinct from the first.\n\n" +
		"Paragraph three rounds things out so ChunkFile has multiple blocks to walk."
	if err := os.WriteFile(docPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	wantChunks, err := ingest.ChunkFile(docPath, 450, 60)
	if err != nil {
		t.Fatalf("ingest.ChunkFile: %v", err)
	}
	if len(wantChunks) < 2 {
		t.Fatalf("fixture produced only %d chunk(s); need >= 2 to exercise multi-episode recording", len(wantChunks))
	}

	cmdRecord(repo, []string{"--from-file", docPath})

	fs, err := engine.NewFileStore(repo)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	got := fs.Episodes()

	if len(got) != len(wantChunks) {
		t.Fatalf("episodes recorded = %d, want %d (matching chunk count)", len(got), len(wantChunks))
	}
	for i := range wantChunks {
		if got[i].Text != wantChunks[i].Text {
			t.Errorf("episode[%d].Text = %q, want %q", i, got[i].Text, wantChunks[i].Text)
		}
	}
}

// TestCmdRecordFromFileWithReward checks that --reward/--label/--dimension
// apply uniformly to every chunk's Outcome, and that each episode gets its
// own Outcome value (not a shared pointer that could later be mutated out
// from under earlier episodes).
func TestCmdRecordFromFileWithReward(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	docPath := filepath.Join(t.TempDir(), "doc.txt")
	content := "First paragraph block for reward propagation testing purposes here.\n\n" +
		"Second paragraph block, a wholly separate section of the same document."
	if err := os.WriteFile(docPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	wantChunks, err := ingest.ChunkFile(docPath, 450, 60)
	if err != nil {
		t.Fatalf("ingest.ChunkFile: %v", err)
	}

	cmdRecord(repo, []string{"--from-file", docPath, "--reward", "0.5", "--label", "good", "--dimension", "quality"})

	fs, err := engine.NewFileStore(repo)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	got := fs.Episodes()
	if len(got) != len(wantChunks) {
		t.Fatalf("episodes recorded = %d, want %d", len(got), len(wantChunks))
	}
	for i, ep := range got {
		if ep.Outcome == nil {
			t.Fatalf("episode[%d].Outcome = nil, want non-nil", i)
		}
		if ep.Outcome.Reward != 0.5 || ep.Outcome.Label != "good" || ep.Outcome.Dimension != "quality" {
			t.Errorf("episode[%d].Outcome = %+v, want Reward=0.5 Label=good Dimension=quality", i, ep.Outcome)
		}
	}
	// Each episode must own its Outcome pointer, not share one across chunks.
	for i := 1; i < len(got); i++ {
		if got[i].Outcome == got[0].Outcome {
			t.Errorf("episode[%d] shares an *Outcome pointer with episode[0]; each chunk should get its own", i)
		}
	}
}

// TestCmdRecordMutuallyExclusiveWithFromFile documents (via the flag-parsing
// helper, not a full process exit) that a positional TEXT arg together with
// --from-file is rejected before any recording happens.
func TestCmdRecordMutuallyExclusiveWithFromFile(t *testing.T) {
	text, f := firstArgAndFlags([]string{"some text", "--from-file", "doc.txt"})
	if text == "" {
		t.Fatalf("expected positional text to be parsed")
	}
	if path, ok := f.strs["from-file"]; !ok || path != "doc.txt" {
		t.Fatalf("expected --from-file to be parsed as doc.txt, got %q (ok=%v)", path, ok)
	}
	// cmdRecord would fatal() (os.Exit) in this situation, which we can't
	// safely exercise in-process; the flags above are exactly what cmdRecord
	// checks to decide that, so this pins the precondition.
}
