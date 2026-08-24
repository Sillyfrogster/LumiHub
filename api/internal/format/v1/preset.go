package v1

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/preset"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/Sillyfrogster/Illarin/api/internal/protected"
	"github.com/google/uuid"
)

func readPreset(ctx context.Context, row PresetRow) (readResult, error) {
	parsed, err := parseJSON(ctx, preset.LumiverseModule{}, row.Payload)
	if err != nil {
		return readResult{}, fmt.Errorf("v1 preset payload: %w", err)
	}
	if err := activateCurrentSealedPrompts(&parsed, row); err != nil {
		return readResult{}, err
	}
	parsed.Format = ID
	parsed.Tags = append([]string(nil), row.Common.Tags...)
	answer := row.Common.IsNSFW
	created := row.Common.CreatedAt
	parsed.IsNSFW = &answer
	parsed.CreatedAt = &created
	parsed.Header = format.Header{
		Name: row.Common.Name, AssetVersion: row.LatestVersion,
	}
	if len([]rune(row.Common.Description)) <= format.MaxBlurbRunes {
		parsed.Header.Blurb = row.Common.Description
	} else {
		parsed.Elements = append(parsed.Elements, block.Element{
			ID: uuid.New(), Type: block.TypeProse,
			Content: block.Prose{Text: row.Common.Description},
		})
	}
	parsed.Remainder = stripRemainderKeys(
		parsed.Remainder, preset.LumiverseID, "name", "description", "presetVersion",
	)

	changes := make([]block.TextItem, 0, len(row.Versions))
	records := make([]PreservedRecord, 0, len(row.Versions))
	for _, version := range row.Versions {
		if version.Changelog != "" {
			changes = append(changes, block.TextItem{
				ID: block.NewItemID(), Name: version.Version, Text: version.Changelog,
			})
		}
		records = append(records, PreservedRecord{
			Table: "preset_versions", SourceID: strconv.FormatInt(version.ID, 10),
			AssetID: row.Common.ID, OwnerID: row.Common.OwnerID, Payload: mustJSON(version),
		})
	}
	if len(changes) > 0 {
		parsed.Elements = append(parsed.Elements, block.Element{
			ID: uuid.New(), Type: block.TypeTextSet,
			Content: block.TextSet{Texts: changes},
		})
	}
	sealed := make([]SealedBlock, len(row.SealedBlocks))
	for i, source := range row.SealedBlocks {
		sealed[i] = SealedBlock{
			ID: source.ID, AssetID: row.Common.ID, OwnerID: row.Common.OwnerID,
			Version: source.Version, Key: source.Key, Content: source.Content, SHA256: source.SHA256,
			CreatedBy: source.CreatedBy, CreatedAt: source.CreatedAt, UpdatedAt: source.UpdatedAt,
		}
	}
	return readResult{parsed: parsed, preservedRecords: records, sealedBlocks: sealed}, nil
}

func activateCurrentSealedPrompts(parsed *format.Parsed, row PresetRow) error {
	seenPlaceholderKeys := map[string]bool{}
	for elementIndex := range parsed.Elements {
		element := &parsed.Elements[elementIndex]
		list, ok := element.Content.(block.PromptList)
		if !ok {
			continue
		}
		for fragmentIndex := range list.Fragments {
			fragment := &list.Fragments[fragmentIndex]
			key, placeholder := v1SealedPlaceholder(fragment.Text)
			if !placeholder {
				continue
			}
			if key == "" || key != strings.TrimSpace(key) || len([]rune(key)) > 256 {
				return fmt.Errorf("current sealed prompt has an invalid source key")
			}
			if seenPlaceholderKeys[key] {
				return fmt.Errorf("current sealed prompt source key appears more than once")
			}
			seenPlaceholderKeys[key] = true
			fragment.Protected = true
			fragment.Text = ""
			parsed.Protected.Prompts = append(parsed.Protected.Prompts, format.ProtectedPrompt{
				FragmentID: fragment.ID, SourceKey: key, ReuseExisting: true,
			})
		}
		element.Content = list
	}
	if len(parsed.Protected.Prompts) > 0 {
		parsed.Protected.Apps = []string{protected.AppLumiverse}
	}

	latestVersionSealedRows := make(map[string][]SealedBlockRow)
	for _, sealed := range row.SealedBlocks {
		if sealed.Version == nil {
			if row.LatestVersion != "" {
				continue
			}
		} else if *sealed.Version != row.LatestVersion {
			continue
		}
		latestVersionSealedRows[sealed.Key] = append(latestVersionSealedRows[sealed.Key], sealed)
	}

	for index := range parsed.Protected.Prompts {
		prompt := &parsed.Protected.Prompts[index]
		matches := latestVersionSealedRows[prompt.SourceKey]
		if len(matches) != 1 {
			return fmt.Errorf("current sealed prompt needs exactly one preserved source row")
		}
		sealed := matches[0]
		if !prompt.ReuseExisting && prompt.Text != sealed.Content {
			return fmt.Errorf("current sealed prompt does not match its preserved source row")
		}
		if strings.TrimSpace(sealed.Content) == "{{presetBlock::"+prompt.SourceKey+"}}" {
			return fmt.Errorf("current sealed prompt source contains its placeholder")
		}
		storedDigest, err := hex.DecodeString(sealed.SHA256)
		if err != nil || len(storedDigest) != sha256.Size {
			return fmt.Errorf("current sealed prompt source has an invalid digest")
		}
		digest := sha256.Sum256([]byte(sealed.Content))
		if !bytes.Equal(storedDigest, digest[:]) {
			return fmt.Errorf("current sealed prompt source does not match its digest")
		}
		prompt.Text = sealed.Content
		prompt.ReuseExisting = false
	}
	return nil
}

func v1SealedPlaceholder(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	const opening = "{{presetBlock::"
	if !strings.HasPrefix(trimmed, opening) || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(trimmed, opening), "}}"), true
}

func parseJSON(ctx context.Context, reader format.Reader, body json.RawMessage) (format.Parsed, error) {
	var root map[string]json.RawMessage
	if json.Unmarshal(body, &root) != nil {
		return format.Parsed{}, format.MalformedInput(fmt.Errorf("expected an object"))
	}
	return parseRoot(ctx, reader, probe.JSON, root, int64(len(body)))
}

func parseRoot(
	ctx context.Context,
	reader format.Reader,
	container probe.Container,
	root map[string]json.RawMessage,
	byteSize int64,
) (format.Parsed, error) {
	file := probe.Inspection{Container: container, Payloads: []probe.Payload{{
		ID: 1, Locator: probe.Locator{Container: container}, Root: root, ByteSize: byteSize,
	}}}
	claim, claimed := reader.Claim(file)
	if !claimed {
		return format.Parsed{}, format.MalformedInput(fmt.Errorf("the payload did not match %s", reader.ID()))
	}
	return reader.Parse(ctx, file, claim)
}

func stripRemainderKeys(rows []format.Remainder, namespace string, names ...string) []format.Remainder {
	stripped := make([]format.Remainder, 0, len(rows))
	for _, row := range rows {
		if row.Owner != format.OwnerAsset || row.Namespace != namespace {
			stripped = append(stripped, row)
			continue
		}
		var body map[string]json.RawMessage
		if json.Unmarshal(row.Payload, &body) != nil {
			stripped = append(stripped, row)
			continue
		}
		for _, name := range names {
			delete(body, name)
		}
		if len(body) > 0 {
			row.Payload = mustJSON(body)
			stripped = append(stripped, row)
		}
	}
	return stripped
}
