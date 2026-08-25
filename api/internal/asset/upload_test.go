package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/google/uuid"
)

// fileLongerThanTheDetectionPeek is what makes this test worth having. The
// first bytes are read twice, once to recognise the file and once to store it,
// and only a file longer than that peek can tell the two apart.
func fileLongerThanTheDetectionPeek() []byte {
	file := make([]byte, detectHeadBytes*4)
	for i := range file {
		file[i] = byte(i)
	}
	return file
}

func TestCreateRecordsTheWholeFileItStored(t *testing.T) {
	svc, pool := newTestService(t)
	file := fileLongerThanTheDetectionPeek()

	got, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "long.bin",
		File: bytes.NewReader(file), Name: "Long", Publication: "public",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var hash, storageKey string
	var size int64
	err = pool.QueryRow(context.Background(),
		`select content_hash, byte_size, storage_key from asset_revisions where id = $1`,
		got.CurrentRevisionID).Scan(&hash, &size, &storageKey)
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}

	want := sha256.Sum256(file)
	if hash != hex.EncodeToString(want[:]) {
		t.Errorf("content_hash = %s, want the hash of every byte sent", hash)
	}
	if size != int64(len(file)) {
		t.Errorf("byte_size = %d, want %d", size, len(file))
	}

	stored, err := svc.blob.Get(context.Background(), storageKey)
	if err != nil {
		t.Fatalf("open stored file: %v", err)
	}
	defer stored.Close()

	onDisk, err := io.ReadAll(stored)
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if !bytes.Equal(onDisk, file) {
		t.Errorf("stored %d bytes, sent %d, and they differ", len(onDisk), len(file))
	}
}
