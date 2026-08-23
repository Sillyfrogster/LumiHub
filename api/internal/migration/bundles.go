package migration

import (
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
)

const bundleExtension = ".lumitheme"

// attachThemeBundles claims each theme's bundle by stored id then by a creation time only it shares, and a theme left without one is fatal because its fonts exist nowhere else.
func attachThemeBundles(themes []v1.Row, backup *FileBackup) ([]string, error) {
	wanted := newBackupIndex()
	byCreation := make(map[int64][]int, len(themes))
	for at, source := range themes {
		row := source.(v1.ThemeRow)
		if row.AssetBundleID == "" {
			return nil, fmt.Errorf("v1 theme %s names no bundle", row.Common.ID)
		}
		wanted.wantStem(row.AssetBundleID, at)
		created := row.Common.CreatedAt.Unix()
		byCreation[created] = append(byCreation[created], at)
	}

	matched := make([]bool, len(themes))
	claimed := make([]string, 0, len(themes))
	claimedSizes := make(map[int64]struct{}, len(themes))
	unclaimed := make(map[int64]struct{})
	err := backup.each(func(entry backupEntry) error {
		isBundle := strings.EqualFold(path.Ext(entry.Name), bundleExtension)
		at, exact, found := bundleOwner(wanted, byCreation, entry, isBundle)
		if !found {
			if isBundle {
				unclaimed[entry.Size] = struct{}{}
			}
			return nil
		}
		if matched[at] {
			if exact {
				return fmt.Errorf("a v1 theme bundle appears more than once in the file backup")
			}
			unclaimed[entry.Size] = struct{}{}
			return nil
		}
		body, err := readBounded(entry.Body, block.MaxPayloadBytes)
		if err != nil {
			return fmt.Errorf("read a v1 theme bundle: %w", err)
		}
		row := themes[at].(v1.ThemeRow)
		row.Bundle = body
		themes[at] = row
		matched[at] = true
		claimed = append(claimed, entry.Name)
		claimedSizes[entry.Size] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	remaining, err := claimRemainingBundle(themes, backup, matched, claimedSizes, unclaimed)
	if err != nil {
		return nil, err
	}
	if remaining != "" {
		claimed = append(claimed, remaining)
	}
	return claimed, nil
}

// claimRemainingBundle pairs the last theme with the last unclaimed bundle, which is a pairing rather than a guess.
func claimRemainingBundle(
	themes []v1.Row,
	backup *FileBackup,
	matched []bool,
	claimedSizes map[int64]struct{},
	unclaimed map[int64]struct{},
) (string, error) {
	remaining := make([]int, 0, len(themes))
	for at, found := range matched {
		if !found {
			remaining = append(remaining, at)
		}
	}
	if len(remaining) == 0 {
		return "", nil
	}
	distinct := make(map[int64]struct{}, len(unclaimed))
	for size := range unclaimed {
		if _, claimed := claimedSizes[size]; !claimed {
			distinct[size] = struct{}{}
		}
	}
	if len(remaining) != 1 || len(distinct) != 1 {
		return "", fmt.Errorf(
			"%d v1 themes have no bundle in the file backup and %d unclaimed bundles remain",
			len(remaining), len(distinct),
		)
	}
	var body []byte
	var name string
	err := backup.each(func(entry backupEntry) error {
		if !strings.EqualFold(path.Ext(entry.Name), bundleExtension) {
			return nil
		}
		if _, wanted := distinct[entry.Size]; !wanted || body != nil {
			return nil
		}
		read, err := readBounded(entry.Body, block.MaxPayloadBytes)
		if err != nil {
			return fmt.Errorf("read the remaining v1 theme bundle: %w", err)
		}
		body, name = read, entry.Name
		return nil
	})
	if err != nil {
		return "", err
	}
	if body == nil {
		return "", fmt.Errorf("the remaining v1 theme bundle is absent from the file backup")
	}
	row := themes[remaining[0]].(v1.ThemeRow)
	row.Bundle = body
	themes[remaining[0]] = row
	return name, nil
}

func bundleOwner(
	wanted *backupIndex,
	byCreation map[int64][]int,
	entry backupEntry,
	isBundle bool,
) (int, bool, bool) {
	if at, found := wanted.find(entry.Name); found {
		return at, true, true
	}
	candidates := byCreation[entry.ModTime.Unix()]
	if isBundle && len(candidates) == 1 {
		return candidates[0], false, true
	}
	return 0, false, false
}

func readBounded(reader io.Reader, limit int) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > limit {
		return nil, fmt.Errorf("the file is larger than the %d byte format limit", limit)
	}
	return body, nil
}
