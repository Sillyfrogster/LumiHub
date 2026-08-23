package character

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"path"
	"slices"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/book"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
)

// The card writers build a file out of the asset's roles. Nothing here reads
// another format's bytes. What a card carries is decided by what the asset
// holds and by what the standard has a place for.

const (
	v2SpecVersion = "2.0"
	v3SpecVersion = "3.0"
	// dialogueStart is the separator a card puts before an example exchange.
	dialogueStart = "<START>"
	// defaultAssetURI is how a CCv3 card points at the picture its own
	// container carries.
	defaultAssetURI = "ccdefault:"
)

// v3OnlyKeys must not leak into v2 exports after preservation is restored.
var v3OnlyKeys = []string{
	"assets", "nickname", "group_only_greetings", "creation_date",
	"modification_date", "source", "creator_notes_multilingual",
}

func (CCv2Module) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	return writeCard(asset, V2)
}

func (CCv3Module) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	return writeCard(asset, V3)
}

func (CharXModule) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	return writeCharX(asset)
}

// writeCard chooses an image or JSON container from available media. V3 cards
// also include the v2 payload needed by older readers.
func writeCard(asset format.ExportAsset, formatID string) (format.Artifact, error) {
	picture := embeddablePicture(asset)
	body, entries := cardFields(asset, formatID)
	if formatID != V2 {
		body["assets"] = inlineAssets(asset, picture != nil)
	}
	if err := RestorePreserved(body, entries, asset.Preserved); err != nil {
		return format.Artifact{}, err
	}
	if formatID == V2 {
		for _, key := range v3OnlyKeys {
			delete(body, key)
		}
	}
	card, err := marshalCard(formatID, body)
	if err != nil {
		return format.Artifact{}, err
	}
	copies := []cardCopy{{keyword: chunkName(formatID), card: card}}
	if formatID != V2 {
		fallback, err := marshalCard(V2, olderShape(body))
		if err != nil {
			return format.Artifact{}, err
		}
		copies = append(copies, cardCopy{keyword: chunkName(V2), card: fallback})
	}
	if picture == nil {
		if formatID != V2 {
			card, err = withLegacyFields(card, body)
			if err != nil {
				return format.Artifact{}, err
			}
		}
		return format.Artifact{Body: card, MediaType: "application/json", Extension: ".json"}, nil
	}
	written, err := embedCardsInPNG(picture.Data, copies)
	if err != nil {
		return format.Artifact{}, err
	}
	return format.Artifact{Body: written, MediaType: "image/png", Extension: ".png"}, nil
}

// olderShape is the body with the keys the v3 standard added taken out, which
// is what the v2 copy of a card holds.
func olderShape(body map[string]json.RawMessage) map[string]json.RawMessage {
	older := make(map[string]json.RawMessage, len(body))
	for key, value := range body {
		if !slices.Contains(v3OnlyKeys, key) {
			older[key] = value
		}
	}
	return older
}

// legacyFields are the six a card carried before any spec existed. A v3 JSON
// document repeats them at its top level for the same reason a v3 picture
// carries a v2 chunk: a reader that knows only the older shape looks there.
var legacyFields = []string{
	"name", "description", "personality", "scenario", "first_mes", "mes_example",
}

func withLegacyFields(card []byte, body map[string]json.RawMessage) ([]byte, error) {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(card, &document); err != nil {
		return nil, fmt.Errorf("read the written card: %w", err)
	}
	for _, field := range legacyFields {
		if value, written := body[field]; written {
			document[field] = value
		}
	}
	written, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("write the card: %w", err)
	}
	return written, nil
}

// writeCharX writes the archive form. A v3 card sits beside the pictures it
// names, each one a file rather than a string inside the card.
func writeCharX(asset format.ExportAsset) (format.Artifact, error) {
	body, entries := cardFields(asset, CharX)
	files, records := archivedAssets(asset)
	body["assets"] = records
	if err := RestorePreserved(body, entries, asset.Preserved); err != nil {
		return format.Artifact{}, err
	}
	card, err := marshalCard(V3, body)
	if err != nil {
		return format.Artifact{}, err
	}

	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	if err := writeArchiveFile(archive, "card.json", card); err != nil {
		return format.Artifact{}, err
	}
	for _, file := range files {
		if err := writeArchiveFile(archive, file.path, file.data); err != nil {
			return format.Artifact{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return format.Artifact{}, fmt.Errorf("close CharX: %w", err)
	}
	return format.Artifact{
		Body: output.Bytes(), MediaType: "application/zip", Extension: ".charx",
	}, nil
}

func writeArchiveFile(archive *zip.Writer, name string, data []byte) error {
	destination, err := archive.Create(name)
	if err != nil {
		return fmt.Errorf("create CharX entry %q: %w", name, err)
	}
	if _, err := destination.Write(data); err != nil {
		return fmt.Errorf("write CharX entry %q: %w", name, err)
	}
	return nil
}

func marshalCard(formatID string, body map[string]json.RawMessage) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("write the card body: %w", err)
	}
	version := v3SpecVersion
	if formatID == V2 {
		version = v2SpecVersion
	}
	card, err := json.Marshal(map[string]json.RawMessage{
		"spec":         keys.Must(formatID),
		"spec_version": keys.Must(version),
		"data":         data,
	})
	if err != nil {
		return nil, fmt.Errorf("write the card: %w", err)
	}
	return card, nil
}

