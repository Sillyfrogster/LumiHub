package migration

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAV1AddressIsTheAuthorAndTheName(t *testing.T) {
	candidate := legacyCandidate{
		AssetID: uuid.New(), Author: "Rosa Hendricks", Handle: "rosa.h",
		Name: "Liz and Eric — Torn Between Two",
	}
	if got := candidate.address(); got != "rosa-hendricks/liz-and-eric-torn-between-two" {
		t.Errorf("address = %q", got)
	}
}

func TestAnAssetWithNoCreditedAuthorIsAddressedByItsOwner(t *testing.T) {
	candidate := legacyCandidate{AssetID: uuid.New(), Handle: "rosa.h", Name: "Vellum Faewild"}
	if got := candidate.address(); got != "rosa-h/vellum-faewild" {
		t.Errorf("address = %q", got)
	}
}

func TestANameThatSlugifiesToNothingHasNoAddress(t *testing.T) {
	candidate := legacyCandidate{AssetID: uuid.New(), Handle: "rosa.h", Name: "≽^•⩊•^≼"}
	if got := candidate.address(); got != "" {
		t.Errorf("address = %q, want none", got)
	}
}

func TestTheOlderAssetKeepsAContestedAddress(t *testing.T) {
	older := legacyCandidate{
		AssetID: uuid.New(), Author: "Rosa", Name: "Liz and Eric",
		CreatedAt: time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC),
	}
	newer := legacyCandidate{
		AssetID: uuid.New(), Author: "Rosa", Name: "Liz and Eric",
		CreatedAt: time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
	}
	paths, displaced := resolveLegacyPaths([]legacyCandidate{newer, older})
	if len(paths) != 1 || paths[0].AssetID != older.AssetID {
		t.Fatalf("the address went to %d assets and not to the older one", len(paths))
	}
	if len(displaced) != 1 || displaced[0].AssetID != newer.AssetID {
		t.Fatalf("the newer asset was not left for the ledger")
	}
}

// The legacy record has one writer, and the reason it exists is that somebody eventually reads a column named downloads and assumes it is live.
func TestNoRuntimePathReadsTheLegacyCounters(t *testing.T) {
	frozen := []string{"migration_legacy_counters", "v1_downloads", "v1_views"}
	root := repositoryFile(t, "api")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		holder := filepath.ToSlash(filepath.Dir(relative))
		if holder == "internal/migration" || holder == "internal/db" || holder == "internal/testdb" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, name := range frozen {
			if strings.Contains(string(source), name) {
				t.Errorf("%s names %s, which stopped moving at the cutover", relative, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read the api sources: %v", err)
	}
}
