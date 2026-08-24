package migration

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Settings is everything one migration run reads from and writes to.
type Settings struct {
	Source  *pgxpool.Pool
	Target  *pgxpool.Pool
	Assets  *asset.Service
	Backup  *FileBackup
	Fetcher Fetcher
}

// Report is what one run carried across.
type Report struct {
	Assets      int
	Kinds       map[string]int
	Images      int
	LegacyPaths int
	Preserved   int
	BelowFloor  int
	Staging     StageReport
	Exceptions  []Exception
}

// Run stages every blob and external image first, then commits every asset in one transaction.
func Run(ctx context.Context, settings Settings) (Report, error) {
	ledger, err := NewLedger(v1.Module{}.Declaration().Anomalies)
	if err != nil {
		return Report{}, err
	}
	if err := requireEmptyAssetTarget(ctx, settings.Target); err != nil {
		return Report{}, err
	}
	corpus, err := ReadCorpus(ctx, settings.Source, settings.Backup)
	if err != nil {
		return Report{}, err
	}
	reader := v1.Module{Recoveries: corpus.Recoveries}
	results := make([]v1.Result, 0, len(corpus.Rows))
	for _, row := range corpus.Rows {
		result, err := reader.Read(ctx, row)
		if err != nil {
			return Report{}, fmt.Errorf("read a v1 asset: %w", err)
		}
		if err := recordReadEvents(result, ledger); err != nil {
			return Report{}, err
		}
		results = append(results, result)
	}

	staged, staging, err := Stage(ctx, settings, corpus, results, ledger)
	if err != nil {
		return Report{}, err
	}
	report, err := commit(ctx, settings, corpus, results, staged, ledger)
	report.Staging = staging
	return report, err
}

func requireEmptyAssetTarget(ctx context.Context, target *pgxpool.Pool) error {
	empty, err := db.New(target).MigrationAssetTargetIsEmpty(ctx)
	if err != nil {
		return fmt.Errorf("check that the target catalog is empty: %w", err)
	}
	if !empty {
		return errors.New("the target catalog already holds assets, so the migration will not run")
	}
	return nil
}

// recordReadEvents turns what the reader noticed into ledger entries, so the module declares the policy and the run applies it.
func recordReadEvents(result v1.Result, ledger *Ledger) error {
	for _, event := range result.Events {
		assetID := result.AssetID
		entry := Exception{Subject: result.Parsed.Header.Name, AssetID: &assetID}
		switch event.Kind {
		case v1.RecoveredAlternateGreeting:
			entry.Kind = "recovered_alternate_greeting"
			entry.Detail = "one alternate greeting came back from the creator's surviving card"
		case v1.RecoveredGalleryNames:
			continue
		case v1.GalleryAssetsMismatch:
			entry.Kind = "gallery_assets_mismatch"
			entry.Detail = "the card's asset names did not line up with the gallery, so the array is preserved instead"
		case v1.MissingThemeStatusColors:
			entry.Kind = "missing_theme_status_colors"
			entry.Detail = "the theme carries no status colour set, and the required palette is still complete"
		default:
			return fmt.Errorf("the v1 reader reported %q, which the run does not classify", event.Kind)
		}
		if err := ledger.Raise(entry); err != nil {
			return err
		}
	}
	return nil
}

// placeAsset turns one read row into the blocks its kind catalog declares, choosing nothing itself.
func placeAsset(
	result v1.Result,
	staged *Staged,
	ledger *Ledger,
) (asset.MigratedAsset, error) {
	elements, images, cover, err := placeMedia(result, staged)
	if err != nil {
		return asset.MigratedAsset{}, err
	}
	if err := block.ValidateContentLimits(elements); err != nil {
		return asset.MigratedAsset{}, fmt.Errorf("v1 asset %s: %w", result.AssetID, err)
	}
	blocks, err := block.Place(result.Parsed.Kind, elements)
	if err != nil {
		if raiseErr := ledger.Raise(Exception{
			Kind: "core_role_unreadable", Subject: result.Parsed.Header.Name,
			Detail: err.Error(),
		}); raiseErr != nil {
			return asset.MigratedAsset{}, raiseErr
		}
		return asset.MigratedAsset{}, err
	}
	isNSFW := false
	if result.Parsed.IsNSFW != nil {
		isNSFW = *result.Parsed.IsNSFW
	}
	tags := result.Parsed.Tags
	if tags == nil {
		tags = []string{}
	}
	return asset.MigratedAsset{
		ID: result.AssetID, OwnerID: result.OwnerID, Kind: result.Parsed.Kind,
		Origin: result.OriginFormat, Header: result.Parsed.Header, Tags: tags,
		IsNSFW: isNSFW, CreatedAt: result.CreatedAt, Blocks: blocks,
		Remainder: result.Parsed.Remainder, Protected: result.Parsed.Protected,
		Images: images, CoverID: cover,
	}, nil
}

