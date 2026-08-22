package format_test

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/lorebook"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/preset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/theme"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

type corpusStore struct{ data []byte }

func (s corpusStore) ReadRange(_ context.Context, _ uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data[offset : offset+length])), nil
}

// TestLocalCorpusRunsThroughEveryModule catches overlapping authoritative claims.
func TestLocalCorpusRunsThroughEveryModule(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range slices.Concat(character.Modules(), lorebook.Modules(), preset.Modules(), theme.Modules()) {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	if err := registry.ValidateDeclarations(); err != nil {
		t.Fatalf("module declarations: %v", err)
	}

	root := os.Getenv("ILLARIN_LOCAL_CORPUS")
	if root == "" {
		t.Skip("local probe corpus is not configured")
	}
	root = filepath.Clean(root)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("local probe corpus is not present")
	} else if err != nil {
		t.Fatal("read local probe corpus")
	}

	checked, claimed := 0, 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := probe.Inspect(
			context.Background(), corpusStore{data: data}, uuid.New(), int64(len(data)), "fixture.bin",
		)
		if err != nil {
			return err
		}
		resolution, resolved, err := registry.Resolve(file)
		if err != nil {
			return err
		}
		checked++
		if !resolved {
			return nil
		}
		claimed++
		parsed, err := resolution.Module.Parse(context.Background(), file, resolution.Claim)
		if err != nil {
			return err
		}
		declared := resolution.Module.Declaration()
		if parsed.Kind != declared.Kind || parsed.Format != resolution.Module.ID() {
			t.Errorf("%s parsed as kind %q format %q", entry.Name(), parsed.Kind, parsed.Format)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("local probe corpus failed: %v", err)
	}
	if checked == 0 {
		t.Fatal("local probe corpus is empty")
	}
	if claimed == 0 {
		t.Fatal("no fixture in the local probe corpus resolved to a module")
	}
}
