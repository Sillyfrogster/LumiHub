package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/media"
	"github.com/google/uuid"
)

const (
	ID            = format.OriginV1
	CharacterKind = "character"
	LorebookKind  = "lorebook"
	PresetKind    = "preset"
	ThemeKind     = "theme"
	PackKind      = "pack"
)

type CommonRow struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Description string
	ImagePath   string
	Downloads   int
	Views       int
	Tags        []string
	IsNSFW      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CharacterRow struct {
	Common                  CommonRow
	Nickname                string
	Personality             string
	Scenario                string
	FirstMessage            string
	AlternateGreetings      json.RawMessage
	GroupOnlyGreetings      json.RawMessage
	ExampleDialogue         string
	Creator                 string
	CreatorNotes            string
	CharacterVersion        string
	SystemPrompt            string
	PostHistoryInstructions string
	Tagline                 string
	CharacterBook           json.RawMessage
	Extensions              json.RawMessage
	Assets                  json.RawMessage
	CreationDate            *int64
	ModificationDate        *int64
	Images                  []CharacterImageRow
}

type CharacterRecovery struct {
	AlternateGreeting string
}

func (row CharacterRow) common() CommonRow { return row.Common }

type LorebookRow struct {
	Common  CommonRow
	Creator string
	Entries json.RawMessage
}

func (row LorebookRow) common() CommonRow { return row.Common }

type PresetRow struct {
	Common        CommonRow
	Payload       json.RawMessage
	LatestVersion string
	Versions      []PresetVersionRow
	SealedBlocks  []SealedBlockRow
}

func (row PresetRow) common() CommonRow { return row.Common }

type ThemeRow struct {
	Common        CommonRow
	Colors        json.RawMessage
	Config        json.RawMessage
	CustomCSS     string
	AssetBundleID string
	Bundle        []byte
}

func (row ThemeRow) common() CommonRow { return row.Common }

type PackRow struct {
	Common     CommonRow
	Author     string
	Version    int
	CoverURL   string
	PackExtras json.RawMessage
	LumiaItems json.RawMessage
	LoomItems  json.RawMessage
	LoomTools  json.RawMessage
}

func (row PackRow) common() CommonRow { return row.Common }

type PresetVersionRow struct {
	ID               int64           `json:"id"`
	Version          string          `json:"version"`
	Changelog        string          `json:"changelog"`
	Snapshot         json.RawMessage `json:"snapshot"`
	BlocksAdded      int             `json:"blocks_added"`
	BlocksRemoved    int             `json:"blocks_removed"`
	VariablesAdded   int             `json:"variables_added"`
	VariablesRemoved int             `json:"variables_removed"`
	BlockCount       int             `json:"block_count"`
	VariableCount    int             `json:"variable_count"`
	CreatedBy        *uuid.UUID      `json:"created_by"`
	CreatedAt        time.Time       `json:"created_at"`
}

