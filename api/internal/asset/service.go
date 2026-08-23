package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	mediaproc "github.com/Sillyfrogster/Illarin/api/internal/media"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/Sillyfrogster/Illarin/api/internal/signing"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"
)

var (
	ErrNotFound         = errors.New("asset not found")
	ErrIngestNotFound   = errors.New("ingest operation not found")
	ErrInvalidDiscovery = errors.New("invalid discovery state")
	ErrAssetFrozen      = errors.New("asset is frozen")
	ErrInvalidBlock     = errors.New("invalid block")
	// ErrAssetIsDraft is an operation only a published asset has. Discovery
	// is the one a creator meets.
	ErrAssetIsDraft = errors.New("the asset is still a draft")
	// ErrKindNotBuildable is a kind with no block catalog, which is refused
	// rather than answered with a page that has nothing on it.
	ErrKindNotBuildable = errors.New("that kind cannot be built yet")
	// ErrAppNotAnswered is a kind that is asked which app it is for and was
	// not told, or was told an app Illarin has no slot names for.
	ErrAppNotAnswered = errors.New("that kind needs to know which app it is for")
)

// Service runs the catalog. It knows the module interfaces, never a concrete
// format.
type Service struct {
	pool        *pgxpool.Pool
	reg         *format.Registry
	store       storage.Store
	media       MediaProcessor
	mediaSlots  chan struct{}
	mediaFlight singleflight.Group
	ingest      IngestSettings
	signer      signing.Key
	now         func() time.Time
	siteURL     string
}

type IngestSettings struct {
	ProbeLimits   probe.Limits
	LeaseDuration time.Duration
	RetryBase     time.Duration
	MaxAttempts   int
	MediaWorkers  int
}

type MediaProcessor interface {
	Prepare(context.Context, io.Reader) (mediaproc.Prepared, error)
	Render(context.Context, io.Reader, string) (mediaproc.Derivative, error)
	ComposeSocialPreview(context.Context, io.Reader, string) (mediaproc.Derivative, error)
	DerivativeType() string
}

func DefaultIngestSettings() IngestSettings {
	return IngestSettings{
		ProbeLimits:   probe.DefaultLimits(),
		LeaseDuration: 30 * time.Second,
		RetryBase:     time.Second,
		MaxAttempts:   3,
		MediaWorkers:  2,
	}
}

func NewService(pool *pgxpool.Pool, reg *format.Registry, store storage.Store) *Service {
	return NewServiceWithIngestSettings(pool, reg, store, DefaultIngestSettings())
}

func NewServiceWithProbeLimits(
	pool *pgxpool.Pool,
	reg *format.Registry,
	store storage.Store,
	limits probe.Limits,
) *Service {
	settings := DefaultIngestSettings()
	settings.ProbeLimits = limits
	return NewServiceWithIngestSettings(pool, reg, store, settings)
}

func NewServiceForSite(
	pool *pgxpool.Pool,
	reg *format.Registry,
	store storage.Store,
	limits probe.Limits,
	siteURL string,
) *Service {
	service := NewServiceWithProbeLimits(pool, reg, store, limits)
	service.siteURL = strings.TrimRight(siteURL, "/")
	return service
}

func NewServiceWithIngestSettings(
	pool *pgxpool.Pool,
	reg *format.Registry,
	store storage.Store,
	settings IngestSettings,
) *Service {
	return NewServiceWithMediaProcessor(
		pool, reg, store, settings,
		mediaproc.NewProcessor(mediaproc.DefaultLimits()),
	)
}

func NewServiceWithMediaProcessor(
	pool *pgxpool.Pool,
	reg *format.Registry,
	store storage.Store,
	settings IngestSettings,
	processor MediaProcessor,
) *Service {
	workers := settings.MediaWorkers
	if workers < 1 {
		workers = 1
	}
	return &Service{
		pool: pool, reg: reg, store: store, media: processor,
		mediaSlots: make(chan struct{}, workers),
		ingest:     settings, signer: signing.NewKey(), now: time.Now,
	}
}

// AcceptIngest durably stores an upload and records the work that remains.
func (s *Service) AcceptIngest(ctx context.Context, in IngestInput) (IngestOperation, error) {
	stored, err := s.store.Put(ctx, in.File)
	if err != nil {
		return IngestOperation{}, fmt.Errorf("store upload: %w", err)
	}

	id := uuid.New()
	var tags []string
	if in.Tags != nil {
		tags = *in.Tags
	}
	discovery := in.Discovery
	if discovery == "" {
		discovery = DiscoveryListed
	}
	_, err = s.pool.Exec(ctx, `
		insert into ingest_operations
			(id, owner_id, blob_id, filename, status, name, blurb, tags, is_nsfw, discovery)
		values ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9)
	`, id, in.OwnerID, stored.ID, in.Filename, in.Name, in.Blurb, tags, in.IsNSFW, discovery)
	if err != nil {
		return IngestOperation{}, fmt.Errorf("record ingest: %w", err)
	}
	return IngestOperation{ID: id, Status: IngestPending}, nil
}

