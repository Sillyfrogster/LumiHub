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
		File: bytes.NewReader(file), Name: "Long", Discovery: "listed",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var hash []byte
	var blobID uuid.UUID
	var size int64
	err = pool.QueryRow(context.Background(),
		`select b.sha256, b.byte_size, r.blob_id
		   from asset_revisions r
		   join blobs b on b.id = r.blob_id
		  where r.id = $1`,
		got.CurrentRevisionID).Scan(&hash, &size, &blobID)
	if err != nil {
		t.Fatalf("read revision: %v", err)
	}

	want := sha256.Sum256(file)
	if hex.EncodeToString(hash) != hex.EncodeToString(want[:]) {
		t.Errorf("blob digest = %x, want the hash of every byte sent", hash)
	}
	if size != int64(len(file)) {
		t.Errorf("byte_size = %d, want %d", size, len(file))
	}

	stored, err := svc.store.Open(context.Background(), blobID)
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
