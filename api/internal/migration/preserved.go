package migration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/db"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// writePreservedRecords keeps every v1 row Illarin does not model, because deleting is irreversible and keeping commits Illarin to nothing.
func writePreservedRecords(
	ctx context.Context,
	tx pgx.Tx,
	settings Settings,
	results []v1.Result,
	handles map[uuid.UUID]string,
) (int, error) {
	assets := make(map[uuid.UUID]struct{}, len(results))
	for _, result := range results {
		assets[result.AssetID] = struct{}{}
	}
	written := 0
	for _, result := range results {
		for _, record := range result.PreservedRecords {
			if err := writePreservedRecord(ctx, tx, preservedRecord{
				Table: record.Table, SourceID: record.SourceID,
				AssetID: &record.AssetID, OwnerID: &record.OwnerID, Payload: record.Payload,
			}); err != nil {
				return 0, err
			}
			written++
		}
		for _, sealed := range result.SealedBlocks {
			payload, err := json.Marshal(sealedBlockPayload(sealed))
			if err != nil {
				return 0, fmt.Errorf("preserve a sealed block: %w", err)
			}
			if err := writePreservedRecord(ctx, tx, preservedRecord{
				Table: "preset_sealed_blocks", SourceID: sealed.ID.String(),
				AssetID: &sealed.AssetID, OwnerID: &sealed.OwnerID, Payload: payload,
			}); err != nil {
				return 0, err
			}
			written++
		}
	}
	for _, table := range preservedTables() {
		count, err := preserveSourceTable(ctx, tx, settings, table, assets, handles)
		if err != nil {
			return 0, err
		}
		written += count
	}
	return written, nil
}

type preservedRecord struct {
	Table    string
	SourceID string
	AssetID  *uuid.UUID
	OwnerID  *uuid.UUID
	Payload  json.RawMessage
}

func writePreservedRecord(ctx context.Context, tx pgx.Tx, record preservedRecord) error {
	params := db.InsertPreservedRecordParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		SourceTable: record.Table,
		SourceID:    record.SourceID,
		Payload:     record.Payload,
	}
	if record.AssetID != nil && *record.AssetID != uuid.Nil {
		params.AssetID = pgtype.UUID{Bytes: *record.AssetID, Valid: true}
	}
	if record.OwnerID != nil && *record.OwnerID != uuid.Nil {
		params.OwnerID = pgtype.UUID{Bytes: *record.OwnerID, Valid: true}
	}
	if err := db.New(tx).InsertPreservedRecord(ctx, params); err != nil {
		return fmt.Errorf("preserve %s %s: %w", record.Table, record.SourceID, err)
	}
	return nil
}

// preserveSourceTable copies a whole v1 table across, binding each row to the asset and account it names where they resolve.
func preserveSourceTable(
	ctx context.Context,
	tx pgx.Tx,
	settings Settings,
	table string,
	assets map[uuid.UUID]struct{},
	handles map[uuid.UUID]string,
) (int, error) {
	owner := map[string]string{"favorites": "user_id", "comments": "author_id"}[table]
	if owner == "" {
		return 0, fmt.Errorf("v1 table %s has no owner column declared", table)
	}
	rows, err := settings.Source.Query(ctx, fmt.Sprintf(
		`select id, asset_id, %s, to_jsonb(source) from %s source order by id`, owner, table,
	))
	if err != nil {
		return 0, fmt.Errorf("read the v1 %s: %w", table, err)
	}
	defer rows.Close()
	held := make([]preservedRecord, 0)
	for rows.Next() {
		var id uuid.UUID
		var assetID, ownerID pgtype.UUID
		var payload []byte
		if err := rows.Scan(&id, &assetID, &ownerID, &payload); err != nil {
			return 0, fmt.Errorf("read a v1 %s row: %w", table, err)
		}
		record := preservedRecord{Table: table, SourceID: id.String(), Payload: payload}
		if bound := uuid.UUID(assetID.Bytes); assetID.Valid {
			if _, resolves := assets[bound]; resolves {
				record.AssetID = &bound
			}
		}
		if bound := uuid.UUID(ownerID.Bytes); ownerID.Valid {
			if _, resolves := handles[bound]; resolves {
				record.OwnerID = &bound
			}
		}
		held = append(held, record)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read the v1 %s: %w", table, err)
	}
	for _, record := range held {
		if err := writePreservedRecord(ctx, tx, record); err != nil {
			return 0, err
		}
	}
	return len(held), nil
}

func sealedBlockPayload(sealed v1.SealedBlock) map[string]any {
	payload := map[string]any{
		"id":             sealed.ID.String(),
		"preset_id":      sealed.AssetID.String(),
		"block_key":      sealed.Key,
		"content":        sealed.Content,
		"content_sha256": sealed.SHA256,
		"created_at":     sealed.CreatedAt,
		"updated_at":     sealed.UpdatedAt,
	}
	if sealed.Version != nil {
		payload["version"] = *sealed.Version
	}
	if sealed.CreatedBy != nil {
		payload["created_by"] = sealed.CreatedBy.String()
	}
	return payload
}

// preservedRecordCount is what the source asks for, so a short count fails rather than being explained afterwards.
func preservedRecordCount(results []v1.Result, source map[string]int) int {
	total := 0
	for _, result := range results {
		total += len(result.PreservedRecords) + len(result.SealedBlocks)
	}
	for _, count := range source {
		total += count
	}
	return total
}

func sourceTableCounts(ctx context.Context, settings Settings) (map[string]int, error) {
	counts := make(map[string]int, len(preservedTables()))
	for _, table := range preservedTables() {
		var count int
		if err := settings.Source.QueryRow(ctx,
			"select count(*) from "+table).Scan(&count); err != nil {
			return nil, fmt.Errorf("count the v1 %s: %w", table, err)
		}
		counts[table] = count
	}
	return counts, nil
}
