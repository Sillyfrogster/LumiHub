package migration

import (
	"context"
	"fmt"

	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Corpus is every v1 row the migration reads, with its side tables already attached.
type Corpus struct {
	Rows         []v1.Row
	Recoveries   v1.RecoveryAllowlist
	ClaimedFiles []string
}

// ReadCorpus loads the v1 rows along with theme fonts and verified card recoveries.
func ReadCorpus(ctx context.Context, source *pgxpool.Pool, backup *FileBackup) (Corpus, error) {
	images, err := readCharacterImages(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	characters, err := readCharacters(ctx, source, images)
	if err != nil {
		return Corpus{}, err
	}
	recoveries, err := readVerifiedRecoveries(characters, backup)
	if err != nil {
		return Corpus{}, err
	}
	themes, err := readThemes(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	bundles, err := attachThemeBundles(themes, backup)
	if err != nil {
		return Corpus{}, err
	}
	versions, err := readPresetVersions(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	sealed, err := readSealedBlocks(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	presets, err := readPresets(ctx, source, versions, sealed)
	if err != nil {
		return Corpus{}, err
	}
	lorebooks, err := readLorebooks(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	packs, err := readPacks(ctx, source)
	if err != nil {
		return Corpus{}, err
	}
	rows := make([]v1.Row, 0, len(characters)+len(themes)+len(presets)+len(lorebooks)+len(packs))
	rows = append(rows, characters...)
	rows = append(rows, themes...)
	rows = append(rows, presets...)
	rows = append(rows, lorebooks...)
	rows = append(rows, packs...)
	return Corpus{Rows: rows, Recoveries: recoveries, ClaimedFiles: bundles}, nil
}

func readCharacterImages(
	ctx context.Context,
	source *pgxpool.Pool,
) (map[uuid.UUID][]v1.CharacterImageRow, error) {
	rows, err := source.Query(ctx, `
		select id, character_id, image_type, coalesce(label, ''), file_path,
		       mime_type, file_size, sort_order
		  from character_images
	 order by character_id, sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 character images: %w", err)
	}
	defer rows.Close()
	images := make(map[uuid.UUID][]v1.CharacterImageRow)
	for rows.Next() {
		var characterID uuid.UUID
		var row v1.CharacterImageRow
		if err := rows.Scan(
			&row.ID, &characterID, &row.Type, &row.Label, &row.Path,
			&row.MediaType, &row.ByteSize, &row.Position,
		); err != nil {
			return nil, fmt.Errorf("read a v1 character image: %w", err)
		}
		images[characterID] = append(images[characterID], row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 character images: %w", err)
	}
	return images, nil
}

func readCharacters(
	ctx context.Context,
	source *pgxpool.Pool,
	images map[uuid.UUID][]v1.CharacterImageRow,
) ([]v1.Row, error) {
	rows, err := source.Query(ctx, `
		select id, owner_id, name, description, coalesce(image_path, ''),
		       downloads, views, tags, is_nsfw, created_at, updated_at,
		       coalesce(nickname, ''), personality, scenario, first_mes,
		       alternate_greetings, group_only_greetings, mes_example, creator,
		       creator_notes, character_version, system_prompt,
		       post_history_instructions, coalesce(tagline, ''), character_book,
		       extensions, assets, creation_date, modification_date
		  from characters
	 order by id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 characters: %w", err)
	}
	defer rows.Close()
	var characters []v1.Row
	for rows.Next() {
		var common v1.CommonRow
		var row v1.CharacterRow
		var creationDate, modificationDate pgtype.Int8
		if err := rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Nickname, &row.Personality, &row.Scenario,
			&row.FirstMessage, &row.AlternateGreetings, &row.GroupOnlyGreetings,
			&row.ExampleDialogue, &row.Creator, &row.CreatorNotes, &row.CharacterVersion,
			&row.SystemPrompt, &row.PostHistoryInstructions, &row.Tagline, &row.CharacterBook,
			&row.Extensions, &row.Assets, &creationDate, &modificationDate,
		); err != nil {
			return nil, fmt.Errorf("read a v1 character: %w", err)
		}
		row.Common = common
		row.Images = images[common.ID]
		if creationDate.Valid {
			value := creationDate.Int64
			row.CreationDate = &value
		}
		if modificationDate.Valid {
			value := modificationDate.Int64
			row.ModificationDate = &value
		}
		characters = append(characters, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 characters: %w", err)
	}
	return characters, nil
}

func readPresetVersions(
	ctx context.Context,
	source *pgxpool.Pool,
) (map[uuid.UUID][]v1.PresetVersionRow, error) {
	rows, err := source.Query(ctx, `
		select preset_id, id, version, changelog, snapshot, blocks_added, blocks_removed,
		       variables_added, variables_removed, block_count, variable_count,
		       created_by, created_at
		  from preset_versions
	 order by preset_id, created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 preset versions: %w", err)
	}
	defer rows.Close()
	versions := make(map[uuid.UUID][]v1.PresetVersionRow)
	for rows.Next() {
		var presetID uuid.UUID
		var row v1.PresetVersionRow
		var createdBy pgtype.UUID
		if err := rows.Scan(
			&presetID, &row.ID, &row.Version, &row.Changelog, &row.Snapshot,
			&row.BlocksAdded, &row.BlocksRemoved, &row.VariablesAdded, &row.VariablesRemoved,
			&row.BlockCount, &row.VariableCount, &createdBy, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("read a v1 preset version: %w", err)
		}
		if createdBy.Valid {
			value := uuid.UUID(createdBy.Bytes)
			row.CreatedBy = &value
		}
		versions[presetID] = append(versions[presetID], row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 preset versions: %w", err)
	}
	return versions, nil
}

func readSealedBlocks(
	ctx context.Context,
	source *pgxpool.Pool,
) (map[uuid.UUID][]v1.SealedBlockRow, error) {
	rows, err := source.Query(ctx, `
		select preset_id, id, version, block_key, content,
		       content_sha256, created_by, created_at, updated_at
		  from preset_sealed_blocks
	 order by preset_id, id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 sealed blocks: %w", err)
	}
	defer rows.Close()
	sealed := make(map[uuid.UUID][]v1.SealedBlockRow)
	for rows.Next() {
		var presetID uuid.UUID
		var row v1.SealedBlockRow
		var createdBy pgtype.UUID
		var version pgtype.Text
		if err := rows.Scan(
			&presetID, &row.ID, &version, &row.Key, &row.Content,
			&row.SHA256, &createdBy, &row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("read a v1 sealed block: %w", err)
		}
		if version.Valid {
			value := version.String
			row.Version = &value
		}
		if createdBy.Valid {
			value := uuid.UUID(createdBy.Bytes)
			row.CreatedBy = &value
		}
		sealed[presetID] = append(sealed[presetID], row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 sealed blocks: %w", err)
	}
	return sealed, nil
}

func readPresets(
	ctx context.Context,
	source *pgxpool.Pool,
	versions map[uuid.UUID][]v1.PresetVersionRow,
	sealed map[uuid.UUID][]v1.SealedBlockRow,
) ([]v1.Row, error) {
	rows, err := source.Query(ctx, `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, tags, is_nsfw, created_at, updated_at,
		       preset, coalesce(latest_version, '')
		  from presets
	 order by id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 presets: %w", err)
	}
	defer rows.Close()
	var presets []v1.Row
	for rows.Next() {
		var common v1.CommonRow
		var row v1.PresetRow
		if err := rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Payload, &row.LatestVersion,
		); err != nil {
			return nil, fmt.Errorf("read a v1 preset: %w", err)
		}
		row.Common = common
		row.Versions = versions[common.ID]
		row.SealedBlocks = sealed[common.ID]
		presets = append(presets, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 presets: %w", err)
	}
	return presets, nil
}

func readThemes(ctx context.Context, source *pgxpool.Pool) ([]v1.Row, error) {
	rows, err := source.Query(ctx, `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, tags, is_nsfw, created_at, updated_at,
		       colors, config, coalesce(custom_css, ''), coalesce(asset_bundle_id, '')
		  from themes
	 order by id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 themes: %w", err)
	}
	defer rows.Close()
	var themes []v1.Row
	for rows.Next() {
		var common v1.CommonRow
		var row v1.ThemeRow
		if err := rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Colors, &row.Config,
			&row.CustomCSS, &row.AssetBundleID,
		); err != nil {
			return nil, fmt.Errorf("read a v1 theme: %w", err)
		}
		row.Common = common
		themes = append(themes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 themes: %w", err)
	}
	return themes, nil
}

func readLorebooks(ctx context.Context, source *pgxpool.Pool) ([]v1.Row, error) {
	rows, err := source.Query(ctx, `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, tags, is_nsfw, created_at, updated_at, creator, entries
		  from worldbooks
	 order by id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 lorebooks: %w", err)
	}
	defer rows.Close()
	var lorebooks []v1.Row
	for rows.Next() {
		var common v1.CommonRow
		var row v1.LorebookRow
		if err := rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Creator, &row.Entries,
		); err != nil {
			return nil, fmt.Errorf("read a v1 lorebook: %w", err)
		}
		row.Common = common
		lorebooks = append(lorebooks, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 lorebooks: %w", err)
	}
	return lorebooks, nil
}

func readPacks(ctx context.Context, source *pgxpool.Pool) ([]v1.Row, error) {
	rows, err := source.Query(ctx, `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, is_nsfw, created_at, updated_at, pack_author,
		       coalesce(cover_url, ''), pack_version, pack_extras, lumia_items,
		       loom_items, loom_tools
		  from dlc_packs
	 order by id`)
	if err != nil {
		return nil, fmt.Errorf("read the v1 packs: %w", err)
	}
	defer rows.Close()
	var packs []v1.Row
	for rows.Next() {
		var common v1.CommonRow
		var row v1.PackRow
		if err := rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Author, &row.CoverURL, &row.Version,
			&row.PackExtras, &row.LumiaItems, &row.LoomItems, &row.LoomTools,
		); err != nil {
			return nil, fmt.Errorf("read a v1 pack: %w", err)
		}
		row.Common = common
		packs = append(packs, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the v1 packs: %w", err)
	}
	return packs, nil
}
