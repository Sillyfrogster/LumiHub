package probe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type recordingStore struct {
	data  []byte
	reads []byteRange
}

func TestInspectStreamsAJSONRootThroughRangeReads(t *testing.T) {
	file := []byte(`{"padding":"` + strings.Repeat("x", 128*1024) + `","spec":"chara_card_v3"}`)
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "card.json")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.Container != JSON {
		t.Fatalf("container = %q, want JSON", got.Container)
	}
	if len(got.Payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(got.Payloads))
	}
	if spec, ok := got.Payloads[0].String("spec"); !ok || spec != "chara_card_v3" {
		t.Errorf("spec = %q, %v; want chara_card_v3, true", spec, ok)
	}
	for _, read := range store.reads {
		if read.length > maxRangeRead {
			t.Fatalf("range read length = %d, want at most %d", read.length, maxRangeRead)
		}
	}
}

func TestJSONFilenameDoesNotTurnOpaqueBytesIntoJSON(t *testing.T) {
	file := []byte{0x00, 0xff, 0xfe, 0x10}
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "misleading.json")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.Container != Unknown {
		t.Fatalf("container = %q, want unknown", got.Container)
	}
}

func TestJSONFilenameAllowsWhitespaceBeyondTheSignatureRead(t *testing.T) {
	file := []byte(strings.Repeat(" ", 32) + `{"spec":"chara_card_v3"}`)
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "card.json")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.Container != JSON {
		t.Fatalf("container = %q, want JSON", got.Container)
	}
}

func TestInspectTellsRootZIPEntriesApart(t *testing.T) {
	for _, test := range []struct {
		name      string
		entryName string
		body      string
		payloads  int
	}{
		{name: "theme", entryName: "theme.json", body: `{"name":"Midnight"}`, payloads: 0},
		{name: "character", entryName: "card.json", body: `{"spec":"chara_card_v3"}`, payloads: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := zipFile(t, test.entryName, test.body)
			store := &recordingStore{data: file}

			got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "bundle.zip")
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if got.Container != ZIP {
				t.Fatalf("container = %q, want ZIP", got.Container)
			}
			if len(got.ZIPEntries) != 1 || got.ZIPEntries[0].Name != test.entryName {
				t.Fatalf("entries = %+v, want one root %s", got.ZIPEntries, test.entryName)
			}
			if len(got.Payloads) != test.payloads {
				t.Fatalf("payload count = %d, want %d", len(got.Payloads), test.payloads)
			}
			if test.payloads == 1 && got.Payloads[0].Locator.Name != test.entryName {
				t.Fatalf("payloads = %+v, want decoded %s", got.Payloads, test.entryName)
			}
		})
	}
}

type byteRange struct {
	offset int64
	length int64
}

func (s *recordingStore) ReadRange(_ context.Context, _ uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	s.reads = append(s.reads, byteRange{offset: offset, length: length})
	return io.NopCloser(bytes.NewReader(s.data[offset : offset+length])), nil
}

func TestInspectFindsAJSONPayloadInALatePNGChunk(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString([]byte(`{"spec":"chara_card_v2","name":"Late card"}`))
	file := pngFile(
		pngChunk("IHDR", make([]byte, 13)),
		pngChunk("IDAT", make([]byte, 1024)),
		pngChunk("tEXt", append([]byte("ccv3\x00"), payload...)),
		pngChunk("IEND", nil),
	)
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "misleading.json")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if got.Container != PNG {
		t.Fatalf("container = %q, want PNG", got.Container)
	}
	if len(got.Payloads) != 1 {
		t.Fatalf("payload count = %d, want 1", len(got.Payloads))
	}
	if got.Payloads[0].Locator.Name != "ccv3" {
		t.Errorf("payload locator = %q, want ccv3", got.Payloads[0].Locator.Name)
	}
	if got.Payloads[0].Locator.Offset <= 512 {
		t.Errorf("payload offset = %d, want it beyond the old head peek", got.Payloads[0].Locator.Offset)
	}
	if spec, ok := got.Payloads[0].String("spec"); !ok || spec != "chara_card_v2" {
		t.Errorf("spec = %q, %v; want chara_card_v2, true", spec, ok)
	}
	for _, read := range store.reads {
		if read.offset == 0 && read.length == int64(len(file)) {
			t.Fatal("probe loaded the whole blob instead of using bounded range reads")
		}
	}
}

func pngFile(chunks ...[]byte) []byte {
	file := append([]byte(nil), []byte("\x89PNG\r\n\x1a\n")...)
	for _, chunk := range chunks {
		file = append(file, chunk...)
	}
	return file
}

func pngChunk(kind string, data []byte) []byte {
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(data)))
	chunk.WriteString(kind)
	chunk.Write(data)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
	return chunk.Bytes()
}

func zipFile(t *testing.T, name, body string) []byte {
	t.Helper()
	var file bytes.Buffer
	archive := zip.NewWriter(&file)
	entry, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	if _, err := io.WriteString(entry, body); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return file.Bytes()
}
