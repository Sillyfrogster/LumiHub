package block

import (
	"testing"

	"github.com/google/uuid"
)

func greetings(texts ...string) Element {
	items := make([]TextItem, len(texts))
	for i, text := range texts {
		items[i] = TextItem{ID: uuid.New(), Text: text}
	}
	return Element{
		ID: uuid.New(), Type: TypeTextSet, Role: RoleGreetings,
		Content: TextSet{Texts: items},
	}
}

func images(role Role, count int) Element {
	items := make([]ImageItem, count)
	for i := range items {
		items[i] = ImageItem{ID: uuid.New(), MediaID: uuid.New()}
	}
	return Element{
		ID: uuid.New(), Type: TypeImageSet, Role: role,
		Content: ImageSet{Images: items},
	}
}

func TestSevenFacetsShipAcrossFiveKinds(t *testing.T) {
	counts := map[string]int{
		"character": 4, "preset": 2, "pack": 1, "lorebook": 0, "theme": 0,
	}
	total := 0
	for kind, want := range counts {
		got := len(Facets(kind))
		if got != want {
			t.Errorf("%s declares %d facets, want %d", kind, got, want)
		}
		total += got
	}
	if total != 7 {
		t.Fatalf("%d facets ship, want 7", total)
	}
}

func TestEveryFacetIsKeyedToARoleAndCarriesBuckets(t *testing.T) {
	for _, kind := range Kinds() {
		for _, facet := range Facets(kind) {
			if !facet.Role.Known() {
				t.Errorf("%s facet %s names no known role", kind, facet.Key)
			}
			if facet.Measure == nil {
				t.Errorf("%s facet %s has no measure", kind, facet.Key)
			}
			if len(facet.Buckets) == 0 {
				t.Errorf("%s facet %s offers no buckets", kind, facet.Key)
			}
		}
	}
}

func TestAlternateGreetingsCountsEveryGreetingAfterTheFirst(t *testing.T) {
	cases := []struct {
		name  string
		texts []string
		want  int
	}{
		{"none at all", nil, 0},
		{"one greeting is not an alternate", []string{"Hello"}, 0},
		{"a second greeting is one alternate", []string{"Hello", "Hi"}, 1},
		{"empty texts do not count", []string{"Hello", "", "Hi"}, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := MeasureFacets("character", []Element{greetings(test.texts...)})
			if got[FacetAlternateGreetings] != test.want {
				t.Fatalf("got %d alternate greetings, want %d",
					got[FacetAlternateGreetings], test.want)
			}
		})
	}
}

func TestAFacetSumsEveryElementCarryingItsRole(t *testing.T) {
	counts := MeasureFacets("character", []Element{
		images(RoleGallery, 3), images(RoleGallery, 2),
	})
	if counts[FacetGallery] != 5 {
		t.Fatalf("got %d gallery images, want 5 across two elements", counts[FacetGallery])
	}
}

func TestContentWithNoRoleAnswersNoFacet(t *testing.T) {
	loose := images("", 4)
	counts := MeasureFacets("character", []Element{loose})
	for key, count := range counts {
		if count != 0 {
			t.Fatalf("facet %s counted %d from content with no role", key, count)
		}
	}
}

func TestAFacetOnlyReadsElementsOfItsOwnKind(t *testing.T) {
	counts := MeasureFacets("pack", []Element{images(RoleGallery, 3)})
	if _, offered := counts[FacetGallery]; offered {
		t.Fatal("a pack answered the character gallery facet")
	}
}

func TestBucketsCoverTheirRange(t *testing.T) {
	facet, ok := FacetByKey("pack", string(FacetItems))
	if !ok {
		t.Fatal("pack declares no item count facet")
	}
	for _, test := range []struct {
		count int
		want  string
	}{{1, "1"}, {2, "2-9"}, {9, "2-9"}, {10, "10-up"}, {25, "10-up"}} {
		held := ""
		for _, bucket := range facet.Buckets {
			if bucket.Holds(test.count) {
				held = bucket.Value
			}
		}
		if held != test.want {
			t.Errorf("%d items landed in %q, want %q", test.count, held, test.want)
		}
	}
}

func TestNoBucketHoldsACountBelowZeroOrOutsideTheCatalog(t *testing.T) {
	facet, _ := FacetByKey("character", string(FacetLorebook))
	included, _ := facet.Bucket("true")
	none, _ := facet.Bucket("false")
	if included.Holds(0) || !included.Holds(1) {
		t.Error("the included bucket must hold one entry and not none")
	}
	if !none.Holds(0) || none.Holds(1) {
		t.Error("the none bucket must hold zero entries and nothing else")
	}
}

func TestTheFacetStampMovesWhenTheCatalogDoes(t *testing.T) {
	stamp := FacetStamp()
	if stamp == "" {
		t.Fatal("the facet catalog has no stamp")
	}
	if stamp != FacetStamp() {
		t.Fatal("the facet stamp is not stable across reads")
	}
	restore := packFacets
	t.Cleanup(func() { facetCatalogs["pack"] = restore })
	facetCatalogs["pack"] = []Facet{{
		Key: FacetItems, Label: "Items", Role: RolePackItems, Measure: countRecords,
		Buckets: []Bucket{{Value: "1", Label: "1 item", Min: 1, Max: 4}},
	}}
	if FacetStamp() == stamp {
		t.Fatal("re-cutting a bucket boundary left the stamp unchanged")
	}
}

func TestNoFacetNamesAFormatOrAModuleSlot(t *testing.T) {
	for _, kind := range Kinds() {
		for _, facet := range Facets(kind) {
			if _, isSlot := slotFacetRoles[facet.Role]; isSlot {
				t.Errorf("%s facet %s reads a role whose content is module slots",
					kind, facet.Key)
			}
		}
	}
}

var slotFacetRoles = map[Role]struct{}{
	RoleSamplerSettings:    {},
	RoleCompletionSettings: {},
	RoleAdvancedSettings:   {},
	RoleThemeControls:      {},
}
