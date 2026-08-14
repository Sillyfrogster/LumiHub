package character

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
)

var errInvalidJSON = errors.New("invalid JSON object")

type valueSpan struct {
	start int
	end   int
}

type byteEdit struct {
	start       int
	end         int
	replacement []byte
}

func exportJSON(request format.ExportRequest) (format.ExportedArtifact, error) {
	if err := validatePatch(request.Patch); err != nil {
		return format.ExportedArtifact{}, err
	}
	if !declaresTarget(request.Target) {
		return format.ExportedArtifact{}, fmt.Errorf("export target %q is not declared", request.Target)
	}
	source, err := io.ReadAll(request.Source)
	if err != nil {
		return format.ExportedArtifact{}, fmt.Errorf("read source: %w", err)
	}
	written, err := patchCardJSON(source, request.Patch)
	if err != nil {
		return format.ExportedArtifact{}, err
	}
	return format.ExportedArtifact{
		Artifact: bytes.NewReader(written), MediaType: "application/json", Extension: ".json",
		UnembeddedMedia: request.Media,
	}, nil
}

func exportCard(request format.ExportRequest, formatID, chunkName string) (format.ExportedArtifact, error) {
	if err := validatePatch(request.Patch); err != nil {
		return format.ExportedArtifact{}, err
	}
	if !declaresTarget(request.Target) {
		return format.ExportedArtifact{}, fmt.Errorf("export target %q is not declared", request.Target)
	}
	source, err := io.ReadAll(request.Source)
	if err != nil {
		return format.ExportedArtifact{}, fmt.Errorf("read source: %w", err)
	}
	if bytes.HasPrefix(source, []byte("\x89PNG\r\n\x1a\n")) {
		written, err := exportPNG(source, formatID, chunkName, request.Patch)
		if err != nil {
			return format.ExportedArtifact{}, err
		}
		return format.ExportedArtifact{
			Artifact: bytes.NewReader(written), MediaType: "image/png", Extension: ".png",
			UnembeddedMedia: request.Media,
		}, nil
	}
	return exportJSON(format.ExportRequest{
		Source: bytes.NewReader(source), Target: request.Target, Patch: request.Patch, Media: request.Media,
	})
}

func exportPNG(source []byte, formatID, chunkName string, patch format.Patch) ([]byte, error) {
	var output bytes.Buffer
	output.Write(source[:8])
	inserted := false
	for offset := 8; offset < len(source); {
		if offset+12 > len(source) {
			return nil, fmt.Errorf("PNG chunk at byte %d is truncated", offset)
		}
		length := int(binary.BigEndian.Uint32(source[offset : offset+4]))
		end := offset + 12 + length
		if length < 0 || end < offset || end > len(source) {
			return nil, fmt.Errorf("PNG chunk at byte %d exceeds the file", offset)
		}
		kind := string(source[offset+4 : offset+8])
		data := source[offset+8 : offset+8+length]
		keyword, _, hasText := bytes.Cut(data, []byte{0})
		isCard := kind == "tEXt" && hasText && (string(keyword) == "chara" || string(keyword) == "ccv3")
		if isCard {
			if !inserted {
				card, err := decodeCardText(data[len(keyword)+1:])
				if err == nil && cardMatchesFormat(card, formatID) {
					written, err := patchCardJSON(card, patch)
					if err != nil {
						return nil, err
					}
					encoded := base64.StdEncoding.EncodeToString(written)
					output.Write(makePNGChunk("tEXt", slices.Concat([]byte(chunkName), []byte{0}, []byte(encoded))))
					inserted = true
				}
			}
		} else {
			output.Write(source[offset:end])
		}
		offset = end
	}
	if !inserted {
		return nil, fmt.Errorf("PNG has no %s card payload", formatID)
	}
	return output.Bytes(), nil
}

func cardMatchesFormat(card []byte, formatID string) bool {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(card, &root); err != nil {
		return false
	}
	var spec string
	_ = json.Unmarshal(root["spec"], &spec)
	if formatID == V2 {
		return spec == V2 || spec == "" && hasLegacyShape(root)
	}
	return spec == formatID
}

