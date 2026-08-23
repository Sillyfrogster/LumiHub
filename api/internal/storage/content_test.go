package storage

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
)

func newTestStore(t *testing.T) Store {
	t.Helper()
	store, err := NewStore(testdb.Connect(t), t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestPutComputesTheBlobDigestAndSize(t *testing.T) {
	store := newTestStore(t)

	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("abc")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	const wantDigest = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if hex.EncodeToString(stored.Digest[:]) != wantDigest {
		t.Errorf("digest = %x, want %s", stored.Digest, wantDigest)
	}
	if stored.ByteSize != 3 {
		t.Errorf("byte size = %d, want 3", stored.ByteSize)
	}

}

func TestPutMakesTheBlobReadableByTheByteServer(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(testdb.Connect(t), root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("served by nginx")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	encoded := hex.EncodeToString(stored.Digest[:])
	info, err := os.Stat(filepath.Join(root, "blobs", encoded[:2], encoded))
	if err != nil {
		t.Fatalf("stat blob: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("blob mode = %o, want 644", got)
	}
}

func TestConcurrentIdenticalWritesConvergeOnOneBlob(t *testing.T) {
	store := newTestStore(t)

	const writers = 8
	start := make(chan struct{})
	results := make(chan StoredBlob, writers)
	errors := make(chan error, writers)
	var ready sync.WaitGroup
	ready.Add(writers)
	for range writers {
		go func() {
			ready.Done()
			<-start
			stored, err := store.Put(context.Background(), bytes.NewReader([]byte("same bytes")))
			results <- stored
			errors <- err
		}()
	}
	ready.Wait()
	close(start)

	var first StoredBlob
	for range writers {
		if err := <-errors; err != nil {
			t.Fatalf("Put: %v", err)
		}
		stored := <-results
		if first.ID == [16]byte{} {
			first = stored
		} else if stored.ID != first.ID {
			t.Errorf("identical write returned blob %s, want %s", stored.ID, first.ID)
		}
	}

}

func TestReadRangeReturnsOnlyTheRequestedBytes(t *testing.T) {
	store := newTestStore(t)
	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("0123456789")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	part, err := store.ReadRange(context.Background(), stored.ID, 3, 4)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	defer part.Close()

	got, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read range: %v", err)
	}
	if string(got) != "3456" {
		t.Errorf("range = %q, want %q", got, "3456")
	}
}

func TestReadRangeRejectsEmptyAndOutOfBoundsRequests(t *testing.T) {
	store := newTestStore(t)
	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("0123456789")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	for _, test := range []struct {
		name           string
		offset, length int64
	}{
		{name: "zero length", offset: 3, length: 0},
		{name: "past end", offset: 8, length: 3},
		{name: "starts at end", offset: 10, length: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.ReadRange(context.Background(), stored.ID, test.offset, test.length)
			if !errors.Is(err, ErrInvalidRange) {
				t.Fatalf("ReadRange error = %v, want ErrInvalidRange", err)
			}
		})
	}
}

func TestOpenReturnsTheWholeBlob(t *testing.T) {
	store := newTestStore(t)
	want := []byte{0x00, 0xff, 0x50, 0x4e, 0x47, 0x80}
	stored, err := store.Put(context.Background(), bytes.NewReader(want))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	opened, err := store.Open(context.Background(), stored.ID)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer opened.Close()
	got, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("blob changed: got %x, want %x", got, want)
	}
}

func TestDerivativesAreDisposableWithoutTouchingSourceBlobs(t *testing.T) {
	store := newTestStore(t)
	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("source")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	derivative := DerivativeID{SourceDigest: stored.Digest, Variant: "detail", Version: 2}
	if err := store.PutDerivative(context.Background(), derivative, bytes.NewReader([]byte("rendered"))); err != nil {
		t.Fatalf("PutDerivative: %v", err)
	}

	rendered, err := store.OpenDerivative(context.Background(), derivative)
	if err != nil {
		t.Fatalf("OpenDerivative: %v", err)
	}
	got, err := io.ReadAll(rendered)
	rendered.Close()
	if err != nil {
		t.Fatalf("read derivative: %v", err)
	}
	if string(got) != "rendered" {
		t.Errorf("derivative = %q, want rendered", got)
	}

	if err := store.ClearDerivatives(context.Background()); err != nil {
		t.Fatalf("ClearDerivatives: %v", err)
	}
	if _, err := store.OpenDerivative(context.Background(), derivative); err == nil {
		t.Fatal("derivative still exists after clearing the cache")
	}
	source, err := store.Open(context.Background(), stored.ID)
	if err != nil {
		t.Fatalf("Open source after clearing derivatives: %v", err)
	}
	source.Close()
}

func TestDerivativeIdentityIncludesSourceVariantAndVersion(t *testing.T) {
	store := newTestStore(t)
	first, err := store.Put(context.Background(), bytes.NewReader([]byte("first source")))
	if err != nil {
		t.Fatalf("put first source: %v", err)
	}
	second, err := store.Put(context.Background(), bytes.NewReader([]byte("second source")))
	if err != nil {
		t.Fatalf("put second source: %v", err)
	}

	derivatives := []struct {
		id   DerivativeID
		want string
	}{
		{id: DerivativeID{SourceDigest: first.Digest, Variant: "grid", Version: 1}, want: "first grid v1"},
		{id: DerivativeID{SourceDigest: first.Digest, Variant: "grid", Version: 2}, want: "first grid v2"},
		{id: DerivativeID{SourceDigest: first.Digest, Variant: "detail", Version: 1}, want: "first detail v1"},
		{id: DerivativeID{SourceDigest: second.Digest, Variant: "grid", Version: 1}, want: "second grid v1"},
	}
	for _, derivative := range derivatives {
		if err := store.PutDerivative(context.Background(), derivative.id,
			bytes.NewReader([]byte(derivative.want))); err != nil {
			t.Fatalf("PutDerivative: %v", err)
		}
	}

	for _, derivative := range derivatives {
		opened, err := store.OpenDerivative(context.Background(), derivative.id)
		if err != nil {
			t.Fatalf("OpenDerivative: %v", err)
		}
		got, err := io.ReadAll(opened)
		opened.Close()
		if err != nil {
			t.Fatalf("read derivative: %v", err)
		}
		if string(got) != derivative.want {
			t.Errorf("derivative = %q, want %q", got, derivative.want)
		}
	}
}

func TestDerivativeCanBeHandedToTheInternalByteServer(t *testing.T) {
	store := newTestStore(t)
	stored, err := store.Put(context.Background(), bytes.NewReader([]byte("source")))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	id := DerivativeID{SourceDigest: stored.Digest, Variant: "grid", Version: 1}
	if err := store.PutDerivative(context.Background(), id, bytes.NewReader([]byte("rendered"))); err != nil {
		t.Fatalf("PutDerivative: %v", err)
	}

	redirect, err := store.InternalDerivativeRedirect(context.Background(), id)
	if err != nil {
		t.Fatalf("InternalDerivativeRedirect: %v", err)
	}
	if !strings.HasPrefix(redirect, "/_illarin/derivatives/") {
		t.Fatalf("redirect = %q, want internal derivative location", redirect)
	}

	if err := store.ClearDerivatives(context.Background()); err != nil {
		t.Fatalf("ClearDerivatives: %v", err)
	}
	_, err = store.InternalDerivativeRedirect(context.Background(), id)
	if !errors.Is(err, ErrDerivativeNotFound) {
		t.Fatalf("missing derivative error = %v, want ErrDerivativeNotFound", err)
	}
}
