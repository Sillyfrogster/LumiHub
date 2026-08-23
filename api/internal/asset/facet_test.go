package asset

import (
	"context"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

type roleModule struct {
	claimsFirstPayload
	kind     string
	elements []block.Element
}

func (roleModule) ID() string { return "roles" }

func (m roleModule) Declaration() format.Declaration {
	declaration := testReaderDeclaration("roles", m.kind)
	declaration.Kind = m.kind
	return declaration
}

func (m roleModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return format.Parsed{Kind: m.kind, Format: "roles", Elements: m.elements}, nil
}

func textSet(role block.Role, texts ...string) block.Element {
	items := make([]block.TextItem, len(texts))
	for i, text := range texts {
		items[i] = block.TextItem{ID: uuid.New(), Text: text}
	}
	return block.Element{Type: block.TypeTextSet, Role: role, Content: block.TextSet{Texts: items}}
}

func entryTable(role block.Role, names ...string) block.Element {
	entries := make([]block.Entry, len(names))
	for i, name := range names {
		entries[i] = block.Entry{
			ID: uuid.New(), Name: name, Keys: []string{name},
			Text: name, Enabled: true,
		}
	}
	return block.Element{
		Type: block.TypeEntryTable, Role: role,
		Content: block.EntryTable{Entries: entries},
	}
}

func publishedWithElements(
	t *testing.T,
	kind string,
	elements []block.Element,
) (*Service, uuid.UUID) {
	t.Helper()
	floor := []block.Element{
		{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "Keeps the archive."}},
	}
	carriesGreetings := false
	for _, element := range elements {
		carriesGreetings = carriesGreetings || element.Role == block.RoleGreetings
	}
	if !carriesGreetings {
		floor = append(floor, textSet(block.RoleGreetings, "Welcome back."))
	}
	svc, _ := newTestServiceWithRegistry(t, registryWithModule(t, roleModule{
		kind: kind, elements: append(floor, elements...),
	}))
	ownerID := revisionOwner(t, svc, "facet.owner")
	created := ingestOne(t, svc, ownerID, "asset.json", []byte(`{"payload":true}`))
	publishImported(t, svc, ownerID, created)
	return svc, created.ID
}

func browseWith(t *testing.T, svc *Service, f ListFilter) BrowsePage {
	t.Helper()
	if f.Limit == 0 {
		f.Limit = 24
	}
	page, err := svc.Browse(context.Background(), f, ContentShown)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	return page
}

func TestAFacetFiltersOnWhatTheElementsHold(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash", "Bay"),
	})

	carried := browseWith(t, svc, ListFilter{
		Kind:   "character",
		Facets: []FacetSelection{{Key: string(block.FacetLorebook), Value: "true"}},
	})
	if carried.Total != 1 {
		t.Fatalf("assets with a lorebook = %d, want 1", carried.Total)
	}
	without := browseWith(t, svc, ListFilter{
		Kind:   "character",
		Facets: []FacetSelection{{Key: string(block.FacetLorebook), Value: "false"}},
	})
	if without.Total != 0 {
		t.Fatalf("assets with no lorebook = %d, want 0", without.Total)
	}
}

func TestACountFacetFiltersByItsDeclaredBuckets(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		textSet(block.RoleGreetings, "Hello", "Hi", "Hey"),
	})

	for _, test := range []struct {
		bucket string
		want   int
	}{{"1", 0}, {"2-4", 1}, {"5-up", 0}} {
		page := browseWith(t, svc, ListFilter{
			Kind: "character",
			Facets: []FacetSelection{
				{Key: string(block.FacetAlternateGreetings), Value: test.bucket},
			},
		})
		if page.Total != test.want {
			t.Errorf("bucket %q matched %d assets, want %d", test.bucket, page.Total, test.want)
		}
	}
}

func TestTwoBucketsOfOneFacetWidenTheResultRatherThanEmptyingIt(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		textSet(block.RoleGreetings, "Hello", "Hi", "Hey"),
	})

	page := browseWith(t, svc, ListFilter{
		Kind: "character",
		Facets: []FacetSelection{
			{Key: string(block.FacetAlternateGreetings), Value: "1"},
			{Key: string(block.FacetAlternateGreetings), Value: "2-4"},
		},
	})
	if page.Total != 1 {
		t.Fatalf("two buckets of one facet matched %d assets, want 1", page.Total)
	}
}

func TestAnAssetMustMatchEveryFacetAskedFor(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash"),
	})

	page := browseWith(t, svc, ListFilter{
		Kind: "character",
		Facets: []FacetSelection{
			{Key: string(block.FacetLorebook), Value: "true"},
			{Key: string(block.FacetExpressions), Value: "true"},
		},
	})
	if page.Total != 0 {
		t.Fatalf("matched %d assets, an asset must answer every facet asked for", page.Total)
	}
}

func TestAFacetNoKindDeclaresNarrowsNothing(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash"),
	})

	page := browseWith(t, svc, ListFilter{
		Kind:   "character",
		Facets: []FacetSelection{{Key: "spec", Value: "chara_card_v3"}},
	})
	if page.Total != 1 {
		t.Fatalf("an undeclared facet narrowed the catalog to %d", page.Total)
	}
}

func TestABucketNoFacetDeclaresNarrowsNothing(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash"),
	})

	page := browseWith(t, svc, ListFilter{
		Kind:   "character",
		Facets: []FacetSelection{{Key: string(block.FacetLorebook), Value: "maybe"}},
	})
	if page.Total != 1 {
		t.Fatalf("an undeclared bucket narrowed the catalog to %d", page.Total)
	}
}

func TestFacetsAreScopedToTheirKind(t *testing.T) {
	svc, _ := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash"),
	})

	mixed := browseWith(t, svc, ListFilter{})
	if len(mixed.Facets) != 0 {
		t.Fatalf("the mixed catalog offered %d facet groups, want none", len(mixed.Facets))
	}
	scoped := browseWith(t, svc, ListFilter{Kind: "character"})
	if len(scoped.Facets) != 4 {
		t.Fatalf("character offered %d facet groups, want 4", len(scoped.Facets))
	}
}

func TestTheProjectionStoresTheRawCount(t *testing.T) {
	svc, assetID := publishedWithElements(t, "character", []block.Element{
		entryTable(block.RoleLorebookEntries, "Ash", "Bay", "Cove"),
	})

	var stored map[string]int
	if err := svc.pool.QueryRow(context.Background(),
		`select facets from asset_projections where asset_id = $1`, assetID,
	).Scan(&stored); err != nil {
		t.Fatalf("read the facet projection: %v", err)
	}
	if stored[string(block.FacetLorebook)] != 3 {
		t.Fatalf("stored lorebook count = %d, want the raw 3", stored[string(block.FacetLorebook)])
	}
}
