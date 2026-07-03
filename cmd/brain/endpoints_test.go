package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/muthuishere/brain/engine"
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
		if req["input"] != "hello" {
			t.Errorf("input = %v", req["input"])
		}
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer srv.Close()
	t.Setenv("EMB_KEY", "secret-123")

	e := httpEmbedding{cfg: endpointCfg{BaseURL: srv.URL, Model: "m", APIKeyEnv: "EMB_KEY"}}
	got := e.Embed("hello")
	if len(got) != 3 || got[0] != 0.1 {
		t.Fatalf("embedding = %v", got)
	}
}

func TestHTTPReranker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Rank the second document first.
		_, _ = io.WriteString(w, `{"results":[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.2}]}`)
	}))
	defer srv.Close()

	r := httpReranker{cfg: endpointCfg{BaseURL: srv.URL, Model: "m"}}
	cands := []engine.Recalled{
		{Episode: engine.Episode{ID: "a", Text: "first"}},
		{Episode: engine.Episode{ID: "b", Text: "second"}},
	}
	out := r.Rerank("q", cands)
	if len(out) != 2 || out[0].Episode.ID != "b" {
		t.Fatalf("rerank order = %v", out)
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

	l := httpLLM{cfg: endpointCfg{BaseURL: srv.URL, Model: "m"}}
	if got := l.Answer("q?", "the passage"); got != "grounded answer" {
		t.Fatalf("answer = %q", got)
	}
}

func TestBuildModelsOfflineByDefault(t *testing.T) {
	// No config → offline: hashing embedder, no reranker, no LLM.
	emb, llm, rr := buildModels(endpointsFile{})
	if _, ok := emb.(engine.HashEmbedding); !ok {
		t.Fatalf("default embedder should be HashEmbedding, got %T", emb)
	}
	if llm != nil || rr != nil {
		t.Fatalf("offline: llm and reranker should be nil")
	}
}

func TestBuildModelsWithEndpoints(t *testing.T) {
	emb, llm, rr := buildModels(endpointsFile{
		Embedding: &endpointCfg{BaseURL: "http://x/v1", Model: "e"},
		Reranker:  &endpointCfg{BaseURL: "http://x", Model: "r"},
		LLM:       &endpointCfg{BaseURL: "http://x/v1", Model: "l"},
	})
	if _, ok := emb.(httpEmbedding); !ok {
		t.Fatalf("configured embedder should be httpEmbedding, got %T", emb)
	}
	if llm == nil || rr == nil {
		t.Fatalf("configured llm/reranker should be non-nil")
	}
}
