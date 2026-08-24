package asset

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// StagedImage is one picture already on disk and measured, waiting for the transaction that gives it an asset.
type StagedImage struct {
	BlobID uuid.UUID
	Width  int
	Height int
}

// MigratedImage is one staged picture placed on a migrated asset.
type MigratedImage struct {
	ID     uuid.UUID
	BlobID uuid.UUID
	Role   MediaRole
	Width  int
	Height int
}

// MigratedAsset is one v1 row ready to become asset rows, carrying no revision because the row is the source.
type MigratedAsset struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Kind      string
	Origin    string
	Header    format.Header
	Tags      []string
	IsNSFW    bool
	CreatedAt time.Time
	Blocks    []block.Block
	Remainder []format.Remainder
	Protected format.ProtectedImport
	Images    []MigratedImage
	CoverID   *uuid.UUID
}

// StageImage stores one image and renders its variants, writing nothing to the asset tables.
func (s *Service) StageImage(ctx context.Context, body io.Reader) (StagedImage, error) {
	stored, err := s.store.Put(ctx, body)
	if err != nil {
		return StagedImage{}, fmt.Errorf("store the image: %w", err)
	}
	prepared, err := s.prepareMedia(ctx, stored)
	if err != nil {
		return StagedImage{}, err
	}
	return StagedImage{BlobID: stored.ID, Width: prepared.Width, Height: prepared.Height}, nil
}

// WriteMigratedAsset turns one read v1 row into the rows an upload writes, published whatever today's floor would have said.
func (s *Service) WriteMigratedAsset(ctx context.Context, tx pgx.Tx, one MigratedAsset) error {
	isNSFW := one.IsNSFW
	origin := one.Origin
	createdAt := one.CreatedAt
	record := Asset{
		ID: one.ID, Kind: one.Kind, Format: one.Origin, OriginFormat: &origin,
		AssetVersion: one.Header.AssetVersion, CreditedAuthor: one.Header.CreditedAuthor,
		Nickname: one.Header.Nickname, Name: one.Header.Name, Blurb: one.Header.Blurb,
		Tags: one.Tags, IsNSFW: &isNSFW, Discovery: DiscoveryListed,
		Lifecycle: LifecyclePublished,
	}
	if _, err := insertAsset(ctx, tx, record, one.OwnerID, &createdAt); err != nil {
		return err
	}
	if err := insertBlocks(ctx, tx, one.ID, one.Blocks); err != nil {
		return err
	}
	if err := replacePreservedData(ctx, tx, one.ID, one.Remainder); err != nil {
		return err
	}
	if err := importProtectedPrompts(ctx, tx, one.ID, one.Blocks, one.Protected); err != nil {
		return err
	}
	for _, image := range one.Images {
		if _, err := tx.Exec(ctx, `
			insert into asset_media (id, asset_id, role, width, height, blob_id)
			values ($1, $2, $3, $4, $5, $6)
		`, image.ID, one.ID, image.Role, image.Width, image.Height, image.BlobID); err != nil {
			return fmt.Errorf("record migrated media: %w", err)
		}
	}
	if one.CoverID != nil {
		if err := setCoverMedia(ctx, tx, one.ID, one.CoverID); err != nil {
			return err
		}
	}
	return s.writeProjections(ctx, tx, one.ID)
}

// MigratedShortfall reports what an asset still needs to clear today's publish floor, and nothing where it clears it.
func MigratedShortfall(kind, name string, isNSFW *bool, blocks []block.Block) []ReadinessItem {
	items := readiness(kind, name, isNSFW, blocks)
	if Ready(items) {
		return nil
	}
	return items
}

// LegacyAsset is what a v1 public address resolves to.
type LegacyAsset struct {
	ID   uuid.UUID
	Name string
}

// ResolveLegacyAddress runs the real lookup rather than rewriting the path, so a withheld, deleted or never-existed address is a plain miss.
func (s *Service) ResolveLegacyAddress(ctx context.Context, address string) (LegacyAsset, error) {
	row, err := db.New(s.pool).LegacyPathTarget(ctx, address)
	if errors.Is(err, pgx.ErrNoRows) {
		return LegacyAsset{}, ErrNotFound
	}
	if err != nil {
		return LegacyAsset{}, fmt.Errorf("resolve the v1 address: %w", err)
	}
	return LegacyAsset{ID: uuid.UUID(row.ID.Bytes), Name: row.Name}, nil
}
