// Package ingest turns external content (local files, web pages) into
// brain-native chunks, ready to be recorded as engine.Episode text. Chunking
// and tokenizing are delegated to citenexus/golang's tested, frozen-contract
// packages rather than reimplemented here.
package ingest

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/muthuishere/citenexus/golang/chunker"
)

// Chunk is one chunked unit of a document, addressable by a parent-child id
// scheme (document::block::chunk) matching citenexus's evidence-builder shape.
type Chunk struct {
	ID         string
	Text       string
	BlockOrder int
	ChunkIndex int
}

var blankLineRe = regexp.MustCompile(`\n\s*\n`)

// ChunkFile reads a file, splits it into blocks on blank lines, and chunks
// each block via chunker.ChunkText(maxTokens, overlap).
func ChunkFile(path string, maxTokens, overlap int) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ChunkBytes(path, data, maxTokens, overlap), nil
}

// ChunkBytes splits data into blocks on blank lines and chunks each block via
// chunker.ChunkText(maxTokens, overlap), addressing chunks under docID —
// the same block/chunk splitting ChunkFile uses, but for content that didn't
// come from a local path (e.g. a fetched URL's body).
func ChunkBytes(docID string, data []byte, maxTokens, overlap int) []Chunk {
	blocks := blankLineRe.Split(string(data), -1)

	var chunks []Chunk
	for blockOrder, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		for chunkIndex, text := range chunker.ChunkText(block, maxTokens, overlap) {
			chunks = append(chunks, Chunk{
				ID:         fmt.Sprintf("%s::%d::%d", docID, blockOrder, chunkIndex),
				Text:       text,
				BlockOrder: blockOrder,
				ChunkIndex: chunkIndex,
			})
		}
	}
	return chunks
}
