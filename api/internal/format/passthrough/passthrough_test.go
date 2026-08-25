package passthrough

import (
	"context"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

func TestNeverClaimsToDetectAnything(t *testing.T) {
	if New().Detect("anything.bin", []byte{0x00}) {
		t.Fatal("passthrough must never win detection, it is only the fallback")
	}
}

func TestParseReturnsNoCatalogMetadata(t *testing.T) {
	got, err := New().Parse(context.Background(), strings.NewReader("some bytes"))
	if err != nil {
		t.Fatalf("Parse returned an error: %v", err)
	}
	if got.Format != "unknown" {
		t.Errorf("Format = %q, want unknown", got.Format)
	}
	if got.Name != "" || got.Description != "" || len(got.Tags) != 0 {
		t.Error("passthrough must not invent catalog metadata, the uploader supplies it")
	}
	if len(got.Facets) != 0 {
		t.Error("passthrough cannot read the file, so it has no facets to emit")
	}
}

func TestIsNotEditable(t *testing.T) {
	if _, editable := New().(format.Editor); editable {
		t.Fatal("passthrough must not implement Editor, it cannot edit safely")
	}
}
