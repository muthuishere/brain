// Package storage defines a small, content-addressable object store abstraction
// (bytes/JSON blobs, key-prefix listing) so a brain's data can live on local disk
// or in S3-compatible object storage behind the same interface. It intentionally
// does not know about vectors/embeddings — that stays inside engine.Store.
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Backend is the storage seam: put/get bytes by key, check existence, list and
// delete by prefix. Both LocalFsBackend and S3Backend implement it identically.
type Backend interface {
	PutBytes(key string, data []byte) error
	GetBytes(key string) ([]byte, error)
	Exists(key string) bool
	ListPrefix(prefix string) ([]string, error)
	DeletePrefix(prefix string) error
}

// PutJSON marshals v and stores it at key.
func PutJSON(b Backend, key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.PutBytes(key, data)
}

// GetJSON reads the bytes at key and unmarshals them into v.
func GetJSON(b Backend, key string, v any) error {
	data, err := b.GetBytes(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// PutBlob content-addresses data under prefix by its sha256 hex digest and
// skips the write if that digest already exists (dedup). Returns the digest.
func PutBlob(b Backend, prefix string, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key := prefix + "/" + digest
	if b.Exists(key) {
		return digest, nil
	}
	if err := b.PutBytes(key, data); err != nil {
		return "", err
	}
	return digest, nil
}
