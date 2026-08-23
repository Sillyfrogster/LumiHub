package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/preset"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

func readPreset(ctx context.Context, row PresetRow) (readResult, error) {
	parsed, err := parseJSON(ctx, preset.LumiverseModule{}, row.Payload)
	if err != nil {
		return readResult{}, fmt.Errorf("v1 preset payload: %w", err)
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
