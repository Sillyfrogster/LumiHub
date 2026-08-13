package format_test

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

type corpusStore struct{ data []byte }

func (s corpusStore) ReadRange(_ context.Context, _ uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data[offset : offset+length])), nil
}

type corpusModule struct{ id string }

func (m corpusModule) ID() string { return m.id }
func (m corpusModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if m.id == character.V2 {
		return character.CCv2(file)
	}
	return character.CCv3(file)
}
func (m corpusModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Format: m.id}, nil
}

func TestLocalCorpusHasNoConflictingAuthoritativeClaims(t *testing.T) {
	root := filepath.Clean("../../../.ai/probe-corpus")
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("local probe corpus is not present")
	} else if err != nil {
		t.Fatal("read local probe corpus")
	}

	registry := format.NewRegistry()
	for _, id := range []string{character.V2, character.V3} {
		if err := registry.Register(corpusModule{id: id}); err != nil {
			t.Fatal("register corpus module")
		}
	}

	checked := 0
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
		file, err := probe.Inspect(context.Background(), corpusStore{data: data}, uuid.New(), int64(len(data)), "fixture.bin")
		if err != nil {
			return err
		}
		if _, _, err := registry.Resolve(file); err != nil {
			return err
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatal("local probe corpus failed")
	}
	if checked == 0 {
		t.Fatal("local probe corpus is empty")
	}
}
