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
	"image/gif"
	"image/jpeg"
	"io"
	"os"
	"path"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/image/webp"
)

const maxRangeRead = 64 * 1024

var (
	ErrMalformedInput  = errors.New("malformed input")
	ErrSafetyViolation = errors.New("archive safety violation")
	// ErrRangeRead marks a failure in the backing blob store rather than bad source bytes.
	ErrRangeRead = errors.New("blob range read failed")
)

type Limits struct {
	MaxArchiveEntries   int
	MaxEntryBytes       uint64
	MaxArchiveBytes     uint64
	MaxCompressionRatio float64
}

func DefaultLimits() Limits {
	return Limits{
		MaxArchiveEntries:   512,
		MaxEntryBytes:       32 << 20,
		MaxArchiveBytes:     128 << 20,
		MaxCompressionRatio: 100,
	}
}

type Container string

const (
	Unknown Container = "unknown"
	JSON    Container = "json"
	PNG     Container = "png"
	JPEG    Container = "jpeg"
	WebP    Container = "webp"
	GIF     Container = "gif"
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
	Images     []Image
	PNGChunks  []PNGChunk
	ZIPEntries []ZIPEntry

	// source is where the file lives, so an image can be read back later
	// without a caller having to remember the blob.
	source blobSource
}

// ByteSize returns the exact size of the inspected source container.
func (i Inspection) ByteSize() int64 { return i.source.size }

type blobSource struct {
	store RangeStore
	id    uuid.UUID
	size  int64
}

// Image is one picture the probe found. A format module labels images by ID;
// only the layer that stores media ever reads the bytes.
type Image struct {
	ID      uint32
	Locator Locator
}

// ErrImageUnavailable marks an optional image that cannot be reopened from the source.
var ErrImageUnavailable = errors.New("the probe located no such image")

// OpenImage streams one located image through the same bounded range reads the
// inspection itself used.
func (i Inspection) OpenImage(ctx context.Context, id uint32) (io.ReadCloser, error) {
	var found Image
	var ok bool
	for _, image := range i.Images {
		if image.ID == id {
			found, ok = image, true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("image %d: %w", id, ErrImageUnavailable)
	}
	reader := &rangeReaderAt{ctx: ctx, store: i.source.store, id: i.source.id, size: i.source.size}
	if found.Locator.Container != ZIP {
		return io.NopCloser(io.NewSectionReader(reader, 0, i.source.size)), nil
	}
	archive, err := zip.NewReader(reader, i.source.size)
	if err != nil {
		return nil, fmt.Errorf("reopen archive for image %d: %w", id, err)
	}
	for _, entry := range archive.File {
		if entry.Name != found.Locator.Name {
			continue
		}
		opened, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open archive entry %q: %w", entry.Name, err)
		}
		return opened, nil
	}
	return nil, fmt.Errorf("archive entry %q: %w", found.Locator.Name, ErrImageUnavailable)
}

// imageExtensions name the raster formats the media layer can decode. An
// archived entry is worth offering as an image when its name ends in one of
// them. The name only says where to look; the decoder decides what it is.
var imageExtensions = map[string]bool{
	".png":  true,
	".apng": true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
	".gif":  true,
}

var inlineMediaTypes = map[Container]string{
	PNG:  "image/png",
	JPEG: "image/jpeg",
	WebP: "image/webp",
	GIF:  "image/gif",
}

// InlineMediaType returns a browser-safe type only for a verified raster container.
func (i Inspection) InlineMediaType() string {
	return inlineMediaTypes[i.Container]
}

// IsInlineMediaType reports whether a stored probe result is safe to render.
func IsInlineMediaType(mediaType string) bool {
	for _, known := range inlineMediaTypes {
		if mediaType == known {
			return true
		}
	}
	return false
}

type Locator struct {
	Container Container
	Name      string
	Offset    int64
}

type Payload struct {
	ID       uint32
	Locator  Locator
	Root     map[string]json.RawMessage
	ByteSize int64
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
	return InspectWithLimits(ctx, store, id, size, filename, DefaultLimits())
}

