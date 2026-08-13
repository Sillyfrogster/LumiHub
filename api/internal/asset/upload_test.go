package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"io"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

func patternedFile() []byte {
	file := make([]byte, 128*1024)
	for i := range file {
		file[i] = byte(i)
	}
	return file
}

func TestCreateRecordsTheWholeFileItStored(t *testing.T) {
	svc, pool := newTestService(t)
	file := patternedFile()

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

type ccv2ClaimModule struct{}

func (ccv2ClaimModule) ID() string { return "chara_card_v2" }
func (ccv2ClaimModule) Claim(file probe.Result) (format.Claim, bool) {
	for _, payload := range file.Payloads {
		if spec, ok := payload.String("spec"); ok && spec == "chara_card_v2" {
			return format.Claim{PayloadID: payload.ID, Strength: format.Authoritative}, true
		}
	}
	return format.Claim{}, false
}
func (ccv2ClaimModule) Parse(context.Context, probe.Result, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: "character", Format: "chara_card_v2"}, nil
}

func TestCreateClaimsAPayloadBeyondTheOldHeadPeek(t *testing.T) {
	service, _ := newTestServiceWithRegistry(t, registryWithModule(t, ccv2ClaimModule{}))
	payload := base64.StdEncoding.EncodeToString([]byte(`{"spec":"chara_card_v2"}`))
	file := png(
		chunk("IHDR", make([]byte, 13)),
		chunk("IDAT", make([]byte, 1024)),
		chunk("tEXt", append([]byte("ccv3\x00"), payload...)),
		chunk("IEND", nil),
	)

	got, err := service.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "theme", Filename: "card.png",
		File: bytes.NewReader(file), Name: "Late card",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Kind != "character" || got.Format != "chara_card_v2" {
		t.Fatalf("created kind and format = %q, %q; want character, chara_card_v2", got.Kind, got.Format)
	}
}

func png(chunks ...[]byte) []byte {
	file := append([]byte(nil), []byte("\x89PNG\r\n\x1a\n")...)
	for _, chunk := range chunks {
		file = append(file, chunk...)
	}
	return file
}

func chunk(kind string, data []byte) []byte {
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(data)))
	chunk.WriteString(kind)
	chunk.Write(data)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
	return chunk.Bytes()
}
