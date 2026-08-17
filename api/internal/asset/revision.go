package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// revisionRow is one preserved copy of an asset's bytes.
type revisionRow struct {
	Revision            int
	BlobID              uuid.UUID
	MediaType           string
	Format              string
	PassthroughPlatform *string
}

func insertRevision(ctx context.Context, tx pgx.Tx, id, assetID uuid.UUID, row revisionRow) error {
	queries := db.New(tx)
	params := db.InsertRevisionParams{
		ID:                  uuidToPgtype(id),
		AssetID:             uuidToPgtype(assetID),
		Revision:            int32(row.Revision),
		BlobID:              uuidToPgtype(row.BlobID),
		MediaType:           row.MediaType,
		Format:              row.Format,
		PassthroughPlatform: textToNullable(row.PassthroughPlatform),
	}
	if err := queries.InsertRevision(ctx, params); err != nil {
		return fmt.Errorf("insert revision: %w", err)
	}
	return nil
}

// AcceptRevision stores new bytes for an asset that already exists and records
// the work that remains. Kind is settled before the file is read, because the
// asset already has one and a revision never changes it.
func (s *Service) AcceptRevision(ctx context.Context, in RevisionInput) (IngestOperation, error) {
	var withheldAt pgtype.Timestamptz
	err := s.pool.QueryRow(ctx, `
		select withheld_at
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
	`, in.AssetID, in.OwnerID).Scan(&withheldAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return IngestOperation{}, ErrNotFound
	}
	if err != nil {
		return IngestOperation{}, fmt.Errorf("check revision owner: %w", err)
	}
	if withheldAt.Valid {
		return IngestOperation{}, ErrAssetFrozen
	}

	stored, err := s.store.Put(ctx, in.File)
	if err != nil {
		return IngestOperation{}, fmt.Errorf("store revision: %w", err)
	}
	id := uuid.New()
	_, err = s.pool.Exec(ctx, `
		insert into ingest_operations
			(id, owner_id, blob_id, filename, status, target_asset_id)
		values ($1, $2, $3, $4, 'pending', $5)
	`, id, in.OwnerID, stored.ID, in.Filename, in.AssetID)
	if err != nil {
		return IngestOperation{}, fmt.Errorf("record revision ingest: %w", err)
	}
	return IngestOperation{ID: id, Status: IngestPending}, nil
}

// setCurrentRevision points the asset at its current revision. Callers never
// derive this.
func setCurrentRevision(ctx context.Context, tx pgx.Tx, assetID, revisionID uuid.UUID) error {
	queries := db.New(tx)
	params := db.SetCurrentRevisionParams{
		ID:                uuidToPgtype(assetID),
		CurrentRevisionID: uuidToPgtype(revisionID),
	}
	if err := queries.SetCurrentRevision(ctx, params); err != nil {
		return fmt.Errorf("set current revision: %w", err)
	}
	return nil
}

type revisionLocation struct {
	AssetID    uuid.UUID
	RevisionID uuid.UUID
	BlobID     uuid.UUID
	MediaType  string
	OwnerID    *uuid.UUID
}

func currentRevisionLocation(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
	viewerID *uuid.UUID,
) (revisionLocation, error) {
	queries := db.New(q)
	row, err := queries.CurrentRevisionLocation(ctx, db.CurrentRevisionLocationParams{
		ID: uuidToPgtype(assetID), ViewerID: uuidToNullable(viewerID),
	})
	if err != nil {
		return revisionLocation{}, err
	}
	var ownerID *uuid.UUID
	if row.OwnerID.Valid {
		owner := uuidFromPgtype(row.OwnerID)
		ownerID = &owner
	}
	return revisionLocation{
		AssetID: uuidFromPgtype(row.AssetID), RevisionID: uuidFromPgtype(row.RevisionID),
		BlobID: uuidFromPgtype(row.BlobID), MediaType: row.MediaType,
		OwnerID: ownerID,
	}, nil
}

// setCoverMedia points the asset at the picture a reader should see first.
func setCoverMedia(ctx context.Context, tx pgx.Tx, assetID uuid.UUID, mediaID *uuid.UUID) error {
	if _, err := tx.Exec(ctx,
		`update assets set cover_media_id = $2 where id = $1`, assetID, mediaID,
	); err != nil {
		return fmt.Errorf("set cover media: %w", err)
	}
	return nil
}

// setAlternateCoverMedia uses an alternate until a primary cover takes its place.
func setAlternateCoverMedia(ctx context.Context, tx pgx.Tx, assetID, mediaID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		update assets asset
		   set cover_media_id = $2
		 where asset.id = $1
		   and (
		       asset.cover_media_id is null
		       or exists (
		           select 1
		             from asset_media cover
		            where cover.id = asset.cover_media_id
		              and cover.asset_id = asset.id
		              and cover.role = 'avatar_alt'
		       )
		   )
	`, assetID, mediaID); err != nil {
		return fmt.Errorf("set alternate cover media: %w", err)
	}
	return nil
}

// clearSupersededCover removes an imported cover when its replacement has none.
func clearSupersededCover(ctx context.Context, tx pgx.Tx, assetID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		update assets asset
		   set cover_media_id = null
		 where asset.id = $1
		   and exists (
		       select 1
		         from asset_media media
		        where media.id = asset.cover_media_id
		          and media.asset_id = asset.id
		          and not media.is_current
		   )
	`, assetID); err != nil {
		return fmt.Errorf("clear superseded cover: %w", err)
	}
	return nil
}

// avatarMedia finds the picture that stands for the asset.
func avatarMedia(media []preparedMedia) *uuid.UUID {
	var alternate *uuid.UUID
	for _, item := range media {
		if item.Role == MediaAvatar {
			id := item.ID
			return &id
		}
		if item.Role == MediaAvatarAlt && alternate == nil {
			id := item.ID
			alternate = &id
		}
	}
	return alternate
}
