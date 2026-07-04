package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// LocalFsBackend implements Backend over a local directory tree.
type LocalFsBackend struct {
	Root string
}

func NewLocalFsBackend(root string) *LocalFsBackend {
	return &LocalFsBackend{Root: root}
}

func (l *LocalFsBackend) path(key string) string {
	return filepath.Join(l.Root, filepath.FromSlash(key))
}

func (l *LocalFsBackend) PutBytes(key string, data []byte) error {
	p := l.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func (l *LocalFsBackend) GetBytes(key string) ([]byte, error) {
	return os.ReadFile(l.path(key))
}

func (l *LocalFsBackend) Exists(key string) bool {
	_, err := os.Stat(l.path(key))
	return err == nil
}

func (l *LocalFsBackend) ListPrefix(prefix string) ([]string, error) {
	base := l.path(prefix)
	var keys []string
	root := base
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return keys, nil
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(l.Root, p)
		if err != nil {
			return err
		}
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	return keys, err
}

func (l *LocalFsBackend) DeletePrefix(prefix string) error {
	p := l.path(prefix)
	if !strings.HasPrefix(filepath.Clean(p), filepath.Clean(l.Root)) {
		return os.ErrInvalid
	}
	return os.RemoveAll(p)
}
