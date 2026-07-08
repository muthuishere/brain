package ingest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrawlSameHostBFSWithDepthAndPageCaps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/b">b</a>`))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/c">c</a><a href="/a">back to a</a>`))
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`no links here`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	results := Crawl(srv.URL+"/a", CrawlOpts{MaxPages: 10, MaxDepth: 5})
	if len(results) != 3 {
		t.Fatalf("expected 3 pages (a,b,c, deduped), got %d: %+v", len(results), results)
	}
}

func TestCrawlRespectsMaxPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/b">b</a>`))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/c">c</a>`))
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`done`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	results := Crawl(srv.URL+"/a", CrawlOpts{MaxPages: 1, MaxDepth: 5})
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 page due to MaxPages cap, got %d", len(results))
	}
}

func TestCrawlToleratesPerPageFailures(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<a href="/missing">missing</a><a href="/b">b</a>`))
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`ok`))
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	results := Crawl(srv.URL+"/a", CrawlOpts{MaxPages: 10, MaxDepth: 5})
	if len(results) != 2 {
		t.Fatalf("expected 2 successful pages (a,b), missing page tolerated, got %d: %+v", len(results), results)
	}
}

func TestStripHTMLDropsTagsScriptsAndStyles(t *testing.T) {
	html := []byte(`<html><head><style>body{color:red}</style></head>` +
		`<body><script>alert('x')</script><h1>Title</h1><p>Hello &amp; welcome.</p></body></html>`)
	got := string(StripHTML(html))
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("StripHTML left tag markup behind: %q", got)
	}
	if strings.Contains(got, "color:red") || strings.Contains(got, "alert") {
		t.Fatalf("StripHTML left script/style content behind: %q", got)
	}
	if !strings.Contains(got, "Title") || !strings.Contains(got, "Hello & welcome.") {
		t.Fatalf("StripHTML dropped visible text, got %q", got)
	}
}

func TestFetchAndChunkHTML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><p>First paragraph block of real content here for chunking.</p>` +
			`<p>Second paragraph block, a wholly separate section for the same fetched page.</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chunks, err := FetchAndChunk(srv.URL+"/page", 450, 60)
	if err != nil {
		t.Fatalf("FetchAndChunk: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected at least 1 chunk, got 0")
	}
	for _, c := range chunks {
		if strings.Contains(c.Text, "<p>") || strings.Contains(c.Text, "<html>") {
			t.Errorf("chunk text still contains HTML markup: %q", c.Text)
		}
		if !strings.HasPrefix(c.ID, srv.URL+"/page::") {
			t.Errorf("chunk.ID = %q, want prefix %q", c.ID, srv.URL+"/page::")
		}
	}
}

func TestFetchAndChunkPlainTextUnmodified(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/doc.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Just plain text content, no markup to strip at all here."))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	chunks, err := FetchAndChunk(srv.URL+"/doc.txt", 450, 60)
	if err != nil {
		t.Fatalf("FetchAndChunk: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected exactly 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text != "Just plain text content, no markup to strip at all here." {
		t.Errorf("plain text body was modified: %q", chunks[0].Text)
	}
}

func TestFetchAndChunkPropagatesFetchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	if _, err := FetchAndChunk(srv.URL+"/missing", 450, 60); err == nil {
		t.Fatalf("expected an error for a 404 response, got nil")
	}
}
