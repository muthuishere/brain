package engine

import "github.com/muthuishere/citenexus/golang/fakes"

// HashEmbedding is the deterministic offline embedder (the shared CiteNexus fake).
// Aliased so callers keep using engine.HashEmbedding while the impl lives in the
// published citenexus/golang/fakes package.
type HashEmbedding = fakes.FakeEmbedding
