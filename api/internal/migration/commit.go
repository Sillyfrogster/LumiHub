package migration

import (
	"context"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// commit writes every asset in one transaction, so an abort leaves only the unreferenced blobs the sweep collects.
func commit(
	ctx context.Context,
	settings Settings,
	corpus Corpus,
	results []v1.Result,
	staged *Staged,
	ledger *Ledger,
) (Report, error) {
	handles, err := ownerHandles(ctx, settings.Target)
	if err != nil {
		return Report{}, err
	}
	sourceCounts, err := sourceTableCounts(ctx, settings)
	if err != nil {
		return Report{}, err
	}
	var priorExceptions int
	if err := settings.Target.QueryRow(ctx,
		`select count(*) from migration_exceptions`).Scan(&priorExceptions); err != nil {
		return Report{}, fmt.Errorf("count the ledger before the run: %w", err)
	}
	tx, err := settings.Target.Begin(ctx)
	if err != nil {
		return Report{}, fmt.Errorf("begin the migration: %w", err)
	}
	defer tx.Rollback(ctx)

	report, candidates, err := writeAssets(ctx, tx, settings, results, staged, handles, ledger)
	if err != nil {
		return Report{}, err
	}

	written, err := writeLegacyPaths(ctx, tx, candidates, ledger)
	if err != nil {
		return Report{}, err
	}
	report.LegacyPaths = written

	preserved, err := writePreservedRecords(ctx, tx, settings, results, handles)
	if err != nil {
		return Report{}, err
	}
	report.Preserved = preserved

	if err := ledger.Persist(ctx, tx); err != nil {
		return Report{}, err
	}
	if err := reconcile(ctx, tx, expectation{
		Assets: len(corpus.Rows), Images: report.Images, LegacyPaths: report.LegacyPaths,
		Preserved:  preservedRecordCount(results, sourceCounts),
		Exceptions: priorExceptions + len(ledger.Entries()),
	}, ledger); err != nil {
		return Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Report{}, fmt.Errorf("commit the migration: %w", err)
	}
	report.Exceptions = ledger.Entries()
	return report, nil
}

// writeAssets turns every read row into asset rows and collects each one's claim on a v1 address.
func writeAssets(
	ctx context.Context,
	tx pgx.Tx,
	settings Settings,
	results []v1.Result,
	staged *Staged,
	handles map[uuid.UUID]string,
	ledger *Ledger,
) (Report, []legacyCandidate, error) {
	report := Report{Kinds: map[string]int{}}
	candidates := make([]legacyCandidate, 0, len(results))
	for _, result := range results {
		handle, resolved := handles[result.OwnerID]
		if !resolved {
			return Report{}, nil, ledger.Raise(Exception{
				Kind: "owner_unresolved", Subject: result.Parsed.Header.Name,
				Detail: fmt.Sprintf("account %s is not in the migrated accounts", result.OwnerID),
			})
		}
		one, err := placeAsset(result, staged, ledger)
		if err != nil {
			return Report{}, nil, err
		}
		if err := settings.Assets.WriteMigratedAsset(ctx, tx, one); err != nil {
			return Report{}, nil, err
		}
		if err := writeLegacyCounters(ctx, tx, result); err != nil {
			return Report{}, nil, err
		}
		if err := recordShortfall(one, ledger); err != nil {
			return Report{}, nil, err
		}
		if !asset.Ready(shortfallOf(one)) {
			report.BelowFloor++
		}
		report.Assets++
		report.Kinds[one.Kind]++
		report.Images += len(one.Images)
		candidates = append(candidates, legacyCandidate{
			AssetID: one.ID, Author: one.Header.CreditedAuthor, Handle: handle,
			Name: one.Header.Name, CreatedAt: one.CreatedAt,
		})
	}
	return report, candidates, nil
}

func ownerHandles(ctx context.Context, target *pgxpool.Pool) (map[uuid.UUID]string, error) {
	rows, err := target.Query(ctx, `select id, username from users`)
	if err != nil {
		return nil, fmt.Errorf("read the migrated accounts: %w", err)
	}
	defer rows.Close()
	handles := make(map[uuid.UUID]string)
	for rows.Next() {
		var id uuid.UUID
		var handle string
		if err := rows.Scan(&id, &handle); err != nil {
			return nil, fmt.Errorf("read a migrated account: %w", err)
		}
		handles[id] = handle
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the migrated accounts: %w", err)
	}
	return handles, nil
}

func shortfallOf(one asset.MigratedAsset) []asset.ReadinessItem {
	isNSFW := one.IsNSFW
	return asset.MigratedShortfall(one.Kind, one.Header.Name, &isNSFW, one.Blocks)
}

// recordShortfall ledgers an asset today's floor would have refused, because migration is not a publish.
func recordShortfall(one asset.MigratedAsset, ledger *Ledger) error {
	missing := shortfallOf(one)
	if len(missing) == 0 {
		return nil
	}
	for _, item := range missing {
		if item.Met {
			continue
		}
		assetID := one.ID
		if err := ledger.Raise(Exception{
			Kind: "below_publish_floor", Subject: one.Header.Name,
			Detail:  fmt.Sprintf("the asset arrived published without %s", item.Label),
			AssetID: &assetID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeLegacyCounters(ctx context.Context, tx pgx.Tx, result v1.Result) error {
	return db.New(tx).InsertLegacyCounters(ctx, db.InsertLegacyCountersParams{
		AssetID:     pgtype.UUID{Bytes: result.AssetID, Valid: true},
		V1Downloads: int32(result.Legacy.Downloads),
		V1Views:     int32(result.Legacy.Views),
		V1UpdatedAt: pgtype.Timestamptz{Time: result.Legacy.UpdatedAt, Valid: true},
	})
}

func writeLegacyPaths(
	ctx context.Context,
	tx pgx.Tx,
	candidates []legacyCandidate,
	ledger *Ledger,
) (int, error) {
	paths, displaced := resolveLegacyPaths(candidates)
	queries := db.New(tx)
	for _, path := range paths {
		if err := queries.InsertLegacyPath(ctx, db.InsertLegacyPathParams{
			Path:    path.Path,
			AssetID: pgtype.UUID{Bytes: path.AssetID, Valid: true},
		}); err != nil {
			return 0, fmt.Errorf("store the v1 address %s: %w", path.Path, err)
		}
	}
	for _, candidate := range displaced {
		assetID := candidate.AssetID
		if err := ledger.Raise(Exception{
			Kind: "slug_collision", Subject: candidate.address(),
			Detail:  "an older asset already answers for this v1 address, so this one gets no redirect",
			AssetID: &assetID,
		}); err != nil {
			return 0, err
		}
	}
	return len(paths), nil
}
