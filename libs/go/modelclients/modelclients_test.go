package modelclients

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/muthuishere/brain/libs/go/engine"
)

func TestHTTPEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %s, want /embeddings", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-123" {
			t.Errorf("auth header = %q", got)
		}
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer srv.Close()
	t.Setenv("EMB_KEY", "secret-123")

	e := HTTPEmbedding{Cfg: EndpointCfg{BaseURL: srv.URL, Model: "m", APIKeyEnv: "EMB_KEY"}}
	got := e.Embed("hello")
	if len(got) != 3 || got[0] != 0.1 {
		t.Fatalf("embedding = %v", got)
	}
}

func TestHTTPEmbeddingCustomAuthHeaderAndScheme(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret-123" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("X-Custom"); got != "yes" {
			t.Errorf("X-Custom = %q", got)
		}
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.1]}]}`)
	}))
	defer srv.Close()
	t.Setenv("EMB_KEY", "secret-123")

	empty := ""
	e := HTTPEmbedding{Cfg: EndpointCfg{
		BaseURL: srv.URL, Model: "m", APIKeyEnv: "EMB_KEY",
		AuthHeader: "x-api-key", AuthScheme: &empty,
		Headers: map[string]string{"X-Custom": "yes"},
	}}
	e.Embed("hello")
}

func TestHTTPEmbeddingEmbedManyBatches(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		resp := `{"data":[`
		for i := range req.Input {
			if i > 0 {
				resp += ","
			}
			resp += `{"embedding":[1]}`
		}
		resp += `]}`
		_, _ = io.WriteString(w, resp)
	}))
	defer srv.Close()

	e := HTTPEmbedding{Cfg: EndpointCfg{BaseURL: srv.URL, Model: "m", BatchSize: 2}}
	got := e.EmbedMany([]string{"a", "b", "c", "d", "e"})
	if len(got) != 5 {
		t.Fatalf("expected 5 embeddings, got %d", len(got))
	}
	if calls != 3 {
		t.Fatalf("expected 3 batched calls (2+2+1), got %d", calls)
	}
}

func TestHTTPReranker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rank the second document first.
		_, _ = io.WriteString(w, `{"results":[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.2}]}`)
	}))
	defer srv.Close()

	r := HTTPReranker{Cfg: EndpointCfg{BaseURL: srv.URL, Model: "m"}}
	cands := []engine.Recalled{
		{Episode: engine.Episode{ID: "a", Text: "first"}},
		{Episode: engine.Episode{ID: "b", Text: "second"}},
	}
	out := r.Rerank("q", cands)
	if len(out) != 2 || out[0].Episode.ID != "b" {
		t.Fatalf("rerank order = %v", out)
	}
}

func TestHTTPRerankerAppendsOmittedCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Endpoint only ranks index 1, omitting 0 and 2.
		_, _ = io.WriteString(w, `{"results":[{"index":1,"relevance_score":0.9}]}`)
	}))
	defer srv.Close()

	r := HTTPReranker{Cfg: EndpointCfg{BaseURL: srv.URL, Model: "m"}}
	cands := []engine.Recalled{
		{Episode: engine.Episode{ID: "a"}},
		{Episode: engine.Episode{ID: "b"}},
		{Episode: engine.Episode{ID: "c"}},
	}
	out := r.Rerank("q", cands)
	if len(out) != 3 {
		t.Fatalf("expected all 3 candidates preserved, got %d", len(out))
	}
	if out[0].Episode.ID != "b" {
		t.Fatalf("expected ranked candidate first, got %s", out[0].Episode.ID)
	}
	if out[1].Episode.ID != "a" || out[2].Episode.ID != "c" {
		t.Fatalf("expected omitted candidates appended in original order, got %v", out)
	}
}

func TestHTTPLLM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "Passage:") {
			t.Errorf("user message missing passage: %s", body)
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"grounded answer"}}]}`)
	}))
	defer srv.Close()

	l := HTTPLLM{Cfg: EndpointCfg{BaseURL: srv.URL, Model: "m"}}
	if got := l.Answer("q?", "the passage"); got != "grounded answer" {
		t.Fatalf("answer = %q", got)
	}
}
