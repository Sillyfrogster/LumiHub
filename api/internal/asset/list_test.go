package asset

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
)

func create(t *testing.T, svc *Service, name, kind, publication string) Asset {
	t.Helper()
	a, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: kind, Filename: name + ".bin",
		File: bytes.NewReader([]byte(name)), Name: name, Publication: publication,
	})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return a
}

func TestListReturnsOnlyPublicAssets(t *testing.T) {
	svc, _ := newTestService(t)
	create(t, svc, "visible", "character", "public")
	create(t, svc, "quiet", "character", "unlisted")

	got, err := svc.List(context.Background(), ListFilter{Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Name != "visible" {
		t.Fatalf("List returned %d assets, want only the public one", len(got))
	}
}

func TestListFiltersByKind(t *testing.T) {
	svc, _ := newTestService(t)
	create(t, svc, "a-character", "character", "public")
	create(t, svc, "a-theme", "theme", "public")

	got, err := svc.List(context.Background(), ListFilter{Kind: "theme", Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Name != "a-theme" {
		t.Fatalf("kind filter returned %d assets, want just the theme", len(got))
	}
}
