package storage

import (
	"testing"
)

func TestLocalFsBackendRoundTrip(t *testing.T) {
	b := NewLocalFsBackend(t.TempDir())

	if b.Exists("a/b.txt") {
		t.Fatal("expected key to not exist yet")
	}
	if err := b.PutBytes("a/b.txt", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if !b.Exists("a/b.txt") {
		t.Fatal("expected key to exist after put")
	}
	got, err := b.GetBytes("a/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}

	if err := b.PutBytes("a/c.txt", []byte("world")); err != nil {
		t.Fatal(err)
	}
	keys, err := b.ListPrefix("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %v", keys)
	}

	if err := b.DeletePrefix("a"); err != nil {
		t.Fatal(err)
	}
	if b.Exists("a/b.txt") {
		t.Fatal("expected key to be gone after DeletePrefix")
	}
}

func TestPutBlobDedup(t *testing.T) {
	b := NewLocalFsBackend(t.TempDir())
	d1, err := PutBlob(b, "raw", []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	d2, err := PutBlob(b, "raw", []byte("same content"))
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("expected identical digests, got %q and %q", d1, d2)
	}
}

func TestPutGetJSON(t *testing.T) {
	b := NewLocalFsBackend(t.TempDir())
	type payload struct{ Name string }
	if err := PutJSON(b, "p.json", payload{Name: "brain"}); err != nil {
		t.Fatal(err)
	}
	var out payload
	if err := GetJSON(b, "p.json", &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != "brain" {
		t.Fatalf("got %+v", out)
	}
}
