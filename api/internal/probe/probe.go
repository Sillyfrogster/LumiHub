package probe

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"
)

const maxRangeRead = 64 * 1024

type Container string

const (
	Unknown Container = "unknown"
	JSON    Container = "json"
	PNG     Container = "png"
	ZIP     Container = "zip"
)

// RangeStore reads parts of a stored blob.
type RangeStore interface {
	ReadRange(ctx context.Context, id uuid.UUID, offset, length int64) (io.ReadCloser, error)
}

// Inspection holds the container details and decoded payloads from one file.
type Inspection struct {
	Container  Container
	Payloads   []Payload
	PNGChunks  []PNGChunk
	ZIPEntries []ZIPEntry
}

type Locator struct {
	Container Container
	Name      string
	Offset    int64
}

type Payload struct {
	ID      uint32
	Locator Locator
	Root    map[string]json.RawMessage
}

func (p Payload) String(name string) (string, bool) {
	raw, ok := p.Root[name]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

type PNGChunk struct {
	Type   string
	Offset int64
	Length int64
	Name   string
}

type ZIPEntry struct {
	Name               string
	UncompressedSize   uint64
	CompressedSize     uint64
	Directory          bool
	Mode               uint32
	GeneralPurposeBits uint16
}

// Inspect identifies a stored container and decodes its format payloads.
func Inspect(ctx context.Context, store RangeStore, id uuid.UUID, size int64, filename string) (Inspection, error) {
	result := Inspection{Container: Unknown}
	if size == 0 {
		return result, nil
	}
	reader := &rangeReaderAt{ctx: ctx, store: store, id: id, size: size}
	prefixLength := min(size, 8)
	prefix := make([]byte, prefixLength)
	if _, err := reader.ReadAt(prefix, 0); err != nil && !errors.Is(err, io.EOF) {
		return Inspection{}, fmt.Errorf("read container signature: %w", err)
	}
	jsonObject, err := looksLikeJSONObject(reader, prefix, filename)
	if err != nil {
		return Inspection{}, fmt.Errorf("inspect JSON signature: %w", err)
	}

	switch {
	case bytes.Equal(prefix, []byte("\x89PNG\r\n\x1a\n")):
		result.Container = PNG
		if err := inspectPNG(reader, &result); err != nil {
			return Inspection{}, err
		}
	case isZIP(prefix):
		result.Container = ZIP
		if err := inspectZIP(reader, &result); err != nil {
			return Inspection{}, err
		}
	case jsonObject:
		result.Container = JSON
		root, err := decodeObject(io.NewSectionReader(reader, 0, size))
		if err != nil {
			return Inspection{}, fmt.Errorf("inspect JSON: %w", err)
		}
		result.addPayload(Locator{Container: JSON, Name: "root"}, root)
	}
	return result, nil
}

func looksLikeJSONObject(reader *rangeReaderAt, prefix []byte, filename string) (bool, error) {
	marker := firstNonSpace(prefix)
	if marker != 0 {
		return marker == '{', nil
	}
	if !strings.EqualFold(path.Ext(filename), ".json") {
		return false, nil
	}
	length := min(reader.size, maxRangeRead)
	start := make([]byte, length)
	if _, err := reader.ReadAt(start, 0); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return firstNonSpace(start) == '{', nil
}

func isZIP(prefix []byte) bool {
	return len(prefix) >= 4 && bytes.Equal(prefix[:2], []byte("PK")) &&
		(prefix[2] == 3 && prefix[3] == 4 ||
			prefix[2] == 5 && prefix[3] == 6 ||
			prefix[2] == 7 && prefix[3] == 8)
}

func firstNonSpace(data []byte) byte {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b
		}
	}
	return 0
}

