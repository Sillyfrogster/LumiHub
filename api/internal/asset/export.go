package asset

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ExportFile is one resolved primary export artifact.
type ExportFile struct {
	Artifact        io.ReadCloser
	MediaType       string
	Extension       string
	Target          string
	UnembeddedMedia []format.ExportMedia
	sourceBlobID    uuid.UUID
	derivativeID    storage.DerivativeID
}

type exportSource struct {
	revisionID uuid.UUID
	blobID     uuid.UUID
	formatID   string
	mediaType  string
	digest     [sha256.Size]byte
}

// OpenExport applies additions to the current source for a resolved target.
func (s *Service) OpenExport(
	ctx context.Context,
	assetID uuid.UUID,
	viewerID *uuid.UUID,
	target string,
) (ExportFile, error) {
	source, err := s.exportSource(ctx, assetID, viewerID)
	if err != nil {
		return ExportFile{}, err
	}
	exporter, resolvedTarget, supported := s.reg.ResolveExporter(source.formatID, target)
	if !supported {
		artifact, err := s.store.Open(ctx, source.blobID)
		if err != nil {
			return ExportFile{}, fmt.Errorf("open raw export: %w", err)
		}
		return ExportFile{
			Artifact: artifact, MediaType: source.mediaType, Target: format.RawTarget,
			sourceBlobID: source.blobID,
		}, nil
	}
	patch, err := s.filePatch(ctx, assetID, source.revisionID)
	if err != nil {
		return ExportFile{}, err
	}
	media, err := s.exportMedia(ctx, assetID)
	if err != nil {
		return ExportFile{}, err
	}
	if resolvedTarget == format.RawTarget && len(patch) == 0 && len(media) == 0 {
		artifact, err := s.store.Open(ctx, source.blobID)
		if err != nil {
			return ExportFile{}, fmt.Errorf("open raw export: %w", err)
		}
		return ExportFile{
			Artifact: artifact, MediaType: source.mediaType, Target: format.RawTarget,
			sourceBlobID: source.blobID,
		}, nil
	}
	stored, err := s.store.Open(ctx, source.blobID)
	if err != nil {
		return ExportFile{}, fmt.Errorf("open export source: %w", err)
	}
	written, exportErr := exporter.Export(ctx, format.ExportRequest{
		Source: stored, Target: resolvedTarget, Patch: patch, Media: media,
	})
	closeErr := stored.Close()
	if exportErr != nil {
		return ExportFile{}, fmt.Errorf("export %s: %w", resolvedTarget, exportErr)
	}
	if closeErr != nil {
		return ExportFile{}, fmt.Errorf("close export source: %w", closeErr)
	}
	return ExportFile{
		Artifact: io.NopCloser(written.Artifact), MediaType: written.MediaType,
		Extension: written.Extension, Target: resolvedTarget,
		UnembeddedMedia: written.UnembeddedMedia,
		derivativeID:    exportDerivativeID(source.digest, resolvedTarget, patch, media),
	}, nil
}

func (s *Service) exportSource(ctx context.Context, assetID uuid.UUID, viewerID *uuid.UUID) (exportSource, error) {
	var source exportSource
	var digest []byte
	err := s.pool.QueryRow(ctx, `
		select revision.id, revision.blob_id, revision.format, revision.media_type, blob.sha256
		  from assets asset
		  join asset_revisions revision on revision.id = asset.current_revision_id
		  join blobs blob on blob.id = revision.blob_id
		 where asset.id = $1 and asset.deleted_at is null
		   and (asset.withheld_at is null or asset.owner_id = $2)
	`, assetID, viewerID).Scan(
		&source.revisionID, &source.blobID, &source.formatID, &source.mediaType, &digest,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return exportSource{}, ErrNotFound
	}
	if err != nil {
		return exportSource{}, fmt.Errorf("find export source: %w", err)
	}
	if len(digest) != sha256.Size {
		return exportSource{}, fmt.Errorf("find export source: invalid blob digest length %d", len(digest))
	}
	copy(source.digest[:], digest)
	return source, nil
}

func exportDerivativeID(
	source [sha256.Size]byte,
	target string,
	patch format.Patch,
	media []format.ExportMedia,
) storage.DerivativeID {
	hash := sha256.New()
	writeFingerprintPart(hash, []byte(target))
	fields := make([]string, 0, len(patch))
	for field := range patch {
		fields = append(fields, string(field))
	}
	sort.Strings(fields)
	for _, field := range fields {
		writeFingerprintPart(hash, []byte(field))
		writeFingerprintPart(hash, []byte(patch[format.Field(field)]))
	}
	for _, item := range media {
		writeFingerprintPart(hash, []byte(item.Role))
		writeFingerprintPart(hash, []byte(item.MediaType))
		writeFingerprintPart(hash, item.Data)
	}
	return storage.DerivativeID{
		SourceDigest: source,
		Variant:      "export/" + target + "/" + hex.EncodeToString(hash.Sum(nil)),
		Version:      1,
	}
}

func writeFingerprintPart(destination io.Writer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = destination.Write(length[:])
	_, _ = destination.Write(value)
}

func (s *Service) filePatch(ctx context.Context, assetID, revisionID uuid.UUID) (format.Patch, error) {
	rows, err := s.pool.Query(ctx, `
		select field, value
		  from file_field_patches
		 where asset_id = $1
		   and (provenance = 'creator' or revision_id = $2)
		 order by case provenance when 'reconciliation' then 0 else 1 end
	`, assetID, revisionID)
	if err != nil {
		return nil, fmt.Errorf("read file patch: %w", err)
	}
	defer rows.Close()
	patch := make(format.Patch)
	for rows.Next() {
		var field format.Field
		var value string
		if err := rows.Scan(&field, &value); err != nil {
			return nil, fmt.Errorf("scan file patch: %w", err)
		}
		patch[field] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read file patch: %w", err)
	}
	return patch, nil
}

func (s *Service) exportMedia(ctx context.Context, assetID uuid.UUID) ([]format.ExportMedia, error) {
	rows, err := s.pool.Query(ctx, `
		select id, role, blob_id from asset_media where asset_id = $1 order by created_at, id
	`, assetID)
	if err != nil {
		return nil, fmt.Errorf("list export media: %w", err)
	}
	defer rows.Close()
	type foundMedia struct {
		id     uuid.UUID
		role   MediaRole
		blobID uuid.UUID
	}
	var found []foundMedia
	for rows.Next() {
		var item foundMedia
		if err := rows.Scan(&item.id, &item.role, &item.blobID); err != nil {
			return nil, fmt.Errorf("scan export media: %w", err)
		}
		found = append(found, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list export media: %w", err)
	}
	media := make([]format.ExportMedia, 0, len(found))
	for _, item := range found {
		opened, err := s.store.Open(ctx, item.blobID)
		if err != nil {
			return nil, fmt.Errorf("open export media %s: %w", item.id, err)
		}
		data, readErr := io.ReadAll(opened)
		closeErr := opened.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read export media %s: %w", item.id, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close export media %s: %w", item.id, closeErr)
		}
		media = append(media, format.ExportMedia{
			ID: item.id.String(), Role: item.role, MediaType: http.DetectContentType(data), Data: data,
		})
	}
	return media, nil
}
