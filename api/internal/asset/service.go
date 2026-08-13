package asset

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
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

// detectHeadBytes is how much of a file a module gets to recognise it by.
const detectHeadBytes = 512

// Create stores the upload, reads what it can from it, and publishes one
// catalog entry. Nothing is committed unless every step succeeds.
//
// The file is never held whole. It is hashed and counted as it is written, and
// the module reads it back from storage, so the memory this costs does not
// grow with the size of the upload.
func (s *Service) Create(ctx context.Context, in CreateInput) (Asset, error) {
	assetID := uuid.New()
	revisionID := uuid.New()

	file := bufio.NewReaderSize(in.File, detectHeadBytes)
	head, err := file.Peek(detectHeadBytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return Asset{}, fmt.Errorf("read upload: %w", err)
	}
	module := s.reg.Detect(in.Filename, head)

	// The file is written before the transaction on purpose. A failed
	// transaction leaves an unreferenced file, which a sweeper can collect.
	// Writing it after would let a committed row point at a file that is
	// not there yet, which is the worse of the two failures.
	stored, err := s.store.Put(ctx, file)
	if err != nil {
		return Asset{}, fmt.Errorf("store upload: %w", err)
	}

	parsed, err := s.parseStored(ctx, module, stored.ID)
	if err != nil {
		return Asset{}, err
	}

	a := Asset{
		ID:            assetID,
		Kind:          in.Kind,
		Platform:      parsed.Platform,
		Format:        parsed.Format,
		FormatVersion: parsed.FormatVersion,
		Name:          orElse(in.Name, parsed.Name),
		Description:   orElse(in.Description, parsed.Description),
		Tags:          in.Tags,
		IsNSFW:        in.IsNSFW,
		Publication:   in.Publication,
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
		Revision:  1,
		BlobID:    stored.ID,
		MediaType: "application/octet-stream",
	}); err != nil {
		return Asset{}, err
	}
	if err := insertFacets(ctx, tx, a.ID, revisionID, parsed.Facets); err != nil {
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

// parseStored gives the module the stored file rather than a copy held in
// memory.
func (s *Service) parseStored(ctx context.Context, module format.Module, blobID uuid.UUID) (format.Parsed, error) {
	stored, err := s.store.Open(ctx, blobID)
	if err != nil {
		return format.Parsed{}, fmt.Errorf("reopen stored upload: %w", err)
	}
	defer stored.Close()

	parsed, err := module.Parse(ctx, stored)
	if err != nil {
		return format.Parsed{}, fmt.Errorf("parse upload: %w", err)
	}
	return parsed, nil
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