func InspectWithLimits(
	ctx context.Context,
	store RangeStore,
	id uuid.UUID,
	size int64,
	filename string,
	limits Limits,
) (Inspection, error) {
	result := Inspection{
		Container: Unknown,
		source:    blobSource{store: store, id: id, size: size},
	}
	if size == 0 {
		return result, nil
	}
	reader := &rangeReaderAt{ctx: ctx, store: store, id: id, size: size}
	prefixLength := min(size, 12)
	prefix := make([]byte, prefixLength)
	if _, err := reader.ReadAt(prefix, 0); err != nil && !errors.Is(err, io.EOF) {
		return Inspection{}, fmt.Errorf("read container signature: %w", err)
	}
	jsonObject, err := looksLikeJSONObject(reader, prefix, filename)
	if err != nil {
		return Inspection{}, fmt.Errorf("inspect JSON signature: %w", err)
	}

	switch {
	case len(prefix) >= 8 && bytes.Equal(prefix[:8], []byte("\x89PNG\r\n\x1a\n")):
		result.Container = PNG
		if err := inspectPNG(reader, &result); err != nil {
			return Inspection{}, classifyContainerError(err)
		}
	case isJPEG(prefix):
		result.Container = JPEG
		if _, err := jpeg.DecodeConfig(io.NewSectionReader(reader, 0, size)); err != nil {
			return Inspection{}, classifyContainerError(fmt.Errorf("inspect JPEG: %w", err))
		}
	case isWebP(prefix):
		result.Container = WebP
		if _, err := webp.DecodeConfig(io.NewSectionReader(reader, 0, size)); err != nil {
			return Inspection{}, classifyContainerError(fmt.Errorf("inspect WebP: %w", err))
		}
	case isGIF(prefix):
		result.Container = GIF
		if _, err := gif.DecodeConfig(io.NewSectionReader(reader, 0, size)); err != nil {
			return Inspection{}, classifyContainerError(fmt.Errorf("inspect GIF: %w", err))
		}
	case isZIP(prefix):
		result.Container = ZIP
		if err := inspectZIP(reader, &result, limits); err != nil {
			return Inspection{}, classifyContainerError(err)
		}
	case jsonObject:
		result.Container = JSON
		root, err := decodeObject(io.NewSectionReader(reader, 0, size))
		if err != nil {
			return Inspection{}, classifyContainerError(fmt.Errorf("inspect JSON: %w", err))
		}
		result.addPayload(Locator{Container: JSON, Name: "root"}, root, size)
	}
	if _, raster := inlineMediaTypes[result.Container]; raster {
		result.addImage(Locator{Container: result.Container})
	}
	return result, nil
}

func classifyContainerError(err error) error {
	if errors.Is(err, ErrSafetyViolation) || errors.Is(err, ErrRangeRead) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrMalformedInput, err)
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

func isJPEG(prefix []byte) bool {
	return len(prefix) >= 3 && prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff
}

func isWebP(prefix []byte) bool {
	return len(prefix) >= 12 && string(prefix[:4]) == "RIFF" && string(prefix[8:12]) == "WEBP"
}

