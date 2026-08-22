package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
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
	MediaPackItem         = mediaproc.PackItem
)

type Media struct {
	ID                uuid.UUID
	AssetID           uuid.UUID
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
	// Private is a draft's image, which is short-lived and must not be cached
	// the way a public one is.
	Private bool
}

// MediaRequest is one image request. It names the image and the size wanted,
// and carries whatever the caller presented for it.
type MediaRequest struct {
	MediaID  uuid.UUID
	Variant  string
	Version  uint32
	ViewerID *uuid.UUID
	// Expires and Signature carry the signature a draft's image is served
	// against. A published image needs neither.
	Expires   string
	Signature string
}

type preparedMedia struct {
	ID          uuid.UUID
	BlobID      uuid.UUID
	Role        MediaRole
	ElementRole block.Role
	Name        string
	Width       int
	Height      int
}

type sourceErrorReader struct {
	reader io.Reader
	err    error
}

func (r *sourceErrorReader) Read(payload []byte) (int, error) {
	count, err := r.reader.Read(payload)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = err
	}
	return count, err
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
	fingerprint, err := s.contentFingerprint(ctx, tx, in.AssetID)
	if err != nil {
		return Media{}, err
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
	switch in.Role {
	case MediaAvatar:
		if err := setCoverMedia(ctx, tx, in.AssetID, &id); err != nil {
			return Media{}, err
		}
	case MediaAvatarAlt:
		if err := setAlternateCoverMedia(ctx, tx, in.AssetID, id); err != nil {
			return Media{}, err
		}
	}
	// A picture nothing points at yet reaches no file. This moves the counter
	// where the new picture became the cover, and leaves it where a gallery
	// image waits for the block save that will use it.
	if err := s.moveContentGeneration(ctx, tx, in.AssetID, fingerprint); err != nil {
		return Media{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Media{}, fmt.Errorf("commit media addition: %w", err)
	}
	return Media{
		ID: id, AssetID: in.AssetID, Role: in.Role,
		Width: prepared.Width, Height: prepared.Height,
		DerivativeVersion: mediaproc.DerivativeVersion,
	}, nil
}

// ListMedia returns an asset's media.
func (s *Service) ListMedia(ctx context.Context, assetID uuid.UUID, viewerID *uuid.UUID) ([]Media, error) {
	var foundAssetID uuid.UUID
	err := s.pool.QueryRow(ctx,
		`select id
		   from assets
		  where id = $1 and deleted_at is null
		    and (lifecycle = 'published' or owner_id = $2)
		    and (withheld_at is null or owner_id = $2)`, assetID, viewerID,
	).Scan(&foundAssetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find asset media: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		select id, asset_id, role, width, height
		  from asset_media
		 where asset_id = $1 and is_current
		 order by created_at, id
	`, foundAssetID)
	if err != nil {
		return nil, fmt.Errorf("list asset media: %w", err)
	}
	defer rows.Close()
	media := make([]Media, 0)
	for rows.Next() {
		var found Media
		var width, height pgtype.Int4
		if err := rows.Scan(
			&found.ID, &found.AssetID, &found.Role, &width, &height,
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
			if errors.Is(err, probe.ErrImageUnavailable) {
				continue
			}
			return nil, fmt.Errorf("open extracted media: %w", err)
		}
		tracked := &sourceErrorReader{reader: source}
		stored, err := s.store.Put(ctx, tracked)
		closeErr := source.Close()
		if localImageReadFailure(tracked.err) || localImageReadFailure(closeErr) {
			continue
		}
		if tracked.err != nil {
			return nil, fmt.Errorf("read extracted media: %w", tracked.err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close extracted media: %w", closeErr)
		}
		if err != nil {
			return nil, fmt.Errorf("store extracted media: %w", err)
		}
		image, err := s.prepareMedia(ctx, stored)
		if err != nil {
			if errors.Is(err, mediaproc.ErrUnsupportedImage) {
				continue
			}
			return nil, err
		}
		prepared = append(prepared, preparedMedia{
			ID: uuid.New(), BlobID: stored.ID, Role: role,
			ElementRole: item.ElementRole, Name: item.Name,
			Width: image.Width, Height: image.Height,
		})
	}
	return prepared, nil
}

func localImageReadFailure(err error) bool {
	return err != nil && !errors.Is(err, probe.ErrRangeRead) && !errors.Is(err, context.Canceled)
}

func elementsForExtractedMedia(media []preparedMedia) []block.Element {
	grouped := make(map[block.Role][]block.ImageItem)
	order := make([]block.Role, 0, 2)
	for _, item := range media {
		if item.ElementRole == "" {
			continue
		}
		if _, seen := grouped[item.ElementRole]; !seen {
			order = append(order, item.ElementRole)
		}
		grouped[item.ElementRole] = append(grouped[item.ElementRole], block.ImageItem{
			ID: block.NewItemID(), MediaID: item.ID, Name: item.Name,
		})
	}
	elements := make([]block.Element, 0, len(order))
	for _, role := range order {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeImageSet, Role: role,
			Content: block.ImageSet{Images: grouped[role]},
		})
	}
	return elements
}

func insertAssetMedia(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	media []preparedMedia,
) error {
	for _, item := range media {
		_, err := tx.Exec(ctx, `
			insert into asset_media
			  (id, asset_id, role, width, height, blob_id, is_extracted)
			values ($1, $2, $3, $4, $5, $6, true)
		`, item.ID, assetID, item.Role, item.Width, item.Height, item.BlobID)
		if err != nil {
			return fmt.Errorf("record extracted media: %w", err)
		}
	}
	return nil
}

func supersedeExtractedMedia(ctx context.Context, tx pgx.Tx, assetID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		update asset_media
		   set is_current = false
		 where asset_id = $1 and is_extracted and is_current
	`, assetID); err != nil {
		return fmt.Errorf("supersede extracted media: %w", err)
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

// MediaVariant returns a cached image and safely regenerates a missing one. A
// draft's image is at the same address as a published one and is served only
// against a signature Go wrote.
func (s *Service) MediaVariant(ctx context.Context, in MediaRequest) (MediaDownload, error) {
	_, ordinary := mediaproc.VariantByName(in.Variant)
	_, composed := mediaproc.SocialPreviewByName(in.Variant)
	if (!ordinary && !composed) || in.Version != mediaproc.DerivativeVersion {
		return MediaDownload{}, ErrMediaNotFound
	}
	variant, version := in.Variant, in.Version
	var blobID uuid.UUID
	var digestBytes []byte
	var lifecycle string
	err := s.pool.QueryRow(ctx, `
		select media.blob_id, blob.sha256, asset.lifecycle
		  from asset_media media
		  join assets asset on asset.id = media.asset_id
		  join blobs blob on blob.id = media.blob_id
		 where media.id = $1
		   and asset.deleted_at is null
		   and (asset.withheld_at is null or asset.owner_id = $2)
	`, in.MediaID, in.ViewerID).Scan(&blobID, &digestBytes, &lifecycle)
	if errors.Is(err, pgx.ErrNoRows) {
		return MediaDownload{}, ErrMediaNotFound
	}
	if err != nil {
		return MediaDownload{}, fmt.Errorf("find media: %w", err)
	}
	private := Lifecycle(lifecycle) == LifecycleDraft
	if private {
		path := fmt.Sprintf("/media/%s/%s/%d", in.MediaID, variant, version)
		if !s.signer.Valid(path, in.Expires, in.Signature, s.now()) {
			return MediaDownload{}, ErrMediaNotFound
		}
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
	return MediaDownload{
		InternalRedirect: redirect,
		MediaType:        s.media.DerivativeType(),
		Private:          private,
	}, nil
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