func decodeCardText(text []byte) ([]byte, error) {
	if json.Valid(text) {
		return slices.Clone(text), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil || !json.Valid(decoded) {
		return nil, errInvalidJSON
	}
	return decoded, nil
}

func makePNGChunk(kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return chunk
}

func exportCharX(request format.ExportRequest) (format.ExportedArtifact, error) {
	if err := validatePatch(request.Patch); err != nil {
		return format.ExportedArtifact{}, err
	}
	if !declaresTarget(request.Target) {
		return format.ExportedArtifact{}, fmt.Errorf("export target %q is not declared", request.Target)
	}
	source, err := io.ReadAll(request.Source)
	if err != nil {
		return format.ExportedArtifact{}, fmt.Errorf("read source: %w", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		return format.ExportedArtifact{}, fmt.Errorf("open CHARX: %w", err)
	}
	entries := make(map[string]bool, len(archive.File)+len(request.Media))
	for _, entry := range archive.File {
		if unsafeExportPath(entry.Name) || entries[entry.Name] {
			return format.ExportedArtifact{}, fmt.Errorf("unsafe or duplicate CHARX entry %q", entry.Name)
		}
		entries[entry.Name] = true
	}
	mediaEntries, assets, err := charXMedia(request.Media, entries)
	if err != nil {
		return format.ExportedArtifact{}, err
	}

	var output bytes.Buffer
	written := zip.NewWriter(&output)
	foundCard := false
	for _, entry := range archive.File {
		if entry.Name == "card.json" {
			foundCard = true
			card, err := readZipEntry(entry)
			if err != nil {
				return format.ExportedArtifact{}, err
			}
			card, err = patchCardJSON(card, request.Patch)
			if err != nil {
				return format.ExportedArtifact{}, err
			}
			card, err = appendCardAssets(card, assets)
			if err != nil {
				return format.ExportedArtifact{}, err
			}
			destination, err := written.CreateHeader(&entry.FileHeader)
			if err != nil {
				return format.ExportedArtifact{}, fmt.Errorf("create card.json: %w", err)
			}
			if _, err := destination.Write(card); err != nil {
				return format.ExportedArtifact{}, fmt.Errorf("write card.json: %w", err)
			}
			continue
		}
		raw, err := entry.OpenRaw()
		if err != nil {
			return format.ExportedArtifact{}, fmt.Errorf("open CHARX entry %q: %w", entry.Name, err)
		}
		destination, err := written.CreateRaw(&entry.FileHeader)
		if err != nil {
			return format.ExportedArtifact{}, fmt.Errorf("copy CHARX entry %q: %w", entry.Name, err)
		}
		if _, err := io.Copy(destination, raw); err != nil {
			return format.ExportedArtifact{}, fmt.Errorf("copy CHARX entry %q: %w", entry.Name, err)
		}
	}
	if !foundCard {
		return format.ExportedArtifact{}, errors.New("CHARX has no card.json")
	}
	for _, entry := range mediaEntries {
		destination, err := written.Create(entry.path)
		if err != nil {
			return format.ExportedArtifact{}, fmt.Errorf("create CHARX media %q: %w", entry.path, err)
		}
		if _, err := destination.Write(entry.data); err != nil {
			return format.ExportedArtifact{}, fmt.Errorf("write CHARX media %q: %w", entry.path, err)
		}
	}
	if err := written.Close(); err != nil {
		return format.ExportedArtifact{}, fmt.Errorf("close CHARX: %w", err)
	}
	return format.ExportedArtifact{
		Artifact: bytes.NewReader(output.Bytes()), MediaType: "application/zip", Extension: ".charx",
	}, nil
}

type charXMediaEntry struct {
	path string
	data []byte
}

func charXMedia(available []format.ExportMedia, entries map[string]bool) ([]charXMediaEntry, []json.RawMessage, error) {
	written := make([]charXMediaEntry, 0, len(available))
	assets := make([]json.RawMessage, 0, len(available))
	for _, item := range available {
		assetType, name := charXMediaRole(item.Role, item.ID)
		extension := mediaExtension(item.MediaType)
		entryPath := path.Join("assets", assetType, "images", safeMediaName(item.ID)+"."+extension)
		if entries[entryPath] {
			return nil, nil, fmt.Errorf("CHARX media path %q already exists", entryPath)
		}
		entries[entryPath] = true
		asset, err := json.Marshal(struct {
			Type string `json:"type"`
			URI  string `json:"uri"`
			Name string `json:"name"`
			Ext  string `json:"ext"`
		}{Type: assetType, URI: embeddedPrefix + entryPath, Name: name, Ext: extension})
		if err != nil {
			return nil, nil, fmt.Errorf("write CHARX media record: %w", err)
		}
		written = append(written, charXMediaEntry{path: entryPath, data: item.Data})
		assets = append(assets, asset)
	}
	return written, assets, nil
}

func charXMediaRole(role media.Role, id string) (string, string) {
	switch role {
	case media.Avatar:
		return "icon", "main"
	case media.AvatarAlt:
		return "icon", id
	case media.Expression:
		return "emotion", id
	case media.PerspectiveLayer:
		return "x_perspective_layer", id
	default:
		return "x_gallery", id
	}
}

func mediaExtension(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return "bin"
	}
}

func safeMediaName(id string) string {
	var name strings.Builder
	for _, value := range id {
		if value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' ||
			value >= '0' && value <= '9' || value == '-' || value == '_' {
			name.WriteRune(value)
		} else {
			name.WriteByte('-')
		}
	}
	if name.Len() == 0 {
		return "media"
	}
	return name.String()
}

func appendCardAssets(card []byte, additions []json.RawMessage) ([]byte, error) {
	if len(additions) == 0 {
		return card, nil
	}
	root, _, err := objectSpans(card)
	if err != nil {
		return nil, fmt.Errorf("read CHARX card: %w", err)
	}
	dataSpan, ok := root["data"]
	if !ok {
		return nil, errors.New("CHARX card has no data object")
	}
	data := card[dataSpan.start:dataSpan.end]
	fields, _, err := objectSpans(data)
	if err != nil {
		return nil, fmt.Errorf("read CHARX card data: %w", err)
	}
	var assets []byte
	if span, ok := fields["assets"]; ok {
		assets, err = appendJSONArray(data[span.start:span.end], additions)
		if err != nil {
			return nil, fmt.Errorf("append CHARX assets: %w", err)
		}
	} else {
		assets = []byte("[]")
		assets, _ = appendJSONArray(assets, additions)
	}
	data, err = setJSONObjectRaw(data, "assets", assets)
	if err != nil {
		return nil, err
	}
	result := slices.Concat(card[:dataSpan.start], data, card[dataSpan.end:])
	if !json.Valid(result) {
		return nil, errors.New("exported CHARX card is not valid JSON")
	}
	return result, nil
}

func appendJSONArray(array []byte, additions []json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(array)
	if len(trimmed) < 2 || trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' || !json.Valid(trimmed) {
		return nil, errInvalidJSON
	}
	closeAt := bytes.LastIndexByte(array, ']')
	hasValues := len(bytes.TrimSpace(array[bytes.IndexByte(array, '[')+1:closeAt])) > 0
	var added bytes.Buffer
	if hasValues {
		added.WriteByte(',')
	}
	for i, value := range additions {
		if i > 0 {
			added.WriteByte(',')
		}
		added.Write(value)
	}
	return slices.Concat(array[:closeAt], added.Bytes(), array[closeAt:]), nil
}

