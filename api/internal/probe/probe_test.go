package probe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
)

var errReadUnavailable = errors.New("read unavailable")

type recordingStore struct {
	data  []byte
	reads []byteRange
}

type failingStore struct{}

func (failingStore) ReadRange(context.Context, uuid.UUID, int64, int64) (io.ReadCloser, error) {
	return nil, errReadUnavailable
}

func TestInspectDistinguishesMalformedInputFromAStorageFailure(t *testing.T) {
	malformed := []byte(`{"broken":`)
	_, malformedErr := Inspect(
		context.Background(), &recordingStore{data: malformed}, uuid.New(), int64(len(malformed)), "card.json",
	)
	if !errors.Is(malformedErr, ErrMalformedInput) {
		t.Fatalf("malformed error = %v, want ErrMalformedInput", malformedErr)
	}

	_, readErr := Inspect(context.Background(), failingStore{}, uuid.New(), 8, "card.json")
	if !errors.Is(readErr, errReadUnavailable) {
		t.Fatalf("read error = %v, want storage cause", readErr)
	}
	if errors.Is(readErr, ErrMalformedInput) {
		t.Fatalf("storage error was reported as malformed input: %v", readErr)
	}
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

func TestInspectRejectsAWebPSignatureWithoutAnImage(t *testing.T) {
	file := []byte("RIFF\x0c\x00\x00\x00WEBPVP8X\x00\x00\x00\x00")
	_, err := Inspect(
		context.Background(), &recordingStore{data: file}, uuid.New(), int64(len(file)), "image.webp",
	)
	if !errors.Is(err, ErrMalformedInput) {
		t.Fatalf("Inspect error = %v, want ErrMalformedInput", err)
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

func TestInspectEnforcesEveryArchiveResourceLimit(t *testing.T) {
	cases := []struct {
		name   string
		file   func(t *testing.T) []byte
		limits func() Limits
	}{
		{
			name: "entry count",
			file: func(t *testing.T) []byte {
				return zipEntries(t, zipEntry{"one", "1", zip.Store}, zipEntry{"two", "2", zip.Store})
			},
			limits: func() Limits {
				limits := DefaultLimits()
				limits.MaxArchiveEntries = 1
				return limits
			},
		},
		{
			name: "entry bytes",
			file: func(t *testing.T) []byte {
				return zipEntries(t, zipEntry{"large", "12345", zip.Store})
			},
			limits: func() Limits {
				limits := DefaultLimits()
				limits.MaxEntryBytes = 4
				return limits
			},
		},
		{
			name: "total bytes",
			file: func(t *testing.T) []byte {
				return zipEntries(t, zipEntry{"one", "123", zip.Store}, zipEntry{"two", "456", zip.Store})
			},
			limits: func() Limits {
				limits := DefaultLimits()
				limits.MaxArchiveBytes = 5
				return limits
			},
		},
		{
			name: "compression ratio",
			file: func(t *testing.T) []byte {
				return zipEntries(t, zipEntry{"compressed", strings.Repeat("0", 4096), zip.Deflate})
			},
			limits: func() Limits {
				limits := DefaultLimits()
				limits.MaxCompressionRatio = 2
				return limits
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			file := test.file(t)
			_, err := InspectWithLimits(
				context.Background(), &recordingStore{data: file}, uuid.New(),
				int64(len(file)), "bundle.zip", test.limits(),
			)
			if !errors.Is(err, ErrSafetyViolation) {
				t.Fatalf("InspectWithLimits error = %v, want ErrSafetyViolation", err)
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
	return zipEntries(t, zipEntry{name: name, body: body, method: zip.Store})
}

type zipEntry struct {
	name   string
	body   string
	method uint16
}

func zipEntries(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var file bytes.Buffer
	archive := zip.NewWriter(&file)
	for _, value := range entries {
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: value.name, Method: value.method})
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := io.WriteString(entry, value.body); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return file.Bytes()
}

func TestInspectOffersARasterFileAsItsOwnImage(t *testing.T) {
	file := pngFile(
		pngChunk("IHDR", make([]byte, 13)),
		pngChunk("tEXt", append([]byte("chara\x00"), []byte(`{"spec":"chara_card_v2"}`)...)),
		pngChunk("IEND", nil),
	)
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "card.png")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(got.Images) != 1 {
		t.Fatalf("image count = %d, want the file itself", len(got.Images))
	}
	if got.Images[0].Locator.Container != PNG || got.Images[0].Locator.Name != "" {
		t.Fatalf("image locator = %+v, want the whole PNG", got.Images[0].Locator)
	}

	opened, err := got.OpenImage(context.Background(), got.Images[0].ID)
	if err != nil {
		t.Fatalf("OpenImage: %v", err)
	}
	defer opened.Close()
	read, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	if !bytes.Equal(read, file) {
		t.Fatal("the extracted image is not the source bytes")
	}
}

func TestInspectListsArchivedImagesAndLeavesOtherEntriesAlone(t *testing.T) {
	picture := pngFile(pngChunk("IHDR", make([]byte, 13)), pngChunk("IEND", nil))
	file := zipEntries(t,
		zipEntry{name: "card.json", body: `{"spec":"chara_card_v3"}`, method: zip.Store},
		zipEntry{name: "assets/icon/main.png", body: string(picture), method: zip.Store},
		zipEntry{name: "assets/notes.txt", body: "not a picture", method: zip.Store},
	)
	store := &recordingStore{data: file}

	got, err := Inspect(context.Background(), store, uuid.New(), int64(len(file)), "card.charx")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(got.Images) != 1 {
		t.Fatalf("image count = %d, want only the PNG entry", len(got.Images))
	}
	if got.Images[0].Locator.Name != "assets/icon/main.png" {
		t.Fatalf("image locator = %+v, want the icon entry", got.Images[0].Locator)
	}

	opened, err := got.OpenImage(context.Background(), got.Images[0].ID)
	if err != nil {
		t.Fatalf("OpenImage: %v", err)
	}
	defer opened.Close()
	read, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	if !bytes.Equal(read, picture) {
		t.Fatal("the extracted archive entry is not the stored picture")
	}
}

func TestOpenImageRefusesAnIDTheProbeNeverIssued(t *testing.T) {
	file := []byte(`{"spec":"chara_card_v3"}`)
	got, err := Inspect(context.Background(), &recordingStore{data: file}, uuid.New(), int64(len(file)), "card.json")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(got.Images) != 0 {
		t.Fatalf("image count = %d, want none in a bare JSON card", len(got.Images))
	}
	if _, err := got.OpenImage(context.Background(), 0); err == nil {
		t.Fatal("OpenImage accepted an image the probe never located")
	}
}
