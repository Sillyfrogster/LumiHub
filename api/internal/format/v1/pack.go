package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	packformat "github.com/Sillyfrogster/Illarin/api/internal/format/pack"
	"github.com/Sillyfrogster/Illarin/api/internal/media"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
)

func readPack(ctx context.Context, row PackRow) (readResult, error) {
	root := map[string]json.RawMessage{
		"packName":   mustJSON(row.Common.Name),
		"packAuthor": mustJSON(row.Author),
		"version":    mustJSON(row.Version),
		"lumiaItems": row.LumiaItems,
		"loomItems":  json.RawMessage(`[]`),
	}
	parsed, err := parseRoot(ctx, packformat.Module{}, probe.JSON, root, int64(len(row.LumiaItems)))
	if err != nil {
		return readResult{}, fmt.Errorf("v1 pack: %w", err)
	}
	parsed.Format = ID
	parsed.Tags = nil
	answer := row.Common.IsNSFW
	created := row.Common.CreatedAt
	parsed.IsNSFW = &answer
	parsed.CreatedAt = &created
	parsed.Header = format.Header{
		Name: row.Common.Name, Blurb: row.Common.Description,
		AssetVersion: strconv.Itoa(row.Version), CreditedAuthor: row.Author,
	}
	parsed.Remainder = stripRemainderKeys(
		parsed.Remainder, packformat.ID,
		"packName", "packAuthor", "version", "loomItems", "packExtras", "loomTools",
	)

	external := make([]ExternalMedia, 0)
	if row.CoverURL != "" {
		external = append(external, ExternalMedia{Owner: ExternalCover, Role: media.Avatar, URL: row.CoverURL})
	}
	list, ok := packRecords(parsed.Elements)
	if !ok {
		return readResult{}, fmt.Errorf("v1 pack: the pack reader produced no records")
	}
	var items []struct {
		AvatarURL string `json:"avatarUrl"`
	}
	if json.Unmarshal(row.LumiaItems, &items) != nil || len(items) != len(list.Records) {
		return readResult{}, fmt.Errorf("v1 pack: item count did not reconcile")
	}
	for i, item := range items {
		if item.AvatarURL != "" {
			external = append(external, ExternalMedia{
				Owner: ExternalPackItem, OwnerID: list.Records[i].ID,
				Role: media.PackItem, URL: item.AvatarURL,
			})
		}
	}
	return readResult{parsed: parsed, externalMedia: external}, nil
}

func packRecords(elements []block.Element) (block.RecordList, bool) {
	for _, element := range elements {
		if element.Role == block.RolePackItems {
			list, ok := element.Content.(block.RecordList)
			return list, ok
		}
	}
	return block.RecordList{}, false
}
