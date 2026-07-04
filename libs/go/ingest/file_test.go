package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChunkFileBlocksAndIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	content := "first block of text.\n\nsecond block of text."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	chunks, err := ChunkFile(path, 450, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks (one per block), got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].ID != path+"::0::0" {
		t.Fatalf("unexpected id: %s", chunks[0].ID)
	}
	if chunks[1].BlockOrder != 1 {
		t.Fatalf("expected second chunk from block 1, got %d", chunks[1].BlockOrder)
	}
}
