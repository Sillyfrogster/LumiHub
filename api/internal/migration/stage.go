package migration

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/db"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ownedBackupDirectories are the parts of v1's uploads directory this migration owns, where a file no row names is an orphan.
var ownedBackupDirectories = []string{
	"uploads/characters",
	"uploads/characters/images",
	"uploads/themes",
	"uploads/themes/bundles",
	"uploads/presets",
	"uploads/worldbooks",
}

// Staged is every blob phase one put on disk, addressed by the stored path or URL that named it.
type Staged struct {
	images map[string]asset.StagedImage
}

// Image returns what a stored path or URL was staged as.
func (staged *Staged) Image(source string) (asset.StagedImage, bool) {
	image, found := staged.images[source]
	return image, found
}

// StageReport says what phase one did, so a re-run can be seen to re-fetch nothing.
type StageReport struct {
	Stored  int
	Fetched int
	Reused  int
	Failed  int
}

// Stage puts every blob and external image on disk and writes nothing to the asset tables, reusing what content addressing already holds.
func Stage(
	ctx context.Context,
	settings Settings,
	corpus Corpus,
	results []v1.Result,
	ledger *Ledger,
) (*Staged, StageReport, error) {
	staged := &Staged{images: make(map[string]asset.StagedImage)}
	report := StageReport{}
	known, err := db.New(settings.Target).StagedMedia(ctx)
	if err != nil {
		return nil, report, fmt.Errorf("read what is already staged: %w", err)
	}
	for _, row := range known {
		staged.images[row.Source] = asset.StagedImage{
			BlobID: uuid.UUID(row.BlobID.Bytes),
			Width:  int(row.Width), Height: int(row.Height),
		}
	}

	if err := stageBackupFiles(ctx, settings, corpus, results, staged, &report, ledger); err != nil {
		return nil, report, err
	}
	if err := fetchExternalImages(ctx, settings, results, staged, &report, ledger); err != nil {
		return nil, report, err
	}
	return staged, report, nil
}

// wantedFiles indexes the files the run stores and the ones it only proves are there.
type wantedFiles struct {
	stored     *backupIndex
	paths      []string
	referenced *backupIndex
	references []referencedFile
	claimed    map[string]struct{}
}

type referencedFile struct {
	Path    string
	AssetID uuid.UUID
}

// filesWantedFrom indexes every stored path a read row names, and every path a row only points at.
func filesWantedFrom(corpus Corpus, results []v1.Result) *wantedFiles {
	wanted := &wantedFiles{
		stored: newBackupIndex(), paths: make([]string, 0, 512),
		referenced: newBackupIndex(), references: make([]referencedFile, 0, len(corpus.Rows)),
		claimed: make(map[string]struct{}, len(corpus.ClaimedFiles)),
	}
	claim := func(stored string) {
		if cleaned := cleanBackupPath(stored); cleaned != "" {
			wanted.stored.want(cleaned, len(wanted.paths))
			wanted.paths = append(wanted.paths, cleaned)
		}
	}
	for _, result := range results {
		if result.Cover != nil {
			claim(result.Cover.Path)
		}
		for _, source := range result.Media {
			claim(source.Path)
		}
	}
	for _, row := range corpus.Rows {
		common := v1.Common(row)
		cleaned := cleanBackupPath(common.ImagePath)
		if cleaned == "" {
			continue
		}
		wanted.referenced.want(cleaned, len(wanted.references))
		wanted.references = append(wanted.references, referencedFile{Path: cleaned, AssetID: common.ID})
	}
	for _, name := range corpus.ClaimedFiles {
		wanted.claimed[name] = struct{}{}
	}
	return wanted
}

// stageBackupFiles walks the archive once, storing the files rows name, proving the ones they only reference, and ledgering the rest.
func stageBackupFiles(
	ctx context.Context,
	settings Settings,
	corpus Corpus,
	results []v1.Result,
	staged *Staged,
	report *StageReport,
	ledger *Ledger,
) error {
	wanted := filesWantedFrom(corpus, results)
	found := make([]bool, len(wanted.paths))
	present := make([]bool, len(wanted.references))
	orphans := make([]string, 0)
	err := settings.Backup.each(func(entry backupEntry) error {
		named := false
		if at, ok := wanted.referenced.find(entry.Name); ok {
			present[at] = true
			named = true
		}
		at, ok := wanted.stored.find(entry.Name)
		if !ok {
			_, held := wanted.claimed[entry.Name]
			if !named && !held && ownedBackupFile(entry.Name) {
				orphans = append(orphans, entry.Name)
			}
			return nil
		}
		found[at] = true
		return stageBackupFile(ctx, settings, staged, wanted.paths[at], entry, report)
	})
	if err != nil {
		return err
	}
	return ledgerMissingFiles(wanted, found, present, orphans, ledger)
}

