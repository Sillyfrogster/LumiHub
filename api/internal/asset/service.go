package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	mediaproc "github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
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
	now         func() time.Time
}

type IngestSettings struct {
	ProbeLimits   probe.Limits
	LeaseDuration time.Duration
	NeedsKindTTL  time.Duration
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
		NeedsKindTTL:  7 * 24 * time.Hour,
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
		ingest:     settings, now: time.Now,
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
			(id, owner_id, blob_id, filename, status, kind, name, blurb, tags, is_nsfw, discovery)
		values ($1, $2, $3, $4, 'pending', null, $5, $6, $7, $8, $9)
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
	var name pgtype.Text
	var expiresAt pgtype.Timestamptz
	var failureReason pgtype.Text
	err := s.pool.QueryRow(ctx, `
		select status, asset_id, name, expires_at, failure_reason
		  from ingest_operations where id = $1 and owner_id = $2
	`, id, ownerID).Scan(&status, &assetID, &name, &expiresAt, &failureReason)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestOperation{}, ErrIngestNotFound
	}
	if err != nil {
		return IngestOperation{}, fmt.Errorf("read ingest: %w", err)
	}
	operation := IngestOperation{ID: id, Status: status}
	if status == IngestNeedsKind {
		if expiresAt.Valid && !expiresAt.Time.After(s.now()) {
			_, _ = s.pool.Exec(ctx,
				`delete from ingest_operations where id = $1 and status = 'needs_kind'`, id)
			return IngestOperation{}, ErrIngestNotFound
		}
		operation.NeedsKind = &NeedsKind{Name: name.String}
	}
	if status == IngestFailed && failureReason.Valid {
		operation.Failure = &IngestFailure{
			Reason:  failureReason.String,
			Message: ingestFailureMessage(failureReason.String),
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

func ingestFailureMessage(reason string) string {
	switch reason {
	case "malformed_input":
		return "The file is malformed and could not be read."
	case "unsupported_format":
		return "The file names a format LumiHub does not support."
	case "unsupported_version":
		return "The file uses a version LumiHub cannot read safely."
	case "safety_violation":
		return "The file breaks an archive safety rule."
	default:
		return "LumiHub could not finish this upload. Please try again."
	}
}

// Create stores the upload, reads what it can from it, and publishes one
// catalog entry. Nothing is committed unless every step succeeds.
func (s *Service) Create(ctx context.Context, in CreateInput) (Asset, error) {
	assetID := uuid.New()
	revisionID := uuid.New()

	// The file is written before the transaction on purpose. A failed
	// transaction leaves an unreferenced file, which a sweeper can collect.
	// Writing it after would let a committed row point at a file that is
	// not there yet, which is the worse of the two failures.
	stored, err := s.store.Put(ctx, in.File)
	if err != nil {
		return Asset{}, fmt.Errorf("store upload: %w", err)
	}

	inspected, err := probe.Inspect(ctx, s.store, stored.ID, stored.ByteSize, in.Filename)
	if err != nil {
		return Asset{}, fmt.Errorf("probe upload: %w", err)
	}
	resolution, claimed, err := s.reg.Resolve(inspected)
	if err != nil {
		return Asset{}, fmt.Errorf("resolve upload format: %w", err)
	}
	parsed := format.Parsed{Format: "unknown"}
	if claimed {
		parsed, err = resolution.Module.Parse(ctx, inspected, resolution.Claim)
		if err != nil {
			return Asset{}, fmt.Errorf("parse upload: %w", err)
		}
	}

	kind := parsed.Kind
	passthroughPlatform := (*string)(nil)
	if !claimed {
		kind = in.Kind
		if kind == "" {
			return Asset{}, errors.New("a passthrough upload needs a kind")
		}
		passthroughPlatform = parsed.PassthroughPlatform
	} else {
		if kind == "" {
			return Asset{}, fmt.Errorf("format module %q did not declare a kind", resolution.Module.ID())
		}
		if parsed.PassthroughPlatform != nil {
			return Asset{}, fmt.Errorf("format module %q returned a passthrough platform", resolution.Module.ID())
		}
	}
	discovery := in.Discovery
	if discovery == "" {
		discovery = DiscoveryListed
	}

	a := Asset{
		ID:                  assetID,
		Kind:                kind,
		Format:              parsed.Format,
		PassthroughPlatform: passthroughPlatform,
		Name:                orElse(in.Name, parsed.Name),
		Blurb:               orElse(in.Blurb, parsed.Blurb),
		Tags:                in.Tags,
		IsNSFW:              in.IsNSFW,
		Discovery:           discovery,
	}
	if len(a.Tags) == 0 {
		a.Tags = parsed.Tags
	}
	if a.Tags == nil {
		a.Tags = []string{}
	}
	extractedMedia, err := s.prepareExtractedMedia(ctx, parsed.Media)
	if err != nil {
		return Asset{}, fmt.Errorf("prepare extracted media: %w", err)
	}

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
		Revision:            1,
		BlobID:              stored.ID,
		MediaType:           "application/octet-stream",
		Format:              a.Format,
		PassthroughPlatform: a.PassthroughPlatform,
	}); err != nil {
		return Asset{}, err
	}
	if err := insertFacets(ctx, tx, revisionID, parsed.Facets); err != nil {
		return Asset{}, err
	}
	if err := insertRevisionMedia(ctx, tx, revisionID, extractedMedia); err != nil {
		return Asset{}, err
	}
	if err := setCurrentRevision(ctx, tx, a.ID, revisionID); err != nil {
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
	return browseAssets(ctx, s.pool, s.reg, f, visibility)
}

// OpenSource opens the stored upload exactly as it arrived.
func (s *Service) OpenSource(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	blobID, _, err := currentRevisionLocation(ctx, s.pool, assetID, nil)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find current revision: %w", err)
	}

	rc, err := s.store.Open(ctx, blobID)
	if err != nil {
		return nil, fmt.Errorf("open stored file: %w", err)
	}
	return rc, nil
}

type SourceDownload struct {
	InternalRedirect string
	MediaType        string
	Inline           bool
}

// DownloadSource resolves the current source file for an nginx handoff.
func (s *Service) DownloadSource(ctx context.Context, assetID uuid.UUID, viewerID *uuid.UUID) (SourceDownload, error) {
	blobID, mediaType, err := currentRevisionLocation(ctx, s.pool, assetID, viewerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SourceDownload{}, ErrNotFound
		}
		return SourceDownload{}, fmt.Errorf("find current revision: %w", err)
	}
	redirect, err := s.store.InternalRedirect(ctx, blobID)
	if err != nil {
		return SourceDownload{}, fmt.Errorf("resolve stored file: %w", err)
	}
	return SourceDownload{
		InternalRedirect: redirect,
		MediaType:        mediaType,
		Inline:           probe.IsInlineMediaType(mediaType),
	}, nil
}
