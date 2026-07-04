package main

import (
	"testing"

	"github.com/muthuishere/brain/libs/go/engine"
	"github.com/muthuishere/brain/libs/go/modelclients"
)

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
		Embedding: &modelclients.EndpointCfg{BaseURL: "http://x/v1", Model: "e"},
		Reranker:  &modelclients.EndpointCfg{BaseURL: "http://x", Model: "r"},
		LLM:       &modelclients.EndpointCfg{BaseURL: "http://x/v1", Model: "l"},
	})
	if _, ok := emb.(modelclients.HTTPEmbedding); !ok {
		t.Fatalf("configured embedder should be modelclients.HTTPEmbedding, got %T", emb)
	}
	if llm == nil || rr == nil {
		t.Fatalf("configured llm/reranker should be non-nil")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("TEST_URL", "http://from-env/v1")
	t.Setenv("TEST_MODEL", "env-model")
	got := envOverride(nil, "TEST_URL", "TEST_MODEL", "TEST_KEY_ENV")
	if got == nil || got.BaseURL != "http://from-env/v1" || got.Model != "env-model" {
		t.Fatalf("envOverride = %+v", got)
	}
}

func TestEnvOverrideNilWhenNoEnvAndNoExisting(t *testing.T) {
	if got := envOverride(nil, "UNSET_URL_VAR", "UNSET_MODEL_VAR", "UNSET_KEY_VAR"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
