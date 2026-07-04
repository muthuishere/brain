package ingest

import (
	"net/http"
	"net/http/httptest"
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