func setJSONObjectRaw(object []byte, field string, value []byte) ([]byte, error) {
	if !json.Valid(value) {
		return nil, errInvalidJSON
	}
	fields, closeAt, err := objectSpans(object)
	if err != nil {
		return nil, err
	}
	if span, ok := fields[field]; ok {
		return slices.Concat(object[:span.start], value, object[span.end:]), nil
	}
	key, _ := json.Marshal(field)
	prefix := []byte(nil)
	if len(fields) > 0 {
		prefix = []byte{','}
	}
	addition := slices.Concat(prefix, key, []byte{':'}, value)
	return slices.Concat(object[:closeAt], addition, object[closeAt:]), nil
}

func readZipEntry(entry *zip.File) ([]byte, error) {
	opened, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open CHARX entry %q: %w", entry.Name, err)
	}
	data, readErr := io.ReadAll(opened)
	closeErr := opened.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read CHARX entry %q: %w", entry.Name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close CHARX entry %q: %w", entry.Name, closeErr)
	}
	return data, nil
}

func unsafeExportPath(name string) bool {
	cleaned := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	return cleaned == "." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/")
}

func declaresTarget(target string) bool {
	if target == format.RawTarget {
		return true
	}
	for _, option := range exportTargets() {
		if option.Value == target {
			return true
		}
	}
	return false
}

func patchCardJSON(source []byte, patch format.Patch) ([]byte, error) {
	root, _, err := objectSpans(source)
	if err != nil {
		return nil, fmt.Errorf("read card root: %w", err)
	}
	data, ok := root["data"]
	if !ok {
		applicable := make(format.Patch)
		for field, value := range patch {
			if _, exists := root[string(field)]; exists {
				applicable[field] = value
			}
		}
		return patchJSONObject(source, applicable)
	}
	written, err := patchJSONObject(source[data.start:data.end], patch)
	if err != nil {
		return nil, fmt.Errorf("patch card data: %w", err)
	}
	result := slices.Concat(source[:data.start], written, source[data.end:])
	if !json.Valid(result) {
		return nil, fmt.Errorf("patched card is not valid JSON: %w", format.ErrInvalidPatch)
	}
	return result, nil
}