func isGIF(prefix []byte) bool {
	return len(prefix) >= 6 && (string(prefix[:6]) == "GIF87a" || string(prefix[:6]) == "GIF89a")
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
			name, root, payloadBytes, ok := textPayload(data)
			chunk.Name = name
			if ok {
				result.addPayload(Locator{Container: PNG, Name: name, Offset: offset}, root, payloadBytes)
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

func inspectZIP(reader *rangeReaderAt, result *Inspection, limits Limits) error {
	archive, err := zip.NewReader(reader, reader.size)
	if err != nil {
		return fmt.Errorf("inspect ZIP: %w", err)
	}
	if len(archive.File) > limits.MaxArchiveEntries {
		return fmt.Errorf("%w: archive has %d entries, limit is %d",
			ErrSafetyViolation, len(archive.File), limits.MaxArchiveEntries)
	}
	var archiveBytes uint64
	for _, entry := range archive.File {
		if unsafeArchivePath(entry.Name) {
			return fmt.Errorf("%w: unsafe archive path %q", ErrSafetyViolation, entry.Name)
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: archive entry %q is a symlink", ErrSafetyViolation, entry.Name)
		}
		if entry.Flags&1 != 0 {
			return fmt.Errorf("%w: archive entry %q is encrypted", ErrSafetyViolation, entry.Name)
		}
		if entry.UncompressedSize64 > limits.MaxEntryBytes {
			return fmt.Errorf("%w: archive entry %q is too large", ErrSafetyViolation, entry.Name)
		}
		if entry.UncompressedSize64 > 0 && (entry.CompressedSize64 == 0 ||
			float64(entry.UncompressedSize64)/float64(entry.CompressedSize64) > limits.MaxCompressionRatio) {
			return fmt.Errorf("%w: archive entry %q exceeds the compression ratio",
				ErrSafetyViolation, entry.Name)
		}
		if ^uint64(0)-archiveBytes < entry.UncompressedSize64 {
			return fmt.Errorf("%w: archive size overflow", ErrSafetyViolation)
		}
		archiveBytes += entry.UncompressedSize64
		if archiveBytes > limits.MaxArchiveBytes {
			return fmt.Errorf("%w: archive expands beyond its limit", ErrSafetyViolation)
		}
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
		if entry.FileInfo().IsDir() {
			continue
		}
		if imageExtensions[strings.ToLower(path.Ext(entry.Name))] {
			result.addImage(Locator{Container: ZIP, Name: entry.Name, Offset: offset})
		}
		if !strings.EqualFold(entry.Name, "card.json") {
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
		result.addPayload(
			Locator{Container: ZIP, Name: entry.Name, Offset: offset},
			root, int64(entry.UncompressedSize64),
		)
	}
	return nil
}

func unsafeArchivePath(name string) bool {
	portable := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(portable, "/") ||
		(len(portable) >= 2 && portable[1] == ':' &&
			((portable[0] >= 'a' && portable[0] <= 'z') ||
				(portable[0] >= 'A' && portable[0] <= 'Z'))) {
		return true
	}
	cleaned := path.Clean(portable)
	return cleaned == ".." || strings.HasPrefix(cleaned, "../")
}

func (i *Inspection) addImage(locator Locator) {
	i.Images = append(i.Images, Image{ID: uint32(len(i.Images)), Locator: locator})
}

func (i *Inspection) addPayload(locator Locator, root map[string]json.RawMessage, byteSize int64) {
	i.Payloads = append(i.Payloads, Payload{
		ID:       uint32(len(i.Payloads)),
		Locator:  locator,
		Root:     root,
		ByteSize: byteSize,
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

func textPayload(data []byte) (string, map[string]json.RawMessage, int64, bool) {
	separator := bytes.IndexByte(data, 0)
	if separator < 1 {
		return "", nil, 0, false
	}
	name := string(data[:separator])
	text := data[separator+1:]
	if root, ok := object(text); ok {
		return name, root, int64(len(text)), true
	}
	decoded, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return name, nil, 0, false
	}
	root, ok := object(decoded)
	return name, root, int64(len(decoded)), ok
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
			return int(read), fmt.Errorf("%w: %w", ErrRangeRead, err)
		}
		n, copyErr := io.ReadFull(part, p[read:read+length])
		closeErr := part.Close()
		read += int64(n)
		if copyErr != nil {
			return int(read), fmt.Errorf("%w: %v", ErrRangeRead, copyErr)
		}
		if closeErr != nil {
			return int(read), fmt.Errorf("%w: %v", ErrRangeRead, closeErr)
		}
	}
	if wanted < int64(len(p)) {
		return int(read), io.EOF
	}
	return int(read), nil
}