// GetIngest returns one operation only to the creator who started it.
func (s *Service) GetIngest(ctx context.Context, ownerID, id uuid.UUID) (IngestOperation, error) {
	var status IngestStatus
	var assetID pgtype.UUID
	var failureReason pgtype.Text
	var failureMessage pgtype.Text
	err := s.pool.QueryRow(ctx, `
		select status, asset_id, failure_reason, failure_message
		  from ingest_operations where id = $1 and owner_id = $2
	`, id, ownerID).Scan(&status, &assetID, &failureReason, &failureMessage)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestOperation{}, ErrIngestNotFound
	}
	if err != nil {
		return IngestOperation{}, fmt.Errorf("read ingest: %w", err)
	}
	operation := IngestOperation{ID: id, Status: status}
	if status == IngestFailed && failureReason.Valid {
		message := s.ingestFailureMessage(failureReason.String)
		if failureMessage.Valid {
			message = failureMessage.String
		}
		operation.Failure = &IngestFailure{
			Reason:  failureReason.String,
			Message: message,
		}
	}
	if assetID.Valid {
		created, err := assetByID(ctx, s.pool, uuidFromPgtype(assetID))
		if err != nil {
			return IngestOperation{}, err
		}
		operation.Asset = &created
	}
	return operation, nil
}

func (s *Service) ingestFailureMessage(reason string) string {
	switch reason {
	case "malformed_input":
		return "The file is malformed and could not be read."
	case "unsupported_format":
		return "No supported format recognised this file. Illarin can read " +
			joinReadable(s.reg.ReadableLabels()) +
			". If yours is not one of those, start from nothing and build it here."
	case "unsupported_version":
		return "The file uses a version LumiHub cannot read safely."
	case "safety_violation":
		return "The file breaks an archive safety rule."
	case "limit_exceeded":
		return "The file is over a content limit."
	case "wrong_kind":
		return "This file is a different kind of thing than the asset it would update."
	default:
		return "LumiHub could not finish this upload. Please try again."
	}
}

// StartFromNothing creates a draft with the kind's required blocks. Preset app
// choice seeds slot names but is not stored.
func (s *Service) StartFromNothing(
	ctx context.Context,
	ownerID uuid.UUID,
	kind string,
	app string,
) (uuid.UUID, error) {
	if _, ok := block.Catalog(kind); !ok {
		return uuid.Nil, ErrKindNotBuildable
	}
	seeded, err := seedElements(kind, app)
	if err != nil {
		return uuid.Nil, err
	}
	blocks, err := block.Place(kind, seeded)
	if err != nil {
		return uuid.Nil, ErrKindNotBuildable
	}

	a := Asset{
		ID: uuid.New(), Kind: kind, Tags: []string{},
		Discovery: DiscoveryListed, Lifecycle: LifecycleDraft,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := insertAsset(ctx, tx, a, ownerID, nil); err != nil {
		return uuid.Nil, err
	}
	if err := insertBlocks(ctx, tx, a.ID, blocks); err != nil {
		return uuid.Nil, err
	}
	if err := s.writeProjections(ctx, tx, a.ID); err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return a.ID, nil
}

// Create stores the upload, reads what it can from it, and publishes one
// catalog entry. Nothing is committed unless every step succeeds.
func (s *Service) Create(ctx context.Context, in CreateInput) (Asset, error) {
	assetID := uuid.New()
	revisionID := uuid.New()

	// Write first so failures leave sweepable orphans instead of missing files.
	stored, err := s.store.Put(ctx, in.File)
	if err != nil {
		return Asset{}, fmt.Errorf("store upload: %w", err)
	}

	inspected, err := probe.Inspect(ctx, s.store, stored.ID, stored.ByteSize, in.Filename)
	if err != nil {
		return Asset{}, fmt.Errorf("probe upload: %w", err)
	}
	read, err := s.readImport(ctx, inspected, "")
	if err != nil {
		return Asset{}, fmt.Errorf("read upload: %w", err)
	}
	parsed := read.Parsed
	kind := parsed.Kind
	discovery := in.Discovery
	if discovery == "" {
		discovery = DiscoveryListed
	}

	a := Asset{
		ID: assetID, Kind: kind, Format: parsed.Format, OriginFormat: &parsed.Format,
		AssetVersion: parsed.Header.AssetVersion, CreditedAuthor: parsed.Header.CreditedAuthor,
		Nickname:  parsed.Header.Nickname,
		Name:      orElse(in.Name, parsed.Header.Name),
		Blurb:     orElse(in.Blurb, parsed.Header.Blurb),
		Tags:      in.Tags,
		IsNSFW:    &in.IsNSFW,
		Discovery: discovery,
		Lifecycle: LifecyclePublished,
	}
	if len(a.Tags) == 0 {
		a.Tags = parsed.Tags
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}
	extractedMedia := read.Media
	blocks := read.Blocks

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Asset{}, err
	}
	defer tx.Rollback(ctx)

	made, err := insertAsset(ctx, tx, a, in.OwnerID, firstDate(in.CreatedAt, parsed.CreatedAt))
	if err != nil {
		return Asset{}, err
	}
	a.CreatedAt = made
	if err := insertRevision(ctx, tx, revisionID, a.ID, revisionRow{
		Revision: 1, BlobID: stored.ID, MediaType: "application/octet-stream", Format: a.Format,
	}); err != nil {
		return Asset{}, err
	}
	if err := insertAssetMedia(ctx, tx, a.ID, extractedMedia); err != nil {
		return Asset{}, err
	}
	if err := insertBlocks(ctx, tx, a.ID, blocks); err != nil {
		return Asset{}, err
	}
	if err := replacePreservedData(ctx, tx, a.ID, parsed.Remainder); err != nil {
		return Asset{}, err
	}
	if err := setCurrentRevision(ctx, tx, a.ID, revisionID); err != nil {
		return Asset{}, err
	}
	if err := setCoverMedia(ctx, tx, a.ID, avatarMedia(extractedMedia)); err != nil {
		return Asset{}, err
	}
	if err := s.writeProjections(ctx, tx, a.ID); err != nil {
		return Asset{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Asset{}, err
	}

	a.CurrentRevisionID = revisionID
	return a, nil
}

// orElse prefers the uploader's catalog metadata. A module only fills in
// blanks.
func orElse(preferred, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}

// firstDate prefers the caller's made date over the file's. Nil from both
// leaves the date to the database, which writes the time of the row.
func firstDate(preferred, fallback *time.Time) *time.Time {
	if preferred != nil {
		return preferred
	}
	return fallback
}

// List returns the assets a visitor is allowed to see, newest first.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Asset, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 24
	}

	return listAssets(ctx, s.pool, f)
}

