package asset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"slices"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/protected"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Content generation advances when a change alters downloadable bytes.

// contentFingerprint excludes page arrangement and digests elements by ID.
func (s *Service) contentFingerprint(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
) (string, error) {
	digest := sha256.New()

	var kind, name, blurb, assetVersion, creditedAuthor, nickname string
	var origin pgtype.Text
	var cover pgtype.UUID
	err := q.QueryRow(ctx, `
		select kind, origin_format, name, blurb, asset_version, credited_author,
		       nickname, cover_media_id
		  from assets
		 where id = $1 and deleted_at is null
	`, assetID).Scan(&kind, &origin, &name, &blurb, &assetVersion, &creditedAuthor,
		&nickname, &cover)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read the asset to fingerprint: %w", err)
	}
	// The origin decides which writers are offered and which preserved
	// namespaces travel, so a moved origin is a moved file.
	fmt.Fprintf(digest, "asset\x00%s\x00%s\n", kind, origin.String)

	values := map[format.HeaderField]string{
		format.HeaderName:           name,
		format.HeaderBlurb:          blurb,
		format.HeaderAssetVersion:   assetVersion,
		format.HeaderCreditedAuthor: creditedAuthor,
		format.HeaderNickname:       nickname,
	}
	for _, field := range s.reg.ExportedHeaderFields(kind) {
		fmt.Fprintf(digest, "header\x00%s\x00%s\n", field, values[field])
	}

	blocks, err := readBlocks(ctx, q, assetID)
	if err != nil {
		return "", err
	}
	if err := protected.RestorePromptFragments(ctx, q, assetID, blocks); err != nil {
		return "", err
	}
	elements := make([]block.Element, 0)
	for _, holder := range blocks {
		elements = append(elements, holder.Elements...)
	}
	slices.SortFunc(elements, func(a, b block.Element) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})
	for _, element := range elements {
		// An element the creator has left empty is one no writer writes, so a
		// block added and not yet filled in changes no file.
		if element.Content == nil || element.Content.Empty() {
			continue
		}
		content, err := element.ContentJSON()
		if err != nil {
			return "", fmt.Errorf("fingerprint the %s element: %w", element.Role, err)
		}
		fmt.Fprintf(digest, "element\x00%s\x00%s\x00%s\n", element.ID, element.Role, content)
	}

	if err := fingerprintPreserved(ctx, q, assetID, digest); err != nil {
		return "", err
	}
	if err := fingerprintPictures(ctx, q, assetID, elements, uuidOrNil(cover), digest); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// fingerprintPreserved digests what the asset kept from the file it arrived
// as, exactly as it is stored. A namespace deleted or replaced is a namespace
// the next download will not carry.
func fingerprintPreserved(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
	digest hash.Hash,
) error {
	rows, err := q.Query(ctx, `
		select owner_kind, owner_id, namespace, payload::text
		  from asset_preserved_data
		 where asset_id = $1
		 order by owner_kind, owner_id, namespace
	`, assetID)
	if err != nil {
		return fmt.Errorf("read preserved data to fingerprint: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ownerKind, namespace, payload string
		var ownerID uuid.UUID
		if err := rows.Scan(&ownerKind, &ownerID, &namespace, &payload); err != nil {
			return fmt.Errorf("read a preserved row to fingerprint: %w", err)
		}
		fmt.Fprintf(digest, "preserved\x00%s\x00%s\x00%s\x00%s\n",
			ownerKind, ownerID, namespace, payload)
	}
	return rows.Err()
}

// fingerprintPictures digests the cover and every image an element points at,
// which are the pictures a writer may put in the file. The blob behind each one
// stands for its bytes, so a replaced picture reads as a changed file without
// any of them being opened.
func fingerprintPictures(
	ctx context.Context,
	q db.DBTX,
	assetID uuid.UUID,
	elements []block.Element,
	cover *uuid.UUID,
	digest hash.Hash,
) error {
	wanted := make([]uuid.UUID, 0)
	if cover != nil {
		wanted = append(wanted, *cover)
	}
	for _, element := range elements {
		set, isSet := element.Content.(block.ImageSet)
		if !isSet {
			continue
		}
		for _, image := range set.Images {
			wanted = append(wanted, image.MediaID)
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	rows, err := q.Query(ctx, `
		select id, blob_id, is_current
		  from asset_media
		 where asset_id = $1 and id = any($2)
		 order by id
	`, assetID, wanted)
	if err != nil {
		return fmt.Errorf("read pictures to fingerprint: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var mediaID, blobID uuid.UUID
		var current bool
		if err := rows.Scan(&mediaID, &blobID, &current); err != nil {
			return fmt.Errorf("read a picture to fingerprint: %w", err)
		}
		fmt.Fprintf(digest, "picture\x00%s\x00%s\x00%t\n", mediaID, blobID, current)
	}
	return rows.Err()
}

// moveContentGeneration advances the counter where the change just made would
// alter a file a reader could download, and leaves it alone otherwise. The
// before fingerprint has to have been taken in this transaction, under the row
// lock the change itself holds.
func (s *Service) moveContentGeneration(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	before string,
) error {
	after, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return err
	}
	if after == before {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		update assets set content_generation = content_generation + 1 where id = $1
	`, assetID); err != nil {
		return fmt.Errorf("move the content generation: %w", err)
	}
	return nil
}
