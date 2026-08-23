package asset

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
)

// ListFilter says which assets Browse should return. Empty fields mean no
// restriction.
type ListFilter struct {
	Kind        string
	Profile     *ProfileListingScope
	Platform    *string
	PlatformSet bool
	Tags        []string
	Facets      []FacetSelection
	Query       string
	Limit       int
	Before      *Cursor
}

type ProfileListingScope struct {
	CreatorID        uuid.UUID
	ViewerID         *uuid.UUID
	CreatorShowsNSFW bool
}

type Cursor struct {
	MadeAt time.Time
	ID     uuid.UUID // two assets can share a made date
}

type ContentVisibility string

const (
	ContentHidden  ContentVisibility = "hidden"
	ContentBlurred ContentVisibility = "blurred"
	ContentShown   ContentVisibility = "shown"
)

type BrowseCover struct {
	URL    string
	Width  int
	Height int
}

type BrowseItem struct {
	ID      uuid.UUID
	Name    string
	Creator string
	Kind    string
	// IsNSFW is nil on a draft whose creator has not answered the adult
	// content question, so a card states that rather than reading as a no.
	IsNSFW *bool
	// OwnerState is what the owner's own listing marks a card with, and is
	// empty everywhere else.
	OwnerState string
	Cover      *BrowseCover
	Withhold   *Withhold
}

type BrowsePage struct {
	Items      []BrowseItem
	Total      int
	Suppressed int
	Next       *Cursor
	EmptyState string
	Platforms  []BrowseOption
	Facets     []BrowseFacet
}

type BrowseOption struct {
	Value    string
	Label    string
	Count    int
	Selected bool
}

type BrowseFacet struct {
	Key     string
	Label   string
	Options []BrowseOption
}

type FacetSelection struct {
	Key   string
	Value string
}