type SealedBlockRow struct {
	ID        uuid.UUID
	Version   *string
	Key       string
	Content   string
	SHA256    string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Row interface {
	common() CommonRow
}

// Common returns the columns every kind of v1 row shares.
func Common(row Row) CommonRow { return row.common() }

type Result struct {
	AssetID           uuid.UUID
	OwnerID           uuid.UUID
	OriginFormat      string
	CreatedAt         time.Time
	ContentUpdatedAt  time.Time
	Parsed            format.Parsed
	Cover             *SourceMedia
	Media             []SourceMedia
	PreservedRecords  []PreservedRecord
	SealedBlocks      []SealedBlock
	ExternalMedia     []ExternalMedia
	Legacy            LegacyRecord
	ContentGeneration int
	Events            []Event
}

// LegacyRecord is v1's own bookkeeping for one row, frozen, and holds no favourite count because the favourites themselves migrate as rows.
type LegacyRecord struct {
	Downloads int
	Views     int
	UpdatedAt time.Time
}

type EventKind string

const (
	RecoveredAlternateGreeting EventKind = "recovered_alternate_greeting"
	RecoveredGalleryNames      EventKind = "recovered_gallery_names"
	GalleryAssetsMismatch      EventKind = "gallery_assets_mismatch"
	MissingThemeStatusColors   EventKind = "missing_theme_status_colors"
)

type Event struct {
	Kind  EventKind
	Count int
}

type ExternalOwner string

const (
	ExternalCover    ExternalOwner = "cover"
	ExternalPackItem ExternalOwner = "pack_item"
)

type ExternalMedia struct {
	Owner   ExternalOwner
	OwnerID uuid.UUID
	Role    media.Role
	URL     string
}

type PreservedRecord struct {
	Table    string
	SourceID string
	AssetID  uuid.UUID
	OwnerID  uuid.UUID
	Payload  json.RawMessage
}

type SealedBlock struct {
	ID        uuid.UUID
	AssetID   uuid.UUID
	OwnerID   uuid.UUID
	Version   *string
	Key       string
	Content   string
	SHA256    string
	CreatedBy *uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CharacterImageRow struct {
	ID        uuid.UUID
	Type      string
	Label     string
	Path      string
	MediaType string
	ByteSize  int
	Position  int
}

type SourceMedia struct {
	SourceID  uuid.UUID
	MediaID   uuid.UUID
	Role      media.Role
	Path      string
	MediaType string
	ByteSize  int
	Name      string
	Position  int
}

type RecoveryAllowlist map[uuid.UUID]CharacterRecovery

type Module struct {
	Recoveries RecoveryAllowlist
}

func (Module) ID() string { return ID }

func (Module) Declaration() format.Declaration {
	roles := make(map[block.Role]format.DirectionalRoleSupport)
	for _, role := range block.Roles() {
		roles[role] = format.DirectionalRoleSupport{
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportNone},
		}
	}
	return format.Declaration{
		ID: ID, Kinds: []string{CharacterKind, LorebookKind, PresetKind, ThemeKind, PackKind},
		Input: format.InputDatabaseRow, Columns: v1Columns(), Anomalies: v1Anomalies(),
		Direction: format.Direction{Read: true}, Roles: roles,
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		Preservation:  format.PreservationDeclaration{Body: ID},
		TestedOrigins: []string{ID},
	}
}

func v1Anomalies() []format.AnomalyDeclaration {
	return []format.AnomalyDeclaration{
		{Kind: "missing_theme_status_colors", Disposition: format.AnomalyTolerated, Reason: "the required palette remains complete"},
		{Kind: "missing_cover_file", Disposition: format.AnomalyTolerated, Reason: "the asset content remains readable"},
		{Kind: "slug_collision", Disposition: format.AnomalyTolerated, Reason: "the older asset keeps the redirect"},
		{Kind: "external_media_fetch_failed", Disposition: format.AnomalyTolerated, Reason: "the URL remains preserved for the migration ledger"},
		{Kind: "below_publish_floor", Disposition: format.AnomalyTolerated, Reason: "migration is not a publish"},
		{Kind: "orphan_source_file", Disposition: format.AnomalyTolerated, Reason: "bytes without an owned row are not migrated"},
		{Kind: "recovered_alternate_greeting", Disposition: format.AnomalyTolerated, Reason: "the one verified recovery is recorded"},
		{Kind: "gallery_assets_mismatch", Disposition: format.AnomalyTolerated, Reason: "the unmatched array remains preserved"},
		{Kind: "core_role_unreadable", Disposition: format.AnomalyFatal, Reason: "the asset cannot retain its defining content"},
		{Kind: "owner_unresolved", Disposition: format.AnomalyFatal, Reason: "an asset cannot migrate without its owner"},
		{Kind: "count_mismatch", Disposition: format.AnomalyFatal, Reason: "the migration corpus must reconcile exactly"},
		{Kind: "undeclared_column", Disposition: format.AnomalyFatal, Reason: "silent source-data loss is forbidden"},
	}
}

func (module Module) ReadDatabaseRow(ctx context.Context, row any) (format.Parsed, error) {
	source, ok := row.(Row)
	if !ok {
		return format.Parsed{}, fmt.Errorf("v1 database row %T is not supported", row)
	}
	result, err := module.Read(ctx, source)
	return result.Parsed, err
}

func (module Module) Read(ctx context.Context, row Row) (Result, error) {
	common := row.common()
	var read readResult
	var err error
	switch source := row.(type) {
	case CharacterRow:
		var recovery *CharacterRecovery
		if allowed, found := module.Recoveries[source.Common.ID]; found {
			copy := allowed
			recovery = &copy
		}
		read, err = readCharacter(source, recovery)
	case LorebookRow:
		read, err = readLorebook(source)
	case PresetRow:
		read, err = readPreset(ctx, source)
	case ThemeRow:
		read, err = readTheme(ctx, source)
	case PackRow:
		read, err = readPack(ctx, source)
	default:
		return Result{}, fmt.Errorf("v1 row %T is not supported", row)
	}
	if err != nil {
		return Result{}, err
	}
	cover := read.cover
	if cover == nil && common.ImagePath != "" && read.parsed.Kind != CharacterKind {
		cover = &SourceMedia{
			SourceID: common.ID, MediaID: uuid.New(), Role: media.Avatar, Path: common.ImagePath,
		}
	}
	return Result{
		AssetID: common.ID, OwnerID: common.OwnerID, OriginFormat: ID,
		CreatedAt: common.CreatedAt, ContentUpdatedAt: common.CreatedAt, Parsed: read.parsed,
		Cover: cover, Media: read.media,
		PreservedRecords: read.preservedRecords, SealedBlocks: read.sealedBlocks,
		ExternalMedia: read.externalMedia,
		Legacy: LegacyRecord{
			Downloads: common.Downloads, Views: common.Views, UpdatedAt: common.UpdatedAt,
		},
		ContentGeneration: 1, Events: read.events,
	}, nil
}

type readResult struct {
	parsed           format.Parsed
	cover            *SourceMedia
	media            []SourceMedia
	preservedRecords []PreservedRecord
	sealedBlocks     []SealedBlock
	externalMedia    []ExternalMedia
	events           []Event
}
