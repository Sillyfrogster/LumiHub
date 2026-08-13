package asset

import (
	"context"
	"fmt"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

// ListFilter says which assets Browse should return. Empty fields mean no
// restriction.
type ListFilter struct {
	Kind        string
	Platform    *string
	PlatformSet bool
	Tags        []string
	Facets      []format.Facet
	Limit       int
	Before      *Cursor
}

type Cursor struct {
	MadeAt time.Time
	ID     uuid.UUID // two assets can share a made date
}

// listAssets takes anything that runs a query, so a read does not have to open
// a transaction.
func listAssets(ctx context.Context, q db.DBTX, f ListFilter) ([]Asset, error) {
	queries := db.New(q)
	facetKeys, facetValues := facetPairs(f.Facets)

	params := db.ListAssetsParams{
		Column1:             f.Kind,
		Column2:             f.PlatformSet,
		PassthroughPlatform: textToNullable(f.Platform),
		Column4:             nullableTags(f.Tags),
		Column5:             facetKeys,
		Column6:             facetValues,
		Limit:               int32(f.Limit),
	}
	if f.Before != nil {
		params.Before = timeToNullable(&f.Before.MadeAt)
		params.BeforeID = uuidToPgtype(f.Before.ID)
	}

	rows, err := queries.ListAssets(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}

	out := make([]Asset, len(rows))
	for i, row := range rows {
		out[i] = Asset{
			ID:                  uuidFromPgtype(row.ID),
			Kind:                row.Kind,
			Format:              row.Format,
			PassthroughPlatform: textToPointer(row.PassthroughPlatform),
			Name:                row.Name,
			Description:         row.Description,
			Tags:                row.Tags,
			IsNSFW:              row.IsNsfw,
			Discovery:           row.Discovery,
			CurrentRevisionID:   uuidFromPgtype(row.CurrentRevisionID),
			CreatedAt:           timeFromPgtype(row.CreatedAt),
		}
	}
	return out, nil
}

// facetPairs splits facets into parallel key and value arrays, dropping
// duplicates.
func facetPairs(facets []format.Facet) (keys, values []string) {
	seen := make(map[format.Facet]bool, len(facets))
	keys = make([]string, 0, len(facets))
	values = make([]string, 0, len(facets))
	for _, f := range facets {
		if seen[f] {
			continue
		}
		seen[f] = true
		keys = append(keys, f.Key)
		values = append(values, f.Value)
	}
	return keys, values
}

func nullableTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return tags
}