// stageBackupFile stores one archive entry, or reuses what an earlier run already put on disk.
func stageBackupFile(
	ctx context.Context,
	settings Settings,
	staged *Staged,
	path string,
	entry backupEntry,
	report *StageReport,
) error {
	if _, already := staged.images[fileSource(path)]; already {
		report.Reused++
		return nil
	}
	body, err := readBounded(entry.Body, block.MaxPayloadBytes)
	if err != nil {
		return fmt.Errorf("read %s from the file backup: %w", path, err)
	}
	image, err := settings.Assets.StageImage(ctx, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("stage %s: %w", path, err)
	}
	return recordStaged(ctx, settings, staged, fileSource(path), image, report)
}

// ledgerMissingFiles records what the archive did not hold and what nothing in it owns.
func ledgerMissingFiles(
	wanted *wantedFiles,
	found, present []bool,
	orphans []string,
	ledger *Ledger,
) error {
	for at, held := range found {
		if !held {
			return fmt.Errorf(
				"the file backup holds no file at %s, which an image row names", wanted.paths[at],
			)
		}
	}
	for at, held := range present {
		if held {
			continue
		}
		assetID := wanted.references[at].AssetID
		if err := ledger.Raise(Exception{
			Kind: "missing_cover_file", Subject: wanted.references[at].Path,
			Detail:  "the row's own cover path has no file in the file backup",
			AssetID: &assetID,
		}); err != nil {
			return err
		}
	}
	slices.Sort(orphans)
	for _, name := range orphans {
		if err := ledger.Raise(Exception{
			Kind: "orphan_source_file", Subject: name,
			Detail: "the file backup holds bytes no v1 row names, so they have no owner to migrate to",
		}); err != nil {
			return err
		}
	}
	return nil
}

// fetchExternalImages is the one bounded exception to Illarin never fetching a creator-supplied URL, and a miss keeps the URL.
func fetchExternalImages(
	ctx context.Context,
	settings Settings,
	results []v1.Result,
	staged *Staged,
	report *StageReport,
	ledger *Ledger,
) error {
	for _, result := range results {
		for _, external := range result.ExternalMedia {
			if _, already := staged.images[external.URL]; already {
				report.Reused++
				continue
			}
			assetID := result.AssetID
			fetched, err := settings.Fetcher.Fetch(ctx, external.URL)
			if err == nil {
				image, stageErr := settings.Assets.StageImage(ctx, bytes.NewReader(fetched.Body))
				if stageErr == nil {
					report.Fetched++
					if err := recordStaged(ctx, settings, staged, external.URL, image, report); err != nil {
						return err
					}
					continue
				}
				err = stageErr
			}
			report.Failed++
			if err := ledger.Raise(Exception{
				Kind: "external_media_fetch_failed", Subject: external.URL,
				Detail:  fmt.Sprintf("the image was not brought onto Illarin: %s", err),
				AssetID: &assetID,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordStaged(
	ctx context.Context,
	settings Settings,
	staged *Staged,
	source string,
	image asset.StagedImage,
	report *StageReport,
) error {
	if err := db.New(settings.Target).RecordStagedMedia(ctx, db.RecordStagedMediaParams{
		Source: source,
		BlobID: pgtype.UUID{Bytes: image.BlobID, Valid: true},
		Width:  int32(image.Width), Height: int32(image.Height),
	}); err != nil {
		return fmt.Errorf("record %s as staged: %w", source, err)
	}
	staged.images[source] = image
	report.Stored++
	return nil
}

// fileSource distinguishes a v1 stored path from a URL in the staging table.
func fileSource(cleaned string) string { return "file:" + cleaned }

func ownedBackupFile(name string) bool {
	cleaned := cleanBackupPath(name)
	directory := path.Dir(cleaned)
	for _, owned := range ownedBackupDirectories {
		if directory == owned || strings.HasSuffix(directory, "/"+owned) {
			return true
		}
	}
	return false
}
