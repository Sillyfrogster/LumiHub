package asset

import (
	"bytes"
	"context"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
)

/** A module that always emits the facets it was built with */
type facetModule struct {
	claimsFirstPayload
	facets []format.Facet
}

func (facetModule) ID() string { return "facets" }
func (facetModule) Declaration() format.Declaration {
	return testReaderDeclaration("facets", "character")
}
func (m facetModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: "character", Format: "facets", Facets: m.facets}, nil
}

func TestListMatchesEveryRequestedFacet(t *testing.T) {
	pool := testdb.Connect(t)
	blob, _ := storage.NewStore(pool, t.TempDir())

	reg := registryWithModule(t, facetModule{facets: []format.Facet{
		{Key: "pack_type", Value: "lumia"},
		{Key: "has_expressions", Value: "true"},
	}})
	svc := NewService(pool, reg, blob)

	if _, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "preset", Filename: "p.json",
		File: bytes.NewReader([]byte("{}")), Name: "Pack", Discovery: "listed",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	matching, err := svc.List(context.Background(), ListFilter{
		Limit:  50,
		Facets: []format.Facet{{Key: "pack_type", Value: "lumia"}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(matching) != 1 {
		t.Fatalf("got %d assets for a matching facet, want 1", len(matching))
	}

	missing, err := svc.List(context.Background(), ListFilter{
		Limit:  50,
		Facets: []format.Facet{{Key: "pack_type", Value: "loom"}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("got %d assets for a facet nothing has, want 0", len(missing))
	}
}

func TestListRequiresAllFacetsNotAny(t *testing.T) {
	pool := testdb.Connect(t)
	blob, _ := storage.NewStore(pool, t.TempDir())

	reg := registryWithModule(t, facetModule{facets: []format.Facet{
		{Key: "pack_type", Value: "lumia"},
	}})
	svc := NewService(pool, reg, blob)

	if _, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "preset", Filename: "p.json",
		File: bytes.NewReader([]byte("{}")), Name: "Pack", Discovery: "listed",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.List(context.Background(), ListFilter{
		Limit: 50,
		Facets: []format.Facet{
			{Key: "pack_type", Value: "lumia"},
			{Key: "has_expressions", Value: "true"},
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d assets, an asset must match every facet asked for", len(got))
	}
}

func TestFacetKeysAndValuesContainingEqualsDoNotCollide(t *testing.T) {
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	reg := registryWithModule(t, facetModule{facets: []format.Facet{
		{Key: "a=b", Value: "c"},
	}})
	svc := NewService(pool, reg, blob)

	if _, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "character", Filename: "p.json",
		File: bytes.NewReader([]byte("{}")), Name: "Pack", Discovery: "listed",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// A different facet that would encode to the same string if key and
	// value were joined with an equals sign.
	got, err := svc.List(context.Background(), ListFilter{
		Limit:  50,
		Facets: []format.Facet{{Key: "a", Value: "b=c"}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d assets, a different key and value must not match", len(got))
	}

	exact, err := svc.List(context.Background(), ListFilter{
		Limit:  50,
		Facets: []format.Facet{{Key: "a=b", Value: "c"}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(exact) != 1 {
		t.Fatalf("got %d assets for the exact facet, want 1", len(exact))
	}
}

func TestRequestingTheSameFacetTwiceStillMatches(t *testing.T) {
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	reg := registryWithModule(t, facetModule{facets: []format.Facet{
		{Key: "pack_type", Value: "lumia"},
	}})
	svc := NewService(pool, reg, blob)

	if _, err := svc.Create(context.Background(), CreateInput{
		OwnerID: uuid.New(), Kind: "preset", Filename: "p.json",
		File: bytes.NewReader([]byte("{}")), Name: "Pack", Discovery: "listed",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := svc.List(context.Background(), ListFilter{
		Limit: 50,
		Facets: []format.Facet{
			{Key: "pack_type", Value: "lumia"},
			{Key: "pack_type", Value: "lumia"},
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d assets, asking for one facet twice must not exclude it", len(got))
	}
}