// chunkName is the PNG text keyword each standard puts its card under.
func chunkName(formatID string) string {
	if formatID == V2 {
		return "chara"
	}
	return "ccv3"
}

// cardFields writes the card body one key at a time out of the asset's roles,
// and returns the book's entries in the order they were written so preserved
// keys can find their way back to them.
func cardFields(
	asset format.ExportAsset,
	formatID string,
) (map[string]json.RawMessage, []block.Entry) {
	greetings := textItems(asset, block.RoleGreetings)
	first := ""
	if len(greetings) > 0 {
		first = greetings[0].Text
	}
	body := map[string]json.RawMessage{
		"name":                      keys.Must(asset.Header.Name),
		"description":               keys.Must(asset.Text(block.RoleDescription)),
		"personality":               keys.Must(asset.Text(block.RolePersonality)),
		"scenario":                  keys.Must(asset.Text(block.RoleScenario)),
		"first_mes":                 keys.Must(first),
		"alternate_greetings":       keys.Must(textsOf(greetings[min(1, len(greetings)):])),
		"mes_example":               keys.Must(dialogueText(asset)),
		"system_prompt":             keys.Must(asset.Text(block.RoleSystemPrompt)),
		"post_history_instructions": keys.Must(asset.Text(block.RolePostHistoryInstructions)),
		"creator_notes":             keys.Must(asset.Text(block.RoleCreatorNotes)),
		"creator":                   keys.Must(asset.Header.CreditedAuthor),
		"character_version":         keys.Must(asset.Header.AssetVersion),
	}
	if formatID != V2 {
		body["nickname"] = keys.Must(asset.Header.Nickname)
		body["group_only_greetings"] = keys.Must(
			textsOf(textItems(asset, block.RoleGroupGreetings)),
		)
	}
	entries := bookEntries(asset)
	if len(entries) > 0 {
		body[bookKey] = writtenBook(entries)
	}
	return body, entries
}

func textItems(asset format.ExportAsset, role block.Role) []block.TextItem {
	content, ok := asset.Content(role)
	if !ok {
		return nil
	}
	set, isSet := content.(block.TextSet)
	if !isSet {
		return nil
	}
	return set.Texts
}

func textsOf(items []block.TextItem) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}
	return texts
}

// dialogueText writes one line per turn; multiline turns cannot round-trip.
func dialogueText(asset format.ExportAsset) string {
	content, ok := asset.Content(block.RoleExampleDialogue)
	if !ok {
		return ""
	}
	sample, isSample := content.(block.DialogueSample)
	if !isSample || len(sample.Turns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(sample.Turns)+1)
	lines = append(lines, dialogueStart)
	for _, turn := range sample.Turns {
		if turn.Speaker == "" {
			lines = append(lines, turn.Text)
			continue
		}
		lines = append(lines, turn.Speaker+": "+turn.Text)
	}
	return strings.Join(lines, "\n")
}

func bookEntries(asset format.ExportAsset) []block.Entry {
	content, ok := asset.Content(block.RoleLorebookEntries)
	if !ok {
		return nil
	}
	table, isTable := content.(block.EntryTable)
	if !isTable {
		return nil
	}
	return table.Entries
}

// writtenBook writes the entries a card carries. Everything a book held that
// the entry table has no place for comes back afterwards, from preservation.
func writtenBook(entries []block.Entry) json.RawMessage {
	return keys.Must(map[string]any{"entries": book.Write(entries)})
}

// cardAssetRecord is one entry of a v3 card's asset list.
type cardAssetRecord struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
	Name string `json:"name"`
	Ext  string `json:"ext"`
}

// archivedFile is one picture written beside the card in a CharX archive.
type archivedFile struct {
	path string
	data []byte
}

// inlineAssets writes the picture list a JSON or PNG card carries, each image
// as a string inside the card itself. The card's own picture points at the
// container it is embedded in rather than repeating it.
func inlineAssets(asset format.ExportAsset, embedded bool) json.RawMessage {
	records := make([]cardAssetRecord, 0)
	for _, picture := range exportedPictures(asset) {
		uri := dataURI(picture.media.MediaType, picture.media.Data)
		if picture.assetType == iconAssetType && embedded {
			uri = defaultAssetURI
		}
		records = append(records, cardAssetRecord{
			Type: picture.assetType, URI: uri, Name: picture.name,
			Ext: mediaExtension(picture.media.MediaType),
		})
	}
	return keys.Must(records)
}