func inspectPNG(reader *rangeReaderAt, result *Inspection) error {
	for offset := int64(8); ; {
		if offset+12 > reader.size {
			return fmt.Errorf("inspect PNG: chunk header at byte %d is truncated", offset)
		}
		header := make([]byte, 8)
		if _, err := reader.ReadAt(header, offset); err != nil {
			return fmt.Errorf("inspect PNG chunk at byte %d: %w", offset, err)
		}
		length := int64(binary.BigEndian.Uint32(header[:4]))
		kind := string(header[4:])
		end := offset + 12 + length
		if end < offset || end > reader.size {
			return fmt.Errorf("inspect PNG: %s chunk at byte %d is truncated", kind, offset)
		}

		chunk := PNGChunk{Type: kind, Offset: offset, Length: length}
		if kind == "tEXt" {
			data := make([]byte, length)
			if _, err := reader.ReadAt(data, offset+8); err != nil {
				return fmt.Errorf("read PNG text chunk at byte %d: %w", offset, err)
			}
			name, root, ok := textPayload(data)
			chunk.Name = name
			if ok {
				result.addPayload(Locator{Container: PNG, Name: name, Offset: offset}, root)
			}
		}
		result.PNGChunks = append(result.PNGChunks, chunk)
		offset = end
		if kind == "IEND" {
			if offset != reader.size {
				return fmt.Errorf("inspect PNG: %d bytes follow IEND", reader.size-offset)
			}
			return nil
		}
	}
}

func inspectZIP(reader *rangeReaderAt, result *Inspection) error {
	archive, err := zip.NewReader(reader, reader.size)
	if err != nil {
		return fmt.Errorf("inspect ZIP: %w", err)
	}
	for _, entry := range archive.File {
		offset, err := entry.DataOffset()
		if err != nil {
			return fmt.Errorf("inspect ZIP entry %q: %w", entry.Name, err)
		}
		result.ZIPEntries = append(result.ZIPEntries, ZIPEntry{
			Name:               entry.Name,
			UncompressedSize:   entry.UncompressedSize64,
			CompressedSize:     entry.CompressedSize64,
			Directory:          entry.FileInfo().IsDir(),
			Mode:               uint32(entry.Mode()),
			GeneralPurposeBits: entry.Flags,
		})
		if entry.FileInfo().IsDir() || !strings.EqualFold(entry.Name, "card.json") {
			continue
		}
		opened, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open ZIP entry %q: %w", entry.Name, err)
		}
		root, decodeErr := decodeObject(opened)
		closeErr := opened.Close()
		if decodeErr != nil {
			return fmt.Errorf("inspect ZIP entry %q: %w", entry.Name, decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close ZIP entry %q: %w", entry.Name, closeErr)
		}
		result.addPayload(Locator{Container: ZIP, Name: entry.Name, Offset: offset}, root)
	}
	return nil
}

func (i *Inspection) addPayload(locator Locator, root map[string]json.RawMessage) {
	i.Payloads = append(i.Payloads, Payload{
		ID:      uint32(len(i.Payloads)),
		Locator: locator,
		Root:    root,
	})
}

func decodeObject(reader io.Reader) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(reader)
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, fmt.Errorf("JSON root is not an object")
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("JSON has more than one root value")
		}
		return nil, err
	}
	return root, nil
}

func textPayload(data []byte) (string, map[string]json.RawMessage, bool) {
	separator := bytes.IndexByte(data, 0)
	if separator < 1 {
		return "", nil, false
	}
	name := string(data[:separator])
	text := data[separator+1:]
	if root, ok := object(text); ok {
		return name, root, true
	}
	decoded, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return name, nil, false
	}
	root, ok := object(decoded)
	return name, root, ok
}

func object(data []byte) (map[string]json.RawMessage, bool) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return nil, false
	}
	return root, true
}

type rangeReaderAt struct {
	ctx   context.Context
	store RangeStore
	id    uuid.UUID
	size  int64
}

func (r *rangeReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("negative range offset")
	}
	if offset >= r.size {
		return 0, io.EOF
	}
	wanted := min(int64(len(p)), r.size-offset)
	read := int64(0)
	for read < wanted {
		length := min(wanted-read, maxRangeRead)
		part, err := r.store.ReadRange(r.ctx, r.id, offset+read, length)
		if err != nil {
			return int(read), err
		}
		n, copyErr := io.ReadFull(part, p[read:read+length])
		closeErr := part.Close()
		read += int64(n)
		if copyErr != nil {
			return int(read), copyErr
		}
		if closeErr != nil {
			return int(read), closeErr
		}
	}
	if wanted < int64(len(p)) {
		return int(read), io.EOF
	}
	return int(read), nil
}
