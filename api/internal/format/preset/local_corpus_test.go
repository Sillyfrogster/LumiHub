package preset

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
)

// TestEveryLocalPresetSurvivesADownload exercises optional local fixtures and
// skips when the corpus is absent.
func TestEveryLocalPresetSurvivesADownload(t *testing.T) {
	root := filepath.Clean("../../../../.ai/probe-corpus/full")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		t.Skip("the local corpus is not on this machine")
	}
	if err != nil {
		t.Fatal(err)
	}

	read := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "preset-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		read++
		t.Run(entry.Name(), func(t *testing.T) {
			file := document(t, string(data))
			resolution, claimed, err := testRegistry(t).Resolve(file)
			if err != nil || !claimed {
				t.Fatalf("resolve: claimed=%v err=%v", claimed, err)
			}
			parsed, err := resolution.Module.Parse(context.Background(), file, resolution.Claim)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := block.ValidateContentLimits(parsed.Elements); err != nil {
				t.Fatalf("content limits: %v", err)
			}
			if _, err := block.Place(parsed.Kind, parsed.Elements); err != nil {
				t.Fatalf("place: %v", err)
			}

			written := write(t, resolution.Module, parsed)
			assertPreservedComesBack(t, parsed.Remainder, written.Body)
			again := parse(t, string(written.Body))
			if again.Format != parsed.Format {
				t.Fatalf("the written file reads as %q, want %q", again.Format, parsed.Format)
			}
			before, after := stripIDs(parsed.Elements), stripIDs(again.Elements)
			if len(before) != len(after) {
				t.Fatalf("read %d elements and then %d", len(before), len(after))
			}
			for i := range before {
				want, _ := json.Marshal(before[i].Content)
				got, _ := json.Marshal(after[i].Content)
				if string(want) != string(got) {
					t.Errorf("%s changed on the way through a download", before[i].Role)
				}
			}
		})
	}
	if read == 0 {
		t.Skip("the local corpus holds no presets")
	}
}
