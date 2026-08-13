package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("asset not found")

// Service runs the catalog. It knows the module interfaces, never a concrete
// format.
type Service struct {
	pool  *pgxpool.Pool
	reg   *format.Registry
	store storage.Store
}

func NewService(pool *pgxpool.Pool, reg *format.Registry, store storage.Store) *Service {
	return &Service{pool: pool, reg: reg, store: store}
}

// Create stores the upload, reads what it can from it, and publishes one
// catalog entry. Nothing is committed unless every step succeeds.
//
// The file is never held whole. Storage hashes and counts it while writing,
// then the shared probe reads bounded ranges from the stored blob.
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
		Description:         orElse(in.Description, parsed.Description),
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

// OpenOriginal opens the stored upload exactly as it arrived.
func (s *Service) OpenOriginal(ctx context.Context, assetID uuid.UUID) (io.ReadCloser, error) {
	blobID, _, err := currentRevisionLocation(ctx, s.pool, assetID)
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
