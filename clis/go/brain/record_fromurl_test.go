package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muthuishere/brain/libs/go/engine"
	"github.com/muthuishere/brain/libs/go/ingest"
)

// TestCmdRecordFromURL is a hermetic integration test for the `record
// --from-url` CLI surface (the only network path this CLI has, and only
// when explicitly invoked): a local httptest server stands in for the
// network, cmdRecord fetches from it, and we assert episodes.ndjson holds
// exactly one episode per chunk ingest.FetchAndChunk would produce, HTML
// markup stripped.
func TestCmdRecordFromURL(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><p>First paragraph block of real content here for chunking.</p>` +
			`<p>Second paragraph block, a wholly separate section for the same fetched page.</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := srv.URL + "/page"

	repo := t.TempDir()
	cmdInit(repo)

	wantChunks, err := ingest.FetchAndChunk(url, 450, 60)
	if err != nil {
		t.Fatalf("ingest.FetchAndChunk: %v", err)
	}
	if len(wantChunks) < 2 {
		t.Fatalf("fixture produced only %d chunk(s); need >= 2 to exercise multi-episode recording", len(wantChunks))
	}

	cmdRecord(repo, []string{"--from-url", url})

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

// TestCmdRecordFromURLWithReward checks --reward/--label/--dimension apply
// uniformly to every chunk's Outcome, mirroring the --from-file behavior.
func TestCmdRecordFromURLWithReward(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/doc.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("First paragraph block for reward propagation testing purposes.\n\n" +
			"Second paragraph block, a wholly separate section of the same document."))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	url := srv.URL + "/doc.txt"

	repo := t.TempDir()
	cmdInit(repo)

	cmdRecord(repo, []string{"--from-url", url, "--reward", "0.5", "--label", "good", "--dimension", "quality"})

	fs, err := engine.NewFileStore(repo)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	got := fs.Episodes()
	if len(got) == 0 {
		t.Fatalf("expected at least 1 episode recorded")
	}
	for i, ep := range got {
		if ep.Outcome == nil {
			t.Fatalf("episode[%d].Outcome = nil, want non-nil", i)
		}
		if ep.Outcome.Reward != 0.5 || ep.Outcome.Label != "good" || ep.Outcome.Dimension != "quality" {
			t.Errorf("episode[%d].Outcome = %+v, want Reward=0.5 Label=good Dimension=quality", i, ep.Outcome)
		}
	}
}

// TestCmdRecordMutuallyExclusiveWithFromURL documents (via the flag-parsing
// helper, not a full process exit) that TEXT/--from-file/--from-url are
// pairwise mutually exclusive before any recording happens.
func TestCmdRecordMutuallyExclusiveWithFromURL(t *testing.T) {
	text, f := firstArgAndFlags([]string{"some text", "--from-url", "https://example.com/doc"})
	if text == "" {
		t.Fatalf("expected positional text to be parsed")
	}
	if url, ok := f.strs["from-url"]; !ok || url != "https://example.com/doc" {
		t.Fatalf("expected --from-url to be parsed as https://example.com/doc, got %q (ok=%v)", url, ok)
	}

	_, f2 := firstArgAndFlags([]string{"--from-file", "doc.txt", "--from-url", "https://example.com/doc"})
	if _, ok := f2.strs["from-file"]; !ok {
		t.Fatalf("expected --from-file to be parsed")
	}
	if _, ok := f2.strs["from-url"]; !ok {
		t.Fatalf("expected --from-url to be parsed")
	}
	// cmdRecord would fatal() (os.Exit) in both situations above, which we
	// can't safely exercise in-process; the flags pinned here are exactly
	// what cmdRecord checks to decide that.
}
