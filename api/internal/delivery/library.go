package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
	"github.com/google/uuid"
)

// Sync records what one instance reports installing, by immutable id so a rename breaks nothing.
func (s *Service) Sync(
	ctx context.Context,
	instance linking.Instance,
	report LibraryReport,
) (LibraryResult, error) {
	entries, removed, err := s.readReport(report)
	if err != nil {
		return LibraryResult{}, err
	}
	action, limit := actionSyncPart, int32(syncPartLimit)
	if report.Snapshot {
		action, limit = actionSyncWhole, int32(syncWholeLimit)
	}
	if err := s.instances.Throttle(
		ctx, action, instance.ID.String(), limit, time.Hour,
	); err != nil {
		return LibraryResult{}, throttled(err)
	}

	assetIDs := make([]uuid.UUID, 0, len(entries))
	generations := make([]int32, 0, len(entries))
	for _, entry := range entries {
		assetIDs = append(assetIDs, entry.AssetID)
		reported := int32(0)
		if entry.ContentGeneration != nil {
			reported = int32(*entry.ContentGeneration)
		}
		generations = append(generations, reported)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return LibraryResult{}, fmt.Errorf("begin a library report: %w", err)
	}
	defer tx.Rollback(ctx)
	queries := db.New(tx)
	accepted, err := queries.ReportLibraryEntries(ctx, db.ReportLibraryEntriesParams{
		InstanceID: uuidValue(instance.ID), AssetIds: uuidValues(assetIDs),
		Generations: generations,
	})
	if err != nil {
		return LibraryResult{}, fmt.Errorf("record a library report: %w", err)
	}
	var dropped int64
	if report.Snapshot {
		dropped, err = queries.PruneLibraryToSnapshot(ctx, db.PruneLibraryToSnapshotParams{
			InstanceID: uuidValue(instance.ID), AssetIds: uuidValues(assetIDs),
		})
	} else if len(removed) > 0 {
		dropped, err = queries.RemoveLibraryEntries(ctx, db.RemoveLibraryEntriesParams{
			InstanceID: uuidValue(instance.ID), AssetIds: uuidValues(removed),
		})
	}
	if err != nil {
		return LibraryResult{}, fmt.Errorf("remove library entries: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LibraryResult{}, fmt.Errorf("commit a library report: %w", err)
	}
	return LibraryResult{
		Accepted: int(accepted), Removed: int(dropped),
		Ignored: len(entries) - int(accepted),
	}, nil
}

// readReport bounds and de-duplicates a report before any of it reaches the database.
func (s *Service) readReport(report LibraryReport) ([]LibraryEntry, []uuid.UUID, error) {
	if len(report.Entries) > s.settings.MaxLibraryEntries ||
		len(report.Removed) > s.settings.MaxLibraryEntries {
		return nil, nil, ErrLibraryTooLarge
	}
	if report.Snapshot && len(report.Removed) > 0 {
		return nil, nil, ErrLibraryReport
	}
	entries := make([]LibraryEntry, 0, len(report.Entries))
	seen := make(map[uuid.UUID]struct{}, len(report.Entries))
	for _, entry := range report.Entries {
		if entry.ContentGeneration != nil && *entry.ContentGeneration < 1 {
			return nil, nil, ErrLibraryReport
		}
		if _, repeated := seen[entry.AssetID]; repeated {
			return nil, nil, ErrLibraryReport
		}
		seen[entry.AssetID] = struct{}{}
		entries = append(entries, entry)
	}
	removed := make([]uuid.UUID, 0, len(report.Removed))
	for _, assetID := range report.Removed {
		if _, installed := seen[assetID]; installed {
			return nil, nil, ErrLibraryReport
		}
		removed = append(removed, assetID)
	}
	return entries, removed, nil
}

// LibraryCountsByInstance is how much of each mirror is behind the catalog, as settings shows it.
func (s *Service) LibraryCountsByInstance(
	ctx context.Context,
	userID uuid.UUID,
) (map[uuid.UUID]LibraryCounts, error) {
	rows, err := db.New(s.pool).InstanceLibraryCounts(ctx, uuidValue(userID))
	if err != nil {
		return nil, fmt.Errorf("count installed assets: %w", err)
	}
	counts := make(map[uuid.UUID]LibraryCounts, len(rows))
	for _, row := range rows {
		counts[uuid.UUID(row.InstanceID.Bytes)] = LibraryCounts{
			Installed: int(row.Installed), UpdatesAvailable: int(row.UpdatesAvailable),
		}
	}
	return counts, nil
}
