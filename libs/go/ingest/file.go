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

// SplitFrontmatter separates a leading YAML frontmatter block (a `---` fenced
// block at the very start of the content) from the body, returning the parsed
// top-level `key: value` pairs and the remaining body. If the content does not
// begin with a balanced `---` … `---` fence, meta is nil and body is data
// unchanged — so plain files are never altered. Nested/indented keys are
// skipped (only top-level scalar keys like name/description are captured), which
// is all the migration cue needs; this deliberately avoids a YAML dependency.
func SplitFrontmatter(data []byte) (meta map[string]string, body []byte) {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, data
	}
	rest := s[strings.IndexByte(s, '\n')+1:]
	end := regexp.MustCompile(`(?m)^---[ \t]*\r?\n?`).FindStringIndex(rest)
	if end == nil {
		return nil, data // no closing fence — treat as ordinary content
	}
	block := rest[:end[0]]
	body = []byte(strings.TrimLeft(rest[end[1]:], "\r\n"))
	meta = map[string]string{}
	kvRe := regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*):[ \t]+(.*)$`)
	for _, line := range strings.Split(block, "\n") {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			continue // indented → nested value, skip
		}
		if m := kvRe.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			meta[m[1]] = strings.Trim(strings.TrimSpace(m[2]), `"'`)
		}
	}
	return meta, body
}

// FrontmatterCue returns the best recall cue from parsed frontmatter:
// description first (a human summary of the lesson), then name. Empty if neither.
func FrontmatterCue(meta map[string]string) string {
	if d := strings.TrimSpace(meta["description"]); d != "" {
		return d
	}
	return strings.TrimSpace(meta["name"])
}

// ChunkFile reads a file, splits it into blocks on blank lines, and chunks
// each block via chunker.ChunkText(maxTokens, overlap).
func ChunkFile(path string, maxTokens, overlap int) ([]Chunk, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ChunkBytes(path, data, maxTokens, overlap), nil
}

// ChunkFileFM is the frontmatter-aware read path used by `record --from-file`:
// it strips a leading YAML frontmatter block before chunking the body and
// returns a recall cue (description/name) drawn from that frontmatter. For a
// file with no leading frontmatter, cue is "" and the chunks are identical to
// ChunkFile — plain files behave exactly as before.
func ChunkFileFM(path string, maxTokens, overlap int) (chunks []Chunk, cue string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	meta, body := SplitFrontmatter(data)
	cue = FrontmatterCue(meta)
	return ChunkBytes(path, body, maxTokens, overlap), cue, nil
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