// archivedAssets writes the picture list a CharX card carries and the files it
// points at.
func archivedAssets(asset format.ExportAsset) ([]archivedFile, json.RawMessage) {
	files := make([]archivedFile, 0)
	records := make([]cardAssetRecord, 0)
	taken := make(map[string]bool)
	for index, picture := range exportedPictures(asset) {
		extension := mediaExtension(picture.media.MediaType)
		entry := path.Join("assets", picture.assetType, fmt.Sprintf("%d.%s", index+1, extension))
		if taken[entry] {
			continue
		}
		taken[entry] = true
		files = append(files, archivedFile{path: entry, data: picture.media.Data})
		records = append(records, cardAssetRecord{
			Type: picture.assetType, URI: embeddedPrefix + entry,
			Name: picture.name, Ext: extension,
		})
	}
	return files, keys.Must(records)
}

const (
	iconAssetType       = "icon"
	emotionAssetType    = "emotion"
	galleryAssetType    = "x_gallery"
	mainIconAssetName   = "main"
	fallbackPictureName = "image"
)

// exportedPicture is one picture on its way into a card, with the asset type
// the standard files it under.
type exportedPicture struct {
	assetType string
	name      string
	media     format.ExportMedia
}

// exportedPictures is every picture a card carries, the asset's own first.
func exportedPictures(asset format.ExportAsset) []exportedPicture {
	pictures := make([]exportedPicture, 0)
	if asset.Cover != nil {
		pictures = append(pictures, exportedPicture{
			assetType: iconAssetType, name: mainIconAssetName, media: *asset.Cover,
		})
	}
	for _, role := range []struct {
		role      block.Role
		assetType string
	}{
		{block.RoleExpressions, emotionAssetType},
		{block.RoleGallery, galleryAssetType},
	} {
		for _, element := range asset.Elements {
			if element.Role != role.role {
				continue
			}
			set, isSet := element.Content.(block.ImageSet)
			if !isSet {
				continue
			}
			for index, image := range set.Images {
				found, held := asset.Images[image.MediaID]
				if !held {
					continue
				}
				pictures = append(pictures, exportedPicture{
					assetType: role.assetType,
					name:      pictureName(image.Name, index),
					media:     found,
				})
			}
		}
	}
	return pictures
}

func pictureName(name string, index int) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s-%d", fallbackPictureName, index+1)
}

// embeddablePicture is the asset's own picture where a card can be written
// inside it. A card goes into a PNG and nothing else, so an asset whose cover
// is another kind of image downloads as a JSON document.
func embeddablePicture(asset format.ExportAsset) *format.ExportMedia {
	if asset.Cover == nil || !bytes.HasPrefix(asset.Cover.Data, pngSignature) {
		return nil
	}
	return asset.Cover
}

func dataURI(mediaType string, data []byte) string {
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
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

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// cardCopy is one card and the PNG text keyword it goes under.
type cardCopy struct {
	keyword string
	card    []byte
}

// embedCardsInPNG writes the cards into a copy of the picture, replacing any
// card chunk already there and leaving every other chunk exactly as it was.
func embedCardsInPNG(source []byte, copies []cardCopy) ([]byte, error) {
	if !bytes.HasPrefix(source, pngSignature) {
		return nil, errors.New("the asset's picture is not a PNG")
	}
	var output bytes.Buffer
	output.Write(source[:8])
	inserted := false
	err := visitPNGChunks(source, func(kind string, data, raw []byte) error {
		if kind == "IEND" && !inserted {
			for _, copied := range copies {
				encoded := base64.StdEncoding.EncodeToString(copied.card)
				output.Write(makePNGChunk("tEXt", slices.Concat(
					[]byte(copied.keyword), []byte{0}, []byte(encoded),
				)))
			}
			inserted = true
		}
		if !isCardChunk(kind, data) {
			output.Write(raw)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, errors.New("the asset's picture has no IEND chunk")
	}
	return output.Bytes(), nil
}

func isCardChunk(kind string, data []byte) bool {
	if kind != "tEXt" {
		return false
	}
	keyword, _, found := bytes.Cut(data, []byte{0})
	return found && (string(keyword) == "chara" || string(keyword) == "ccv3")
}

func makePNGChunk(kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(data)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(chunk[4:8+len(data)]))
	return chunk
}

func visitPNGChunks(source []byte, visit func(kind string, data, raw []byte) error) error {
	if !bytes.HasPrefix(source, pngSignature) {
		return errors.New("file is not a PNG")
	}
	for offset := 8; offset < len(source); {
		if offset+12 > len(source) {
			return fmt.Errorf("PNG chunk at byte %d is truncated", offset)
		}
		length := int(binary.BigEndian.Uint32(source[offset : offset+4]))
		end := offset + 12 + length
		if length < 0 || end < offset || end > len(source) {
			return fmt.Errorf("PNG chunk at byte %d exceeds the file", offset)
		}
		if err := visit(
			string(source[offset+4:offset+8]),
			source[offset+8:offset+8+length],
			source[offset:end],
		); err != nil {
			return err
		}
		offset = end
	}
	return nil
}
