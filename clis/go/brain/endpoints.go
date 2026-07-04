package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/muthuishere/brain/libs/go/engine"
	"github.com/muthuishere/brain/libs/go/modelclients"
)

// Optional model endpoints for the CLI. All are opt-in: with no endpoints.json
// (and no env override) the brain runs fully offline — a deterministic hashing
// embedder, no reranker, no LLM. Configure any subset to upgrade recall.
//
// endpoints.json (in the brain repo folder):
//
//	{
//	  "embedding": {"base_url": "http://localhost:11434/v1", "model": "bge-m3", "api_key_env": "OLLAMA_API_KEY"},
//	  "reranker":  {"base_url": "http://localhost:11434",    "model": "bge-reranker-v2-m3", "api_key_env": ""},
//	  "llm":       {"base_url": "http://localhost:11434/v1", "model": "qwen2.5", "api_key_env": ""}
//	}
//
// api_key_env NAMES an environment variable; the CLI reads the key from it at
// runtime and never stores or prints the value.

type endpointsFile struct {
	Embedding *modelclients.EndpointCfg `json:"embedding"`
	Reranker  *modelclients.EndpointCfg `json:"reranker"`
	LLM       *modelclients.EndpointCfg `json:"llm"`
}

func loadEndpoints(repo string) endpointsFile {
	var ep endpointsFile
	if data, err := os.ReadFile(filepath.Join(repo, "endpoints.json")); err == nil {
		if err := json.Unmarshal(data, &ep); err != nil {
			fatal("endpoints.json: %v", err)
		}
	}
	// Env overrides (handy for the agent skill / CI without editing the repo).
	ep.Embedding = envOverride(ep.Embedding, "BRAIN_EMBED_URL", "BRAIN_EMBED_MODEL", "BRAIN_EMBED_KEY_ENV")
	ep.Reranker = envOverride(ep.Reranker, "BRAIN_RERANK_URL", "BRAIN_RERANK_MODEL", "BRAIN_RERANK_KEY_ENV")
	ep.LLM = envOverride(ep.LLM, "BRAIN_LLM_URL", "BRAIN_LLM_MODEL", "BRAIN_LLM_KEY_ENV")
	return ep
}

func envOverride(cur *modelclients.EndpointCfg, urlVar, modelVar, keyEnvVar string) *modelclients.EndpointCfg {
	url := os.Getenv(urlVar)
	if url == "" && cur == nil {
		return nil
	}
	if cur == nil {
		cur = &modelclients.EndpointCfg{}
	}
	if url != "" {
		cur.BaseURL = url
	}
	if m := os.Getenv(modelVar); m != "" {
		cur.Model = m
	}
	if k := os.Getenv(keyEnvVar); k != "" {
		cur.APIKeyEnv = k
	}
	if cur.BaseURL == "" {
		return nil
	}
	return cur
}

// buildModels wires the configured endpoints into engine seams. Any nil config
// falls back to offline behavior (hashing embedder / no reranker / no LLM).
func buildModels(ep endpointsFile) (engine.Embedder, engine.LLM, engine.Reranker) {
	var embedder engine.Embedder = engine.HashEmbedding{}
	if ep.Embedding != nil {
		embedder = modelclients.HTTPEmbedding{Cfg: *ep.Embedding}
	}
	var llm engine.LLM
	if ep.LLM != nil {
		llm = modelclients.HTTPLLM{Cfg: *ep.LLM}
	}
	var reranker engine.Reranker
	if ep.Reranker != nil {
		reranker = modelclients.HTTPReranker{Cfg: *ep.Reranker}
	}
	return embedder, llm, reranker
}