// Browse returns card-safe catalog results and the reader's effective count.
func (s *Service) Browse(
	ctx context.Context,
	f ListFilter,
	visibility ContentVisibility,
) (BrowsePage, error) {
	if f.Limit <= 0 || f.Limit > 24 {
		f.Limit = 24
	}
	if visibility != ContentHidden && visibility != ContentShown {
		visibility = ContentBlurred
	}
	return s.browseAssets(ctx, f, visibility)
}

// OpenSource opens the stored upload exactly as it arrived.
func (s *Service) OpenSource(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	location, err := currentRevisionLocation(ctx, s.pool, assetID, nil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find current revision: %w", err)
	}

	rc, err := s.store.Open(ctx, location.BlobID)
	if err != nil {
		return nil, fmt.Errorf("open stored file: %w", err)
	}
	return rc, nil
}

type SourceDownload struct {
	InternalRedirect string
	MediaType        string
	Inline           bool
	Event            DownloadEvent
}

// DownloadSource resolves the exact current source for an nginx handoff.
func (s *Service) DownloadSource(
	ctx context.Context,
	assetID uuid.UUID,
	viewerID *uuid.UUID,
) (SourceDownload, error) {
	location, err := currentRevisionLocation(ctx, s.pool, assetID, viewerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SourceDownload{}, ErrNotFound
		}
		return SourceDownload{}, fmt.Errorf("find current revision: %w", err)
	}
	redirect, err := s.store.InternalRedirect(ctx, location.BlobID)
	if err != nil {
		return SourceDownload{}, fmt.Errorf("resolve stored file: %w", err)
	}
	return SourceDownload{
		InternalRedirect: redirect, MediaType: location.MediaType,
		Inline: probe.IsInlineMediaType(location.MediaType),
		Event: downloadEvent(
			location.AssetID, location.RevisionID, RawDownloadTarget,
			location.OwnerID, viewerID,
		),
	}, nil
}

// DownloadExport writes one generated artifact. It is produced on request and
// never cached, because an export is a response rather than stored content.
func (s *Service) DownloadExport(
	ctx context.Context,
	assetID uuid.UUID,
	viewerID *uuid.UUID,
	target string,
) (Export, error) {
	return s.OpenExport(ctx, assetID, viewerID, target)
}

// DownloadExportForLinkedInstance prepares an export after instance authentication.
func (s *Service) DownloadExportForLinkedInstance(
	ctx context.Context,
	assetID uuid.UUID,
	target string,
) (Export, error) {
	download, err := s.OpenExport(ctx, assetID, nil, target)
	if err != nil {
		return Export{}, err
	}
	if download.Event != nil {
		download.Event.AuthorizationClass = AuthorizationLinkedInstance
	}
	return download, nil
}

// joinReadable writes the formats a person can upload the way a person reads
// a list.
func joinReadable(labels []string) string {
	switch len(labels) {
	case 0:
		return "nothing yet"
	case 1:
		return labels[0]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}