func patchJSONObject(source []byte, patch format.Patch) ([]byte, error) {
	fields, closeAt, err := objectSpans(source)
	if err != nil {
		return nil, err
	}
	edits := make([]byteEdit, 0, len(patch)+1)
	missing := make([]format.Field, 0)
	for field, value := range patch {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode field %q: %w", field, err)
		}
		if span, ok := fields[string(field)]; ok {
			edits = append(edits, byteEdit{start: span.start, end: span.end, replacement: encoded})
		} else {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		var addition bytes.Buffer
		if len(fields) > 0 {
			addition.WriteByte(',')
		}
		for i, field := range missing {
			if i > 0 {
				addition.WriteByte(',')
			}
			key, _ := json.Marshal(field)
			value, _ := json.Marshal(patch[field])
			addition.Write(key)
			addition.WriteByte(':')
			addition.Write(value)
		}
		edits = append(edits, byteEdit{start: closeAt, end: closeAt, replacement: addition.Bytes()})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	result := slices.Clone(source)
	for _, edit := range edits {
		result = slices.Concat(result[:edit.start], edit.replacement, result[edit.end:])
	}
	if !json.Valid(result) {
		return nil, fmt.Errorf("patched object is not valid JSON: %w", format.ErrInvalidPatch)
	}
	return result, nil
}

func objectSpans(source []byte) (map[string]valueSpan, int, error) {
	i := skipSpace(source, 0)
	if i >= len(source) || source[i] != '{' {
		return nil, 0, errInvalidJSON
	}
	i++
	fields := make(map[string]valueSpan)
	for {
		i = skipSpace(source, i)
		if i >= len(source) {
			return nil, 0, errInvalidJSON
		}
		if source[i] == '}' {
			return fields, i, nil
		}
		keyStart := i
		keyEnd, err := scanString(source, keyStart)
		if err != nil {
			return nil, 0, err
		}
		var key string
		if err := json.Unmarshal(source[keyStart:keyEnd], &key); err != nil {
			return nil, 0, errInvalidJSON
		}
		i = skipSpace(source, keyEnd)
		if i >= len(source) || source[i] != ':' {
			return nil, 0, errInvalidJSON
		}
		start := skipSpace(source, i+1)
		end, err := scanValue(source, start)
		if err != nil {
			return nil, 0, err
		}
		fields[key] = valueSpan{start: start, end: end}
		i = skipSpace(source, end)
		if i >= len(source) {
			return nil, 0, errInvalidJSON
		}
		switch source[i] {
		case ',':
			i++
		case '}':
			return fields, i, nil
		default:
			return nil, 0, errInvalidJSON
		}
	}
}

func scanValue(source []byte, start int) (int, error) {
	if start >= len(source) {
		return 0, errInvalidJSON
	}
	if source[start] == '"' {
		return scanString(source, start)
	}
	if source[start] == '{' || source[start] == '[' {
		stack := []byte{source[start]}
		for i := start + 1; i < len(source); i++ {
			switch source[i] {
			case '"':
				end, err := scanString(source, i)
				if err != nil {
					return 0, err
				}
				i = end - 1
			case '{', '[':
				stack = append(stack, source[i])
			case '}', ']':
				opening := stack[len(stack)-1]
				if opening == '{' && source[i] != '}' || opening == '[' && source[i] != ']' {
					return 0, errInvalidJSON
				}
				stack = stack[:len(stack)-1]
				if len(stack) == 0 {
					return i + 1, nil
				}
			}
		}
		return 0, errInvalidJSON
	}
	for i := start; i < len(source); i++ {
		if source[i] == ',' || source[i] == '}' || source[i] == ']' || unicode.IsSpace(rune(source[i])) {
			if !json.Valid(slices.Concat([]byte{'['}, source[start:i], []byte{']'})) {
				return 0, errInvalidJSON
			}
			return i, nil
		}
	}
	return 0, errInvalidJSON
}

func scanString(source []byte, start int) (int, error) {
	if start >= len(source) || source[start] != '"' {
		return 0, errInvalidJSON
	}
	for i := start + 1; i < len(source); i++ {
		switch source[i] {
		case '\\':
			i++
		case '"':
			return i + 1, nil
		}
	}
	return 0, errInvalidJSON
}

func skipSpace(source []byte, start int) int {
	for start < len(source) && unicode.IsSpace(rune(source[start])) {
		start++
	}
	return start
}
