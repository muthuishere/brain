package engine

import (
	"crypto/sha1"
	_ "embed"
	"encoding/json"
	"math"
	"regexp"
	"strings"
)

// The pinned CiteNexus tokenizer + faithfulness/relevance gates + a deterministic
// hash embedder, vendored self-contained so the CLI needs no external module and
// `go install` just works. Parity with the Python reference (citenexus).

//go:embed stopwords.json
var stopwordsJSON []byte

var (
	tokenRE   = regexp.MustCompile(`[a-z0-9]+`)
	stopwords = loadStopwords()
)

func loadStopwords() map[string]struct{} {
	var words []string
	if err := json.Unmarshal(stopwordsJSON, &words); err != nil {
		panic("engine: stopwords.json: " + err.Error())
	}
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

// Tokenize lowercases text and returns every [a-z0-9]+ run, in order.
func Tokenize(text string) []string {
	return tokenRE.FindAllString(strings.ToLower(text), -1)
}

// ContentTokens is tokenize(text) minus the pinned stopword set.
func ContentTokens(text string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, tok := range Tokenize(text) {
		if _, isStop := stopwords[tok]; isStop {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

// HasRelevanceOverlap is true when question and passage share a content token.
func HasRelevanceOverlap(question, passage string) bool {
	p := ContentTokens(passage)
	for tok := range ContentTokens(question) {
		if _, ok := p[tok]; ok {
			return true
		}
	}
	return false
}

// IsSupported is true when the answer is non-empty and every answer token
// appears in the passage — the extractive faithfulness gate.
func IsSupported(answer, passage string) bool {
	answerToks := Tokenize(answer)
	if len(answerToks) == 0 {
		return false
	}
	passageSet := make(map[string]struct{})
	for _, tok := range Tokenize(passage) {
		passageSet[tok] = struct{}{}
	}
	for _, tok := range answerToks {
		if _, ok := passageSet[tok]; !ok {
			return false
		}
	}
	return true
}

// Dim is the embedding dimensionality.
const Dim = 64

// HashEmbedding is a deterministic hash-bucketed bag-of-tokens embedder — offline,
// no endpoint. Good enough for keyword-ish recall over an agent's own logs.
type HashEmbedding struct{}

// Embed maps text to an L2-normalized Dim-vector.
func (HashEmbedding) Embed(text string) []float64 {
	vec := make([]float64, Dim)
	for _, tok := range Tokenize(text) {
		sum := sha1.Sum([]byte(tok))
		idx := int(sum[len(sum)-1] & 0x3F)
		vec[idx] += 1.0
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm != 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

// Cosine is the dot product of two vectors (unit vectors → cosine similarity).
func Cosine(a, b []float64) float64 {
	var dot float64
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	return dot
}
