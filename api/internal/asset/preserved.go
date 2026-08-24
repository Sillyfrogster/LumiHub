package asset

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PreservedNamespace is one namespace an asset carries, as the creator's panel
// names it. The panel is read-only apart from deletion, so nothing here
// carries a payload.
type PreservedNamespace struct {
	Name  string
	Bytes int
}

// PreservedNamespaces lists the nonempty preserved namespaces for an asset.
func (s *Service) PreservedNamespaces(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) ([]PreservedNamespace, error) {
	originFormat, err := s.preservedAssetOrigin(ctx, ownerID, assetID)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		select namespace, sum(length(payload::text))::bigint, min(payload::text)
		  from asset_preserved_data
		 where asset_id = $1
		 group by namespace
		 order by namespace
	`, assetID)
	if err != nil {
		return nil, fmt.Errorf("read preserved namespaces: %w", err)
	}
	defer rows.Close()

	declaration, declared := s.reg.Declaration(originFormat)
	found := make([]PreservedNamespace, 0)
	for rows.Next() {
		var name, sample string
		var size int64
		if err := rows.Scan(&name, &size, &sample); err != nil {
			return nil, fmt.Errorf("read preserved namespace: %w", err)
		}
		if declared && declaration.RecordsNothing(name, []byte(sample)) {
			continue
		}
		found = append(found, PreservedNamespace{Name: name, Bytes: int(size)})
	}
	return found, rows.Err()
}

// DeletePreservedNamespace removes one namespace from an asset for good. A
// creator who has moved off a platform can take its provenance out of their
// downloads, and the lossless rule binds Illarin rather than the creator.
func (s *Service) DeletePreservedNamespace(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
	namespace string,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := lockEditableAsset(ctx, tx, ownerID, assetID); err != nil {
		return err
	}
	fingerprint, err := s.contentFingerprint(ctx, tx, assetID)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `
		delete from asset_preserved_data where asset_id = $1 and namespace = $2
	`, assetID, namespace)
	if err != nil {
		return fmt.Errorf("delete preserved %s: %w", namespace, err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := s.moveContentGeneration(ctx, tx, assetID, fingerprint); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// preservedAssetOrigin returns the format an asset arrived in, which is the
// module whose boilerplate list governs the panel. An asset built from nothing
// has no origin and carries no preserved data either.
func (s *Service) preservedAssetOrigin(
	ctx context.Context,
	ownerID uuid.UUID,
	assetID uuid.UUID,
) (string, error) {
	var origin *string
	err := s.pool.QueryRow(ctx, `
		select origin_format
		  from assets
		 where id = $1 and owner_id = $2 and deleted_at is null
	`, assetID, ownerID).Scan(&origin)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read asset origin: %w", err)
	}
	if origin == nil {
		return "", nil
	}
	return *origin, nil
}

// dropUnownedPreservedData removes records detached by an editing operation.
func dropUnownedPreservedData(
	ctx context.Context,
	tx pgx.Tx,
	assetID uuid.UUID,
	blocks []block.Block,
) error {
	owners := make([]uuid.UUID, 0)
	for _, holder := range blocks {
		for _, element := range holder.Elements {
			owners = append(owners, element.ID)
			owners = append(owners, block.ItemIDs(element.Content)...)
		}
	}
	if _, err := tx.Exec(ctx, `
		delete from asset_preserved_data
		 where asset_id = $1 and owner_kind <> $2 and owner_id <> all($3)
	`, assetID, string(format.OwnerAsset), owners); err != nil {
		return fmt.Errorf("drop preserved data with no owner: %w", err)
	}
	return nil
}
