// Package modelclients provides optional HTTP-backed implementations of the
// engine's Embedder/LLM/Reranker seams (OpenAI/Jina/Cohere-compatible). All
// are opt-in: with no EndpointCfg configured, callers should fall back to
// engine.HashEmbedding{} / nil LLM / nil Reranker for fully offline operation.
package modelclients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/muthuishere/brain/libs/go/engine"
)

// EndpointCfg configures one HTTP model endpoint. APIKeyEnv NAMES an
// environment variable — the key is read from it at call time and never
// stored or logged, matching the S3 credential-by-env-name convention.
type EndpointCfg struct {
	BaseURL   string            `json:"base_url"`
	Model     string            `json:"model"`
	APIKeyEnv string            `json:"api_key_env"`
	Headers   map[string]string `json:"headers,omitempty"`
	AuthHeader string           `json:"auth_header,omitempty"` // default "Authorization"
	AuthScheme *string          `json:"auth_scheme,omitempty"` // default "Bearer"; pass "" for schemes with no prefix (e.g. Anthropic's x-api-key)
	TimeoutS  float64           `json:"timeout_s,omitempty"`   // default 60
	BatchSize int               `json:"batch_size,omitempty"`  // default 32, embeddings only
}

func (c EndpointCfg) apiKey() string {
	if c.APIKeyEnv == "" {
		return ""
	}
	return os.Getenv(c.APIKeyEnv)
}

func (c EndpointCfg) timeout() time.Duration {
	if c.TimeoutS <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.TimeoutS * float64(time.Second))
}

func (c EndpointCfg) authHeader() string {
	if c.AuthHeader != "" {
		return c.AuthHeader
	}
	return "Authorization"
}

func (c EndpointCfg) authValue(key string) string {
	scheme := "Bearer"
	if c.AuthScheme != nil {
		scheme = *c.AuthScheme
	}
	if scheme == "" {
		return key
	}
	return scheme + " " + key
}

func postJSON(cfg EndpointCfg, url string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	if key := cfg.apiKey(); key != "" {
		req.Header.Set(cfg.authHeader(), cfg.authValue(key))
	}
	client := &http.Client{Timeout: cfg.timeout()}
	resp, err := client.Do(req)
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

type HTTPEmbedding struct{ Cfg EndpointCfg }

func (e HTTPEmbedding) embedBatch(texts []string) ([][]float64, error) {
	var out struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	body := map[string]any{"model": e.Cfg.Model, "input": texts}
	if err := postJSON(e.Cfg, e.Cfg.BaseURL+"/embeddings", body, &out); err != nil {
		return nil, err
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding endpoint: expected %d embeddings, got %d", len(texts), len(out.Data))
	}
	vecs := make([][]float64, len(out.Data))
	for i, d := range out.Data {
		vecs[i] = d.Embedding
	}
	return vecs, nil
}

// Embed implements engine.Embedder for a single string.
func (e HTTPEmbedding) Embed(text string) []float64 {
	vecs, err := e.embedBatch([]string{text})
	if err != nil {
		log.Fatalf("embedding endpoint: %v", err)
	}
	return vecs[0]
}

// EmbedMany embeds texts in batches of Cfg.BatchSize (default 32), order-preserving.
func (e HTTPEmbedding) EmbedMany(texts []string) [][]float64 {
	batchSize := e.Cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 32
	}
	result := make([][]float64, 0, len(texts))
	for i := 0; i < len(texts); i += batchSize {
		end := min(i+batchSize, len(texts))
		vecs, err := e.embedBatch(texts[i:end])
		if err != nil {
			log.Fatalf("embedding endpoint: %v", err)
		}
		result = append(result, vecs...)
	}
	return result
}

// --- reranker (Jina/Cohere-style /rerank) -----------------------------------

type HTTPReranker struct{ Cfg EndpointCfg }

// Rerank implements engine.Reranker. Any candidate the endpoint omits from
// its results is appended in its original relative order, rather than
// silently dropped — matching citenexus's rerank fallback safety net.
func (r HTTPReranker) Rerank(query string, cands []engine.Recalled) []engine.Recalled {
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
	body := map[string]any{"model": r.Cfg.Model, "query": query, "documents": docs}
	if err := postJSON(r.Cfg, r.Cfg.BaseURL+"/rerank", body, &out); err != nil {
		log.Fatalf("reranker endpoint: %v", err)
	}
	if len(out.Results) == 0 {
		return cands // nothing to reorder by; keep score order
	}
	sort.SliceStable(out.Results, func(i, j int) bool {
		return out.Results[i].RelevanceScore > out.Results[j].RelevanceScore
	})
	seen := make([]bool, len(cands))
	reordered := make([]engine.Recalled, 0, len(cands))
	for _, res := range out.Results {
		if res.Index >= 0 && res.Index < len(cands) {
			reordered = append(reordered, cands[res.Index])
			seen[res.Index] = true
		}
	}
	for i, wasSeen := range seen {
		if !wasSeen {
			reordered = append(reordered, cands[i])
		}
	}
	return reordered
}

// --- LLM (OpenAI-compatible /chat/completions, Anthropic-compatible via AuthScheme/AuthHeader) ---

type HTTPLLM struct{ Cfg EndpointCfg }

func (l HTTPLLM) Answer(question, passage string) string {
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	body := map[string]any{
		"model":       l.Cfg.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": "Answer only from the passage. If it does not support an answer, say you cannot."},
			{"role": "user", "content": "Question: " + question + "\n\nPassage:\n" + passage},
		},
	}
	if err := postJSON(l.Cfg, l.Cfg.BaseURL+"/chat/completions", body, &out); err != nil {
		log.Fatalf("llm endpoint: %v", err)
	}
	if len(out.Choices) == 0 {
		return ""
	}
	return out.Choices[0].Message.Content
}