// placeMedia binds every staged picture to the element that points at it and drops the ones that never arrived.
func placeMedia(
	result v1.Result,
	staged *Staged,
) ([]block.Element, []asset.MigratedImage, *uuid.UUID, error) {
	images := make([]asset.MigratedImage, 0, len(result.Media)+1)
	present := make(map[uuid.UUID]struct{}, len(result.Media))
	for _, source := range result.Media {
		image, found := staged.Image(fileSource(cleanBackupPath(source.Path)))
		if !found {
			continue
		}
		images = append(images, asset.MigratedImage{
			ID: source.MediaID, BlobID: image.BlobID, Role: source.Role,
			Width: image.Width, Height: image.Height,
		})
		present[source.MediaID] = struct{}{}
	}
	var cover *uuid.UUID
	if result.Cover != nil {
		if image, found := staged.Image(fileSource(cleanBackupPath(result.Cover.Path))); found {
			images = append(images, asset.MigratedImage{
				ID: result.Cover.MediaID, BlobID: image.BlobID, Role: result.Cover.Role,
				Width: image.Width, Height: image.Height,
			})
			id := result.Cover.MediaID
			cover = &id
		}
	}
	elements, external := placeExternalMedia(result, staged, &images)
	if external != nil {
		cover = external
	}
	return withoutMissingImages(elements, present), images, cover, nil
}

// placeExternalMedia gives a fetched pack image to the record that named it and a fetched cover to the asset.
func placeExternalMedia(
	result v1.Result,
	staged *Staged,
	images *[]asset.MigratedImage,
) ([]block.Element, *uuid.UUID) {
	elements := result.Parsed.Elements
	var cover *uuid.UUID
	for _, external := range result.ExternalMedia {
		image, found := staged.Image(external.URL)
		if !found {
			continue
		}
		mediaID := uuid.New()
		*images = append(*images, asset.MigratedImage{
			ID: mediaID, BlobID: image.BlobID, Role: external.Role,
			Width: image.Width, Height: image.Height,
		})
		switch external.Owner {
		case v1.ExternalCover:
			cover = &mediaID
		case v1.ExternalPackItem:
			elements = withPackItemImage(elements, external.OwnerID, mediaID)
		}
	}
	return elements, cover
}

func withPackItemImage(
	elements []block.Element,
	recordID uuid.UUID,
	mediaID uuid.UUID,
) []block.Element {
	for i := range elements {
		list, ok := elements[i].Content.(block.RecordList)
		if !ok {
			continue
		}
		records := append([]block.LumiaRecord(nil), list.Records...)
		for at := range records {
			if records[at].ID != recordID {
				continue
			}
			id := mediaID
			records[at].AvatarURL = &id
			list.Records = records
			elements[i].Content = list
			return elements
		}
	}
	return elements
}

// withoutMissingImages drops an image item whose picture never arrived, so a page never points at bytes Illarin does not hold.
func withoutMissingImages(
	elements []block.Element,
	present map[uuid.UUID]struct{},
) []block.Element {
	kept := make([]block.Element, 0, len(elements))
	for _, element := range elements {
		set, ok := element.Content.(block.ImageSet)
		if !ok {
			kept = append(kept, element)
			continue
		}
		items := make([]block.ImageItem, 0, len(set.Images))
		for _, item := range set.Images {
			if _, held := present[item.MediaID]; held {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			continue
		}
		set.Images = items
		element.Content = set
		kept = append(kept, element)
	}
	return kept
}
