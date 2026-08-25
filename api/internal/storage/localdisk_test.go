package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestRoundTripReturnsExactBytes(t *testing.T) {
	blob, err := NewLocalDisk(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalDisk: %v", err)
	}

	// A byte sequence that is not valid UTF-8, so nothing can quietly re-encode it.
	original := []byte{0x00, 0xff, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x80}

	if err := blob.Put(context.Background(), "revisions/abc/1", bytes.NewReader(original)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rc, err := blob.Get(context.Background(), "revisions/abc/1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("bytes changed: got %x, want %x", got, original)
	}
}

func TestGetMissingKeyReturnsAnError(t *testing.T) {
	blob, _ := NewLocalDisk(t.TempDir())
	if _, err := blob.Get(context.Background(), "revisions/nope/1"); err == nil {
		t.Fatal("expected an error reading a key that was never written")
	}
}

func TestKeysCannotEscapeTheRoot(t *testing.T) {
	blob, _ := NewLocalDisk(t.TempDir())
	err := blob.Put(context.Background(), "../escaped", bytes.NewReader([]byte("x")))
	if err == nil {
		t.Fatal("expected a key containing .. to be rejected")
	}
}