// listAssets takes anything that runs a query, so a read does not have to open
// a transaction.
func listAssets(ctx context.Context, q db.DBTX, f ListFilter) ([]Asset, error) {
	queries := db.New(q)

	params := db.ListAssetsParams{
		Column1: f.Kind, Column2: f.PlatformSet, Format: valueOrEmpty(f.Platform),
		Column4: nullableTags(f.Tags), Limit: int32(f.Limit),
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
			ID:   uuidFromPgtype(row.ID),
			Kind: row.Kind,
			// An asset built from nothing has no file, so no origin format.
			Format:            row.Format.String,
			Name:              row.Name,
			Blurb:             row.Blurb,
			Tags:              row.Tags,
			IsNSFW:            &row.IsNsfw,
			Discovery:         Discovery(row.Discovery),
			CurrentRevisionID: uuidFromPgtype(row.CurrentRevisionID),
			CreatedAt:         timeFromPgtype(row.CreatedAt),
		}
	}
	return out, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *Service) browseAssets(
	ctx context.Context,
	f ListFilter,
	visibility ContentVisibility,
) (BrowsePage, error) {
	queries := db.New(s.pool)
	search := parseBrowseQuery(f.Query)
	facetDefinitions := block.Facets(f.Kind)
	chosen := declaredFacetSelections(f.Kind, f.Facets)
	platform := ""
	if f.Platform != nil {
		platform = normalizeBrowseText(*f.Platform)
	}
	formats := formatsForPlatform(platform)
	facetKeys, facetLows, facetHighs := facetRanges(chosen)
	creatorID, creatorShowsNSFW, ownProfile := profileListingValues(f.Profile)
	params := db.BrowseAssetsParams{
		Kind: f.Kind, NsfwVisibility: string(visibility),
		CreatorID: uuidToNullable(creatorID), CreatorAllowsNsfw: creatorShowsNSFW,
		OwnProfile: ownProfile,
		SearchText: search.Text, Author: search.Author, Tags: search.Tags,
		Platform: platform, Formats: formats,
		FacetKeys: facetKeys, FacetLows: facetLows, FacetHighs: facetHighs,
		PageSize: int32(f.Limit + 1),
	}
	if f.Before != nil {
		params.Before = timeToNullable(&f.Before.MadeAt)
		params.BeforeID = uuidToPgtype(f.Before.ID)
	}
	rows, err := queries.BrowseAssets(ctx, params)
	if err != nil {
		return BrowsePage{}, fmt.Errorf("browse assets: %w", err)
	}

	page := BrowsePage{Items: make([]BrowseItem, 0, min(len(rows), f.Limit))}
	for _, row := range rows[:min(len(rows), f.Limit)] {
		draft := Lifecycle(row.Lifecycle) == LifecycleDraft
		item := BrowseItem{
			ID: uuidFromPgtype(row.ID), Name: row.Name, Creator: row.Creator,
			Kind: row.Kind, IsNSFW: boolFromPgtype(row.IsNsfw),
		}
		if ownProfile {
			switch {
			case draft:
				item.OwnerState = "draft"
			case row.WithheldAt.Valid:
				item.OwnerState = "withheld"
				item.Withhold = &Withhold{
					Reason: row.WithheldReason.String,
					Actor:  row.WithheldBy.String,
					At:     row.WithheldAt.Time,
				}
			case row.Discovery == "unlisted":
				item.OwnerState = "unlisted"
			}
		}
		if row.CoverID.Valid && row.CoverWidth.Valid && row.CoverHeight.Valid {
			flagged := item.IsNSFW != nil && *item.IsNSFW
			item.Cover = &BrowseCover{
				URL: s.variantURL(
					uuidFromPgtype(row.CoverID), "grid",
					visibility != ContentShown && flagged, draft,
				),
				Width: int(row.CoverWidth.Int32), Height: int(row.CoverHeight.Int32),
			}
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > f.Limit && len(page.Items) > 0 {
		last := rows[f.Limit-1]
		page.Next = &Cursor{MadeAt: timeFromPgtype(last.CreatedAt), ID: uuidFromPgtype(last.ID)}
	}

	countParams := db.CountBrowseAssetsParams{
		Kind: f.Kind, NsfwVisibility: string(visibility),
		CreatorID: uuidToNullable(creatorID), CreatorAllowsNsfw: creatorShowsNSFW,
		OwnProfile: ownProfile,
		SearchText: search.Text, Author: search.Author, Tags: search.Tags,
		Platform: platform, Formats: formats,
		FacetKeys: facetKeys, FacetLows: facetLows, FacetHighs: facetHighs,
	}
	count, err := queries.CountBrowseAssets(ctx, countParams)
	if err != nil {
		return BrowsePage{}, fmt.Errorf("count browse assets: %w", err)
	}
	page.Total = int(count)
	if visibility == ContentHidden && !ownProfile {
		suppressed, err := queries.CountSuppressedBrowseAssets(
			ctx, db.CountSuppressedBrowseAssetsParams{
				Kind: f.Kind, SearchText: search.Text, Author: search.Author, Tags: search.Tags,
				CreatorID:         uuidToNullable(creatorID),
				CreatorAllowsNsfw: creatorShowsNSFW,
				Platform:          platform, Formats: formats,
				FacetKeys: facetKeys, FacetLows: facetLows, FacetHighs: facetHighs,
			},
		)
		if err != nil {
			return BrowsePage{}, fmt.Errorf("count suppressed browse assets: %w", err)
		}
		page.Suppressed = int(suppressed)
	}
	page.Platforms, err = countedPlatforms(ctx, queries, countParams, platform)
	if err != nil {
		return BrowsePage{}, err
	}
	page.Facets, err = countedFacets(ctx, queries, countParams, chosen, facetDefinitions)
	if err != nil {
		return BrowsePage{}, err
	}
	if page.Total == 0 {
		if page.Suppressed > 0 {
			page.EmptyState = "suppressed"
		} else if f.Kind == "" && search.Text == "" && search.Author == "" &&
			len(search.Tags) == 0 && f.Platform == nil && len(chosen) == 0 {
			page.EmptyState = "catalog"
		} else {
			page.EmptyState = "no_matches"
		}
	}
	return page, nil
}

func profileListingValues(scope *ProfileListingScope) (*uuid.UUID, bool, bool) {
	if scope == nil {
		return nil, false, false
	}
	ownedByViewer := scope.ViewerID != nil && *scope.ViewerID == scope.CreatorID
	return &scope.CreatorID, scope.CreatorShowsNSFW, ownedByViewer
}

func browsePlatforms() []format.App { return format.Apps() }

func formatsForPlatform(platform string) []string {
	for _, app := range format.Apps() {
		if normalizeBrowseText(app.ID) == platform {
			return app.Reads
		}
	}
	return nil
}

type chosenFacet struct {
	FacetSelection
	bucket block.Bucket
}

func declaredFacetSelections(kind string, requested []FacetSelection) []chosenFacet {
	chosen := make([]chosenFacet, 0, len(requested))
	for _, request := range requested {
		facet, known := block.FacetByKey(kind, request.Key)
		if !known {
			continue
		}
		bucket, offered := facet.Bucket(request.Value)
		if !offered {
			continue
		}
		if slices.ContainsFunc(chosen, func(already chosenFacet) bool {
			return already.FacetSelection == request
		}) {
			continue
		}
		chosen = append(chosen, chosenFacet{FacetSelection: request, bucket: bucket})
	}
	return chosen
}

func facetRanges(chosen []chosenFacet) (keys []string, lows, highs []int32) {
	keys = make([]string, 0, len(chosen))
	lows = make([]int32, 0, len(chosen))
	highs = make([]int32, 0, len(chosen))
	for _, one := range chosen {
		keys = append(keys, one.Key)
		lows = append(lows, int32(one.bucket.Min))
		highs = append(highs, int32(one.bucket.Max))
	}
	return keys, lows, highs
}

func countedPlatforms(
	ctx context.Context,
	queries *db.Queries,
	base db.CountBrowseAssetsParams,
	selected string,
) ([]BrowseOption, error) {
	apps := browsePlatforms()
	result := make([]BrowseOption, 0, len(apps))
	for _, app := range apps {
		params := base
		params.Platform = app.ID
		params.Formats = app.Reads
		count, err := queries.CountBrowseAssets(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("count platform %s: %w", app.ID, err)
		}
		result = append(result, BrowseOption{
			Value: app.ID, Label: app.Label, Count: int(count),
			Selected: app.ID == selected,
		})
	}
	return result, nil
}

func countedFacets(
	ctx context.Context,
	queries *db.Queries,
	base db.CountBrowseAssetsParams,
	chosen []chosenFacet,
	definitions []block.Facet,
) ([]BrowseFacet, error) {
	result := make([]BrowseFacet, 0, len(definitions))
	for _, definition := range definitions {
		group := BrowseFacet{Key: string(definition.Key), Label: definition.Label}
		for _, bucket := range definition.Buckets {
			choice := chosenFacet{
				FacetSelection: FacetSelection{
					Key: string(definition.Key), Value: bucket.Value,
				},
				bucket: bucket,
			}
			picked := slices.Contains(chosen, choice)
			candidate := slices.Clone(chosen)
			if !picked {
				candidate = append(candidate, choice)
			}
			keys, lows, highs := facetRanges(candidate)
			params := base
			params.FacetKeys, params.FacetLows, params.FacetHighs = keys, lows, highs
			count, err := queries.CountBrowseAssets(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("count facet %s=%s: %w", definition.Key, bucket.Value, err)
			}
			group.Options = append(group.Options, BrowseOption{
				Value: bucket.Value, Label: bucket.Label, Count: int(count),
				Selected: picked,
			})
		}
		result = append(result, group)
	}
	return result, nil
}

func assetByID(ctx context.Context, q db.DBTX, id uuid.UUID) (Asset, error) {
	row, err := db.New(q).AssetByID(ctx, uuidToPgtype(id))
	if err != nil {
		return Asset{}, fmt.Errorf("read asset: %w", err)
	}
	return Asset{
		ID: uuidFromPgtype(row.ID), Kind: row.Kind, Format: row.Format,
		OriginFormat: textToPointer(row.OriginFormat), AssetVersion: row.AssetVersion,
		CreditedAuthor: row.CreditedAuthor, Nickname: row.Nickname,
		Name: row.Name, Blurb: row.Blurb, Tags: row.Tags,
		IsNSFW: &row.IsNsfw, Discovery: Discovery(row.Discovery), Lifecycle: Lifecycle(row.Lifecycle),
		CurrentRevisionID: uuidFromPgtype(row.CurrentRevisionID),
		CreatedAt:         timeFromPgtype(row.CreatedAt),
	}, nil
}

func nullableTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	return tags
}
