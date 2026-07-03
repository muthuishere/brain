package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/muthuishere/brain/engine"
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

type endpointCfg struct {
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`
}

type endpointsFile struct {
	Embedding *endpointCfg `json:"embedding"`
	Reranker  *endpointCfg `json:"reranker"`
	LLM       *endpointCfg `json:"llm"`
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

func envOverride(cur *endpointCfg, urlVar, modelVar, keyEnvVar string) *endpointCfg {
	url := os.Getenv(urlVar)
	if url == "" && cur == nil {
		return nil
	}
	if cur == nil {
		cur = &endpointCfg{}
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

// apiKey resolves the key from the NAMED env var (never stored in config).
func (c endpointCfg) apiKey() string {
	if c.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(c.APIKeyEnv)
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

func postJSON(url, apiKey string, body any, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("endpoint %s returned %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// --- embedding (OpenAI-compatible /embeddings) ------------------------------

type httpEmbedding struct{ cfg endpointCfg }

func (e httpEmbedding) Embed(text string) []float64 {
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	body := map[string]any{"model": e.cfg.Model, "input": text}
	if err := postJSON(e.cfg.BaseURL+"/embeddings", e.cfg.apiKey(), body, &out); err != nil {
		fatal("embedding endpoint: %v", err)
	}
	if len(out.Data) == 0 {
		fatal("embedding endpoint: empty response")
	}
	return out.Data[0].Embedding
}

// --- reranker (Jina/Cohere-style /rerank) -----------------------------------

type httpReranker struct{ cfg endpointCfg }

func (r httpReranker) Rerank(query string, cands []engine.Recalled) []engine.Recalled {
	docs := make([]string, len(cands))
	for i, c := range cands {
		docs[i] = c.Episode.Text
	}
	var out struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	body := map[string]any{"model": r.cfg.Model, "query": query, "documents": docs}
	if err := postJSON(r.cfg.BaseURL+"/rerank", r.cfg.apiKey(), body, &out); err != nil {
		fatal("reranker endpoint: %v", err)
	}
	if len(out.Results) == 0 {
		return cands // nothing to reorder by; keep score order
	}
	sort.SliceStable(out.Results, func(i, j int) bool {
		return out.Results[i].RelevanceScore > out.Results[j].RelevanceScore
	})
	reordered := make([]engine.Recalled, 0, len(cands))
	for _, res := range out.Results {
		if res.Index >= 0 && res.Index < len(cands) {
			reordered = append(reordered, cands[res.Index])
		}
	}
	return reordered
}

// --- LLM (OpenAI-compatible /chat/completions) ------------------------------

type httpLLM struct{ cfg endpointCfg }

func (l httpLLM) Answer(question, passage string) string {
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	body := map[string]any{
		"model":       l.cfg.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": "Answer only from the passage. If it does not support an answer, say you cannot."},
			{"role": "user", "content": "Question: " + question + "\n\nPassage:\n" + passage},
		},
	}
	if err := postJSON(l.cfg.BaseURL+"/chat/completions", l.cfg.apiKey(), body, &out); err != nil {
		fatal("llm endpoint: %v", err)
	}
	if len(out.Choices) == 0 {
		return ""
	}
	return out.Choices[0].Message.Content
}

// buildModels wires the configured endpoints into engine seams. Any nil config
// falls back to offline behavior (hashing embedder / no reranker / no LLM).
func buildModels(ep endpointsFile) (engine.Embedder, engine.LLM, engine.Reranker) {
	var embedder engine.Embedder = engine.HashEmbedding{}
	if ep.Embedding != nil {
		embedder = httpEmbedding{cfg: *ep.Embedding}
	}
	var llm engine.LLM
	if ep.LLM != nil {
		llm = httpLLM{cfg: *ep.LLM}
	}
	var reranker engine.Reranker
	if ep.Reranker != nil {
		reranker = httpReranker{cfg: *ep.Reranker}
	}
	return embedder, llm, reranker
}
