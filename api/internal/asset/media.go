package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	mediaproc "github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrMediaNotFound    = errors.New("media not found")
	ErrInvalidMediaRole = errors.New("invalid media role")
)

type MediaRole = mediaproc.Role

const (
	MediaAvatar           = mediaproc.Avatar
	MediaExpression       = mediaproc.Expression
	MediaGallery          = mediaproc.Gallery
	MediaAvatarAlt        = mediaproc.AvatarAlt
	MediaPerspectiveLayer = mediaproc.PerspectiveLayer
)

type Media struct {
	ID                uuid.UUID
	AssetID           *uuid.UUID
	RevisionID        *uuid.UUID
	Role              MediaRole
	Width             int
	Height            int
	DerivativeVersion uint32
}

type AddMediaInput struct {
	OwnerID uuid.UUID
	AssetID uuid.UUID
	Role    MediaRole
	File    io.Reader
}

type MediaDownload struct {
	InternalRedirect string
	MediaType        string
}

type preparedMedia struct {
	ID     uuid.UUID
	BlobID uuid.UUID
	Role   MediaRole
	Width  int
	Height int
}

// AddMedia stores one creator-managed image under a new media ID.
func (s *Service) AddMedia(ctx context.Context, in AddMediaInput) (Media, error) {
	if !in.Role.Valid() {
		return Media{}, ErrInvalidMediaRole
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Media{}, fmt.Errorf("begin media addition: %w", err)
	}
	defer tx.Rollback(ctx)

	var withheldAt pgtype.Timestamptz
	err = tx.QueryRow(ctx, `
		select withheld_at
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
		 for update
	`, in.AssetID, in.OwnerID).Scan(&withheldAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, ErrMediaNotFound
	}
	if err != nil {
		return Media{}, fmt.Errorf("check media owner: %w", err)
	}
	if withheldAt.Valid {
		return Media{}, ErrAssetFrozen
	}

	stored, err := s.store.Put(ctx, in.File)
	if err != nil {
		return Media{}, fmt.Errorf("store media: %w", err)
	}
	prepared, err := s.prepareMedia(ctx, stored)
	if err != nil {
		return Media{}, err
	}
	id := uuid.New()
	_, err = tx.Exec(ctx, `
		insert into asset_media (id, asset_id, role, width, height, blob_id)
		values ($1, $2, $3, $4, $5, $6)
	`, id, in.AssetID, in.Role, prepared.Width, prepared.Height, stored.ID)
	if err != nil {
		return Media{}, fmt.Errorf("record media: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Media{}, fmt.Errorf("commit media addition: %w", err)
	}
	assetID := in.AssetID
	return Media{
		ID: id, AssetID: &assetID, Role: in.Role,
		Width: prepared.Width, Height: prepared.Height,
		DerivativeVersion: mediaproc.DerivativeVersion,
	}, nil
}

// ListMedia returns creator-managed media and media from the current revision.
func (s *Service) ListMedia(ctx context.Context, assetID uuid.UUID, viewerID *uuid.UUID) ([]Media, error) {
	var revisionID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`select current_revision_id
		   from assets
		  where id = $1 and deleted_at is null
		    and (withheld_at is null or owner_id = $2)`, assetID, viewerID,
	).Scan(&revisionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find asset media: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		select id, asset_id, revision_id, role, width, height
		  from asset_media
		 where asset_id = $1 or revision_id = $2
		 order by created_at, id
	`, assetID, revisionID)
	if err != nil {
		return nil, fmt.Errorf("list asset media: %w", err)
	}
	defer rows.Close()
	media := make([]Media, 0)
	for rows.Next() {
		var found Media
		var width, height pgtype.Int4
		if err := rows.Scan(
			&found.ID, &found.AssetID, &found.RevisionID, &found.Role, &width, &height,
		); err != nil {
			return nil, fmt.Errorf("read asset media: %w", err)
		}
		if !width.Valid || !height.Valid {
			return nil, fmt.Errorf("media %s has no native dimensions", found.ID)
		}
		found.Width = int(width.Int32)
		found.Height = int(height.Int32)
		found.DerivativeVersion = mediaproc.DerivativeVersion
		media = append(media, found)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list asset media: %w", err)
	}
	return media, nil
}

func (s *Service) prepareMedia(ctx context.Context, stored storage.StoredBlob) (mediaproc.Prepared, error) {
	source, err := s.store.Open(ctx, stored.ID)
	if err != nil {
		return mediaproc.Prepared{}, fmt.Errorf("open stored media: %w", err)
	}
	release, err := s.acquireMediaSlot(ctx)
	if err != nil {
		source.Close()
		return mediaproc.Prepared{}, err
	}
	prepared, prepareErr := s.media.Prepare(ctx, source)
	release()
	closeErr := source.Close()
	if prepareErr != nil {
		return mediaproc.Prepared{}, prepareErr
	}
	if closeErr != nil {
		return mediaproc.Prepared{}, fmt.Errorf("close stored media: %w", closeErr)
	}
	for _, derivative := range prepared.Derivatives {
		id := storage.DerivativeID{
			SourceDigest: stored.Digest,
			Variant:      derivative.Variant,
			Version:      mediaproc.DerivativeVersion,
		}
		if err := s.store.PutDerivative(ctx, id, bytes.NewReader(derivative.Bytes)); err != nil {
			return mediaproc.Prepared{}, fmt.Errorf("store %s media variant: %w", derivative.Variant, err)
		}
	}
	return prepared, nil
}

func (s *Service) prepareExtractedMedia(
	ctx context.Context,
	file probe.Inspection,
	extracted []format.Media,
) ([]preparedMedia, error) {
	prepared := make([]preparedMedia, 0, len(extracted))
	for _, item := range extracted {
		role := item.Role
		if !role.Valid() {
			return nil, fmt.Errorf("format returned media role %q: %w", item.Role, ErrInvalidMediaRole)
		}
		source, err := file.OpenImage(ctx, item.ImageID)
		if err != nil {
			return nil, fmt.Errorf("open extracted media: %w", err)
		}
		stored, err := s.store.Put(ctx, source)
		source.Close()
		if err != nil {
			return nil, fmt.Errorf("store extracted media: %w", err)
		}
		image, err := s.prepareMedia(ctx, stored)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedMedia{
			ID: uuid.New(), BlobID: stored.ID, Role: role,
			Width: image.Width, Height: image.Height,
		})
	}
	return prepared, nil
}

func insertRevisionMedia(
	ctx context.Context,
	tx pgx.Tx,
	revisionID uuid.UUID,
	media []preparedMedia,
) error {
	for _, item := range media {
		_, err := tx.Exec(ctx, `
			insert into asset_media (id, revision_id, role, width, height, blob_id)
			values ($1, $2, $3, $4, $5, $6)
		`, item.ID, revisionID, item.Role, item.Width, item.Height, item.BlobID)
		if err != nil {
			return fmt.Errorf("record extracted media: %w", err)
		}
	}
	return nil
}

func mediaIngestFailure(err error) format.FailureReason {
	switch {
	case errors.Is(err, mediaproc.ErrImageTooLarge):
		return format.FailureSafetyViolation
	case errors.Is(err, mediaproc.ErrUnsupportedImage):
		return format.FailureMalformedInput
	default:
		return format.FailureInternal
	}
}

// MediaVariant returns a cached image and safely regenerates a missing one.
func (s *Service) MediaVariant(
	ctx context.Context,
	mediaID uuid.UUID,
	variant string,
	version uint32,
	viewerID *uuid.UUID,
) (MediaDownload, error) {
	_, ordinary := mediaproc.VariantByName(variant)
	_, composed := mediaproc.SocialPreviewByName(variant)
	if (!ordinary && !composed) || version != mediaproc.DerivativeVersion {
		return MediaDownload{}, ErrMediaNotFound
	}
	var blobID uuid.UUID
	var digestBytes []byte
	err := s.pool.QueryRow(ctx, `
		select media.blob_id, blob.sha256
		  from asset_media media
		  left join asset_revisions revision on revision.id = media.revision_id
		  join assets asset on asset.id = coalesce(media.asset_id, revision.asset_id)
		  join blobs blob on blob.id = media.blob_id
		 where media.id = $1
		   and asset.deleted_at is null
		   and (asset.withheld_at is null or asset.owner_id = $2)
	`, mediaID, viewerID).Scan(&blobID, &digestBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaDownload{}, ErrMediaNotFound
	}
	if err != nil {
		return MediaDownload{}, fmt.Errorf("find media: %w", err)
	}
	if len(digestBytes) != sha256.Size {
		return MediaDownload{}, fmt.Errorf("media blob has a %d-byte digest", len(digestBytes))
	}
	var digest [sha256.Size]byte
	copy(digest[:], digestBytes)
	derivativeID := storage.DerivativeID{
		SourceDigest: digest,
		Variant:      variant,
		Version:      version,
	}
	redirect, err := s.store.InternalDerivativeRedirect(ctx, derivativeID)
	if errors.Is(err, storage.ErrDerivativeNotFound) {
		job := s.mediaFlight.DoChan(fmt.Sprintf("%x/%s/%d", digest, variant, version), func() (any, error) {
			return nil, s.regenerateMediaVariant(ctx, blobID, derivativeID)
		})
		select {
		case <-ctx.Done():
			return MediaDownload{}, ctx.Err()
		case result := <-job:
			if result.Err != nil {
				return MediaDownload{}, result.Err
			}
		}
		redirect, err = s.store.InternalDerivativeRedirect(ctx, derivativeID)
	}
	if err != nil {
		return MediaDownload{}, fmt.Errorf("resolve media variant: %w", err)
	}
	return MediaDownload{InternalRedirect: redirect, MediaType: s.media.DerivativeType()}, nil
}

func (s *Service) regenerateMediaVariant(
	ctx context.Context,
	blobID uuid.UUID,
	id storage.DerivativeID,
) error {
	release, err := s.acquireMediaSlot(ctx)
	if err != nil {
		return err
	}
	defer release()
	source, err := s.store.Open(ctx, blobID)
	if err != nil {
		return fmt.Errorf("open media for regeneration: %w", err)
	}
	var derivative mediaproc.Derivative
	var renderErr error
	if _, composed := mediaproc.SocialPreviewByName(id.Variant); composed {
		derivative, renderErr = s.media.ComposeSocialPreview(ctx, source, id.Variant)
	} else {
		derivative, renderErr = s.media.Render(ctx, source, id.Variant)
	}
	closeErr := source.Close()
	if renderErr != nil {
		return fmt.Errorf("regenerate media variant: %w", renderErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close media after regeneration: %w", closeErr)
	}
	if err := s.store.PutDerivative(ctx, id, bytes.NewReader(derivative.Bytes)); err != nil {
		return fmt.Errorf("store regenerated media variant: %w", err)
	}
	return nil
}

func (s *Service) acquireMediaSlot(ctx context.Context) (func(), error) {
	select {
	case s.mediaSlots <- struct{}{}:
		return func() { <-s.mediaSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
