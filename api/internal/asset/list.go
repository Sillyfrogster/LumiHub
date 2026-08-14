package asset

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
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
	Facets      []format.Facet
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
	ID         uuid.UUID
	Name       string
	Creator    string
	Kind       string
	IsNSFW     bool
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
			Blurb:               row.Blurb,
			Tags:                row.Tags,
			IsNSFW:              row.IsNsfw,
			Discovery:           Discovery(row.Discovery),
			CurrentRevisionID:   uuidFromPgtype(row.CurrentRevisionID),
			CreatedAt:           timeFromPgtype(row.CreatedAt),
		}
	}
	return out, nil
}

func browseAssets(
	ctx context.Context,
	q db.DBTX,
	registry *format.Registry,
	f ListFilter,
	visibility ContentVisibility,
) (BrowsePage, error) {
	queries := db.New(q)
	search := parseBrowseQuery(f.Query)
	definitions := registry.BrowseDefinitions()
	facetDefinitions := browseFacetDefinitions(definitions, f.Kind, f.Platform)
	f.Facets = declaredFacetSelections(f.Facets, facetDefinitions)
	platforms := browsePlatforms(definitions)
	platform := ""
	if f.Platform != nil {
		platform = normalizeBrowseText(*f.Platform)
	}
	formats := formatsForPlatform(definitions, platform)
	facetKeys, facetValues := facetPairs(f.Facets)
	creatorID, creatorShowsNSFW, ownProfile := profileListingValues(f.Profile)
	params := db.BrowseAssetsParams{
		Kind: f.Kind, NsfwVisibility: string(visibility),
		CreatorID: uuidToNullable(creatorID), CreatorAllowsNsfw: creatorShowsNSFW,
		OwnProfile: ownProfile,
		SearchText: search.Text, Author: search.Author, Tags: search.Tags,
		Platform: platform, Formats: formats,
		FacetKeys: facetKeys, FacetValues: facetValues,
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
		item := BrowseItem{
			ID: uuidFromPgtype(row.ID), Name: row.Name, Creator: row.Creator,
			Kind: row.Kind, IsNSFW: row.IsNsfw,
		}
		if ownProfile {
			switch {
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
			item.Cover = &BrowseCover{
				URL: variantURL(
					uuidFromPgtype(row.CoverID), "grid",
					visibility != ContentShown && row.IsNsfw,
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
		FacetKeys: facetKeys, FacetValues: facetValues,
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
				FacetKeys: facetKeys, FacetValues: facetValues,
			},
		)
		if err != nil {
			return BrowsePage{}, fmt.Errorf("count suppressed browse assets: %w", err)
		}
		page.Suppressed = int(suppressed)
	}
	page.Platforms, err = countedPlatforms(ctx, queries, countParams, definitions, platforms, platform)
	if err != nil {
		return BrowsePage{}, err
	}
	page.Facets, err = countedFacets(ctx, queries, countParams, f.Facets, facetDefinitions)
	if err != nil {
		return BrowsePage{}, err
	}
	if page.Total == 0 {
		if page.Suppressed > 0 {
			page.EmptyState = "suppressed"
		} else if f.Kind == "" && search.Text == "" && search.Author == "" &&
			len(search.Tags) == 0 && f.Platform == nil && len(f.Facets) == 0 {
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

func browsePlatforms(definitions []format.RegisteredBrowseDefinition) []format.BrowseOption {
	byValue := map[string]format.BrowseOption{
		"raw":       {Value: "raw", Label: "Original file"},
		"lumiverse": {Value: "lumiverse", Label: "Lumiverse"},
	}
	for _, registered := range definitions {
		for _, target := range registered.Definition.ExportTargets {
			value := normalizeBrowseText(target.Value)
			if value != "" {
				byValue[value] = format.BrowseOption{Value: value, Label: target.Label}
			}
		}
	}
	options := make([]format.BrowseOption, 0, len(byValue))
	for _, option := range byValue {
		options = append(options, option)
	}
	slices.SortFunc(options, func(a, b format.BrowseOption) int {
		if a.Value == "raw" {
			return -1
		}
		if b.Value == "raw" {
			return 1
		}
		return strings.Compare(a.Label, b.Label)
	})
	return options
}

func formatsForPlatform(
	definitions []format.RegisteredBrowseDefinition,
	platform string,
) []string {
	if platform == "" || platform == "raw" {
		return nil
	}
	var formats []string
	for _, registered := range definitions {
		for _, target := range registered.Definition.ExportTargets {
			if normalizeBrowseText(target.Value) == platform {
				formats = append(formats, registered.Format)
				break
			}
		}
	}
	return formats
}

func browseFacetDefinitions(
	definitions []format.RegisteredBrowseDefinition,
	kind string,
	platform *string,
) []format.BrowseFacet {
	if kind == "" {
		return nil
	}
	selectedPlatform := ""
	if platform != nil {
		selectedPlatform = normalizeBrowseText(*platform)
	}
	byKey := make(map[string]format.BrowseFacet)
	order := make([]string, 0)
	for _, registered := range definitions {
		if registered.Definition.Kind != kind {
			continue
		}
		for _, facet := range registered.Definition.Facets {
			if len(facet.Platforms) > 0 && !slices.ContainsFunc(facet.Platforms, func(value string) bool {
				return normalizeBrowseText(value) == selectedPlatform
			}) {
				continue
			}
			current, exists := byKey[facet.Key]
			if !exists {
				current = format.BrowseFacet{Key: facet.Key, Label: facet.Label}
				order = append(order, facet.Key)
			}
			for _, option := range facet.Options {
				if !slices.ContainsFunc(current.Options, func(existing format.BrowseOption) bool {
					return existing.Value == option.Value
				}) {
					current.Options = append(current.Options, option)
				}
			}
			byKey[facet.Key] = current
		}
	}
	result := make([]format.BrowseFacet, 0, len(order))
	for _, key := range order {
		result = append(result, byKey[key])
	}
	return result
}

func declaredFacetSelections(
	requested []format.Facet,
	definitions []format.BrowseFacet,
) []format.Facet {
	var selected []format.Facet
	for _, request := range requested {
		for _, definition := range definitions {
			if request.Key != definition.Key {
				continue
			}
			if slices.ContainsFunc(definition.Options, func(option format.BrowseOption) bool {
				return request.Value == option.Value
			}) {
				selected = append(selected, request)
			}
		}
	}
	return selected
}

func countedPlatforms(
	ctx context.Context,
	queries *db.Queries,
	base db.CountBrowseAssetsParams,
	definitions []format.RegisteredBrowseDefinition,
	options []format.BrowseOption,
	selected string,
) ([]BrowseOption, error) {
	result := make([]BrowseOption, 0, len(options))
	for _, option := range options {
		params := base
		params.Platform = option.Value
		params.Formats = formatsForPlatform(definitions, option.Value)
		count, err := queries.CountBrowseAssets(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("count platform %s: %w", option.Value, err)
		}
		result = append(result, BrowseOption{
			Value: option.Value, Label: option.Label, Count: int(count), Selected: option.Value == selected,
		})
	}
	return result, nil
}

func countedFacets(
	ctx context.Context,
	queries *db.Queries,
	base db.CountBrowseAssetsParams,
	selected []format.Facet,
	definitions []format.BrowseFacet,
) ([]BrowseFacet, error) {
	result := make([]BrowseFacet, 0, len(definitions))
	for _, definition := range definitions {
		group := BrowseFacet{Key: definition.Key, Label: definition.Label}
		for _, option := range definition.Options {
			facet := format.Facet{Key: definition.Key, Value: option.Value}
			candidate := slices.Clone(selected)
			if !slices.Contains(selected, facet) {
				candidate = append(candidate, facet)
			}
			keys, values := facetPairs(candidate)
			params := base
			params.FacetKeys, params.FacetValues = keys, values
			count, err := queries.CountBrowseAssets(ctx, params)
			if err != nil {
				return nil, fmt.Errorf("count facet %s=%s: %w", definition.Key, option.Value, err)
			}
			group.Options = append(group.Options, BrowseOption{
				Value: option.Value, Label: option.Label, Count: int(count),
				Selected: slices.Contains(selected, facet),
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
		PassthroughPlatform: textToPointer(row.PassthroughPlatform),
		Name:                row.Name, Blurb: row.Blurb, Tags: row.Tags,
		IsNSFW: row.IsNsfw, Discovery: Discovery(row.Discovery),
		CurrentRevisionID: uuidFromPgtype(row.CurrentRevisionID),
		CreatedAt:         timeFromPgtype(row.CreatedAt),
	}, nil
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
