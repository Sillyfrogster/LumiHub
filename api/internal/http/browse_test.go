package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type browseModule struct{}

func (browseModule) ID() string { return "browse_card" }
func (browseModule) Declaration() format.Declaration {
	return testReaderDeclaration("browse_card", "character")
}
func (browseModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (browseModule) Parse(_ context.Context, file probe.Inspection, _ format.Claim) (format.Parsed, error) {
	elements := []block.Element{
		{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "Test description"}},
		{Type: block.TypeTextSet, Role: block.RoleGreetings, Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Hello"}}}},
	}
	if entries, ok := file.Payloads[0].String("lorebook"); ok {
		elements = append(elements, block.Element{
			Type: block.TypeEntryTable, Role: block.RoleLorebookEntries,
			Content: block.EntryTable{Entries: []block.Entry{{
				ID: block.NewItemID(), Name: entries, Keys: []string{entries},
				Text: entries, Enabled: true,
			}}},
		})
	}
	return format.Parsed{Kind: "character", Format: "browse_card", Elements: elements}, nil
}

func TestBrowseReturnsOnlyCardContentAndTheReadersEffectiveCount(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Velvet Night")
	metadata["filename"] = "velvet-night.lumitheme"
	metadata["blurb"] = "A quiet midnight theme."
	metadata["tags"] = []string{"Midnight"}
	metadata["isNsfw"] = true
	finished := uploadAndFinish(t, router, session, assets, metadata, []byte("theme"))
	if finished.Code != http.StatusOK {
		t.Fatalf("finish ingest status = %d, want 200: %s", finished.Code, finished.Body.String())
	}
	assetID := assetIDFromIngest(t, finished)
	gallery := send(t, router, authorized(mediaUploadRequest(
		t, assetID, "gallery", httpTestPNG(t, 400, 300),
	), session))
	if gallery.Code != http.StatusCreated {
		t.Fatalf("add gallery status = %d, want 201: %s", gallery.Code, gallery.Body.String())
	}

	response := send(t, router, httptest.NewRequest(
		http.MethodGet, "/v1/assets?nsfw=blurred", nil,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("browse status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var body struct {
		Items      []map[string]any `json:"items"`
		Total      int              `json:"total"`
		Suppressed int              `json:"suppressed"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode browse response: %v", err)
	}
	if body.Total != 1 || body.Suppressed != 0 || len(body.Items) != 1 {
		t.Fatalf("browse counts = total %d, suppressed %d, items %d; want 1, 0, 1",
			body.Total, body.Suppressed, len(body.Items))
	}
	wantKeys := map[string]bool{
		"id": true, "name": true, "creator": true, "kind": true,
		"isNsfw": true, "cover": true,
	}
	for key := range body.Items[0] {
		if !wantKeys[key] {
			t.Errorf("browse card exposed %q; cards carry no catalog or format internals", key)
		}
	}
	if body.Items[0]["name"] != "Velvet Night" ||
		body.Items[0]["creator"] != "verified.creator" ||
		body.Items[0]["kind"] != "character" || body.Items[0]["isNsfw"] != true {
		t.Errorf("browse card = %#v", body.Items[0])
	}
	if cover, present := body.Items[0]["cover"]; !present || cover != nil {
		t.Errorf("coverless card cover = %#v, present %v; want an explicit null", cover, present)
	}
}

func TestBrowseSearchUsesCatalogWordsAndItsTwoQualifiers(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	entries := []struct {
		name        string
		blurb       string
		tags        []string
		sourceBytes string
	}{
		{
			name: "Moon Garden", blurb: "A place for patient stories.",
			tags: []string{"Botanical", "Original Character"}, sourceBytes: "buried-prompt-word",
		},
		{
			name: "Quiet Harbor", blurb: "Moonlit conversations by the water.",
			tags: []string{"Slow Burn", "Botanical"}, sourceBytes: "harbor",
		},
		{
			name: "Mood:gentle", blurb: "An unknown qualifier remains ordinary text.",
			tags: []string{"Comfort"}, sourceBytes: "gentle",
		},
	}
	for _, entry := range entries {
		metadata := exampleMetadata(entry.name)
		metadata["filename"] = entry.name + ".lumitheme"
		metadata["blurb"] = entry.blurb
		metadata["tags"] = entry.tags
		uploadAndFinish(t, router, session, assets, metadata, []byte(entry.sourceBytes))
	}

	cases := []struct {
		query string
		want  []string
	}{
		{query: "MOON", want: []string{"Quiet Harbor", "Moon Garden"}},
		{query: "verified.creator", want: []string{"Mood:gentle", "Quiet Harbor", "Moon Garden"}},
		{query: "botanical", want: nil},
		{query: "tag:botanical", want: []string{"Quiet Harbor", "Moon Garden"}},
		{query: `tag:"slow burn"`, want: []string{"Quiet Harbor"}},
		{query: `tag:botanical tag:"slow burn"`, want: []string{"Quiet Harbor"}},
		{query: "author:verified.creator patient", want: []string{"Moon Garden"}},
		{query: "author:verified.creator author:verified.creator patient", want: []string{"Moon Garden"}},
		{query: "mood:gentle", want: []string{"Mood:gentle"}},
		{query: "buried-prompt-word", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			response := send(t, router, httptest.NewRequest(
				http.MethodGet, "/v1/assets?q="+url.QueryEscape(tc.query), nil,
			))
			if response.Code != http.StatusOK {
				t.Fatalf("browse status = %d, want 200: %s", response.Code, response.Body.String())
			}
			var body struct {
				Items []struct {
					Name string `json:"name"`
				} `json:"items"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode browse response: %v", err)
			}
			got := make([]string, len(body.Items))
			for i, item := range body.Items {
				got[i] = item.Name
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("search %q returned %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

type browseOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
	Selected bool   `json:"selected"`
}

type browseFacetGroup struct {
	Key     string         `json:"key"`
	Label   string         `json:"label"`
	Options []browseOption `json:"options"`
}

type browseReading struct {
	Names     []string
	Platforms []browseOption
	Facets    []browseFacetGroup
}

func readBrowse(t *testing.T, router http.Handler, path string) browseReading {
	t.Helper()
	response := send(t, router, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("browse %s status = %d: %s", path, response.Code, response.Body.String())
	}
	var body struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
		Platforms []browseOption     `json:"platforms"`
		Facets    []browseFacetGroup `json:"facets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode browse response: %v", err)
	}
	reading := browseReading{Platforms: body.Platforms, Facets: body.Facets}
	for _, item := range body.Items {
		reading.Names = append(reading.Names, item.Name)
	}
	return reading
}

func facetGroup(t *testing.T, groups []browseFacetGroup, key string) browseFacetGroup {
	t.Helper()
	for _, group := range groups {
		if group.Key == key {
			return group
		}
	}
	t.Fatalf("no %s facet among %+v", key, groups)
	return browseFacetGroup{}
}

func TestFacetsAreKindScopedAndFilterOnElementContent(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(browseModule{}); err != nil {
		t.Fatalf("register browse module: %v", err)
	}
	router, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Aster")
	metadata["filename"] = "aster.json"
	uploadAndFinish(t, router, session, assets, metadata, []byte(`{"card":true,"lorebook":"Ash"}`))
	metadata = exampleMetadata("Storm")
	metadata["filename"] = "storm.json"
	uploadAndFinish(t, router, session, assets, metadata, []byte(`{"card":true}`))

	mixed := readBrowse(t, router, "/v1/assets")
	if len(mixed.Facets) != 0 {
		t.Fatalf("the mixed catalog offered %+v, want no facets", mixed.Facets)
	}

	scoped := readBrowse(t, router, "/v1/assets?kind=character")
	keys := make([]string, 0, len(scoped.Facets))
	for _, group := range scoped.Facets {
		keys = append(keys, group.Key)
	}
	want := []string{"lorebook", "alternate_greetings", "expressions", "gallery"}
	if !slices.Equal(keys, want) {
		t.Fatalf("character facets = %v, want %v", keys, want)
	}
	lorebook := facetGroup(t, scoped.Facets, "lorebook")
	if lorebook.Options[0].Count != 1 || lorebook.Options[1].Count != 1 {
		t.Errorf("lorebook counts = %+v, want one carrying and one not", lorebook.Options)
	}

	carried := readBrowse(t, router, "/v1/assets?kind=character&facet=lorebook%3Dtrue")
	if !slices.Equal(carried.Names, []string{"Aster"}) {
		t.Fatalf("assets with a lorebook = %v, want Aster", carried.Names)
	}
	none := readBrowse(t, router, "/v1/assets?kind=character&facet=lorebook%3Dfalse")
	if !slices.Equal(none.Names, []string{"Storm"}) {
		t.Fatalf("assets with no lorebook = %v, want Storm", none.Names)
	}
}

func TestArrangingThePageChangesNoFilterResult(t *testing.T) {
	r, session, assets, _ := newCharacterIngestRouterWithPool(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	givePictures(t, r, session, assetID, "gallery", "gallery")
	publishCharacter(t, r, session, assetID)

	before := readBrowse(t, r, "/v1/assets?kind=character&facet=gallery%3Dtrue")
	if !slices.Equal(before.Names, []string{"Ana"}) {
		t.Fatalf("a gallery answered %v, want Ana", before.Names)
	}

	page := fetchStartedAsset(t, r, session, assetID)
	gallery := blockNamed(t, page.Blocks, "gallery")
	messagesBlock := blockNamed(t, page.Blocks, "messages")

	renamed := editableBlock(gallery)
	title := "Concept art"
	renamed.Title = &title
	if response := saveBlock(t, r, session, assetID, gallery.ID, renamed); response.Code != http.StatusOK {
		t.Fatalf("rename the gallery: %d %s", response.Code, response.Body.String())
	}
	reordered := arrangeBlocks(t, r, session, assetID, []arrangedBlock{
		{ID: gallery.ID, Width: gallery.Width},
		{ID: messagesBlock.ID, Width: messagesBlock.Width},
		{ID: blockNamed(t, page.Blocks, "character_core").ID, Width: "full"},
	})
	if reordered.Code != http.StatusOK {
		t.Fatalf("reorder the page: %d %s", reordered.Code, reordered.Body.String())
	}

	messages := editableBlock(messagesBlock)
	messages.Layout = "stack-3"
	messages.Elements[0].Slot = "top"
	messages.Elements[1].Slot = "middle"
	if response := saveBlock(t, r, session, assetID, messagesBlock.ID, messages); response.Code != http.StatusOK {
		t.Fatalf("make room in Messages: %d %s", response.Code, response.Body.String())
	}
	move := httptest.NewRequest(
		http.MethodPost,
		"/v1/assets/"+assetID+"/blocks/"+gallery.ID+"/move-and-remove",
		strings.NewReader(`{"destinationBlockId":"`+messagesBlock.ID+`"}`),
	)
	move.Header.Set("Content-Type", "application/json")
	if moved := send(t, r, authorized(move, session)); moved.Code != http.StatusOK {
		t.Fatalf("move the gallery into Messages: %d %s", moved.Code, moved.Body.String())
	}

	after := readBrowse(t, r, "/v1/assets?kind=character&facet=gallery%3Dtrue")
	if !slices.Equal(after.Names, before.Names) {
		t.Fatalf("arranging the page changed the filter result to %v, want %v", after.Names, before.Names)
	}
}

func TestContentInsideACustomSectionAnswersNoFacet(t *testing.T) {
	router, session := newVerifiedTestRouter(t)
	started := startCharacter(t, router, session)
	writeCharacterFloor(t, router, session, started)
	custom := addedBlock(t, addBlock(t, router, session, started.ID, "custom_section", "text_set"))
	if response := publishAsset(t, router, session, started.ID); response.Code != http.StatusOK {
		t.Fatalf("publish status = %d: %s", response.Code, response.Body.String())
	}

	body := editableBlock(custom)
	body.Elements[0].Content = json.RawMessage(
		`{"texts":[{"text":"One"},{"text":"Two"},{"text":"Three"}]}`,
	)
	if response := saveBlock(t, router, session, started.ID, custom.ID, body); response.Code != http.StatusOK {
		t.Fatalf("save the custom section status = %d: %s", response.Code, response.Body.String())
	}

	for _, bucket := range []string{"1", "2-4", "5-up"} {
		found := readBrowse(t, router, "/v1/assets?kind=character&facet=alternate_greetings%3D"+bucket)
		if len(found.Names) != 0 {
			t.Fatalf("a heading the creator invented answered bucket %q: %v", bucket, found.Names)
		}
	}
}

func TestAnEmptySectionNeverAnswersAsCarried(t *testing.T) {
	r, session, assets, _ := newCharacterIngestRouterWithPool(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	addedBlock(t, addBlock(t, r, session, assetID, "expressions", "image_set"))
	publishCharacter(t, r, session, assetID)

	carried := readBrowse(t, r, "/v1/assets?kind=character&facet=expressions%3Dtrue")
	if len(carried.Names) != 0 {
		t.Fatalf("an empty expression set answered as carried: %v", carried.Names)
	}
	none := readBrowse(t, r, "/v1/assets?kind=character&facet=expressions%3Dfalse")
	if !slices.Equal(none.Names, []string{"Ana"}) {
		t.Fatalf("an empty expression set answered %v, want the none bucket", none.Names)
	}
}

func TestThePlatformControlNamesAppsAndMatchesThroughOfferedTargets(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	router, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.json"
	uploadAndFinish(t, router, session, assets, metadata, []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Keeps the archive.","first_mes":"Welcome back."}
	}`))

	all := readBrowse(t, router, "/v1/assets")
	labels := make([]string, 0, len(all.Platforms))
	for _, option := range all.Platforms {
		labels = append(labels, option.Label)
	}
	if !slices.Equal(labels, []string{"SillyTavern", "RisuAI", "Lumiverse"}) {
		t.Fatalf("the platform control offered %v, want the apps Illarin names", labels)
	}
	for _, option := range all.Platforms {
		if option.Count != 1 {
			t.Errorf("%s count = %d, want the card every named app can open", option.Label, option.Count)
		}
	}

	named := readBrowse(t, router, "/v1/assets?platform=sillytavern")
	if !slices.Equal(named.Names, []string{"Ana"}) {
		t.Fatalf("SillyTavern returned %v, want Ana", named.Names)
	}
	unknown := readBrowse(t, router, "/v1/assets?platform=notepad")
	if len(unknown.Names) != 0 {
		t.Fatalf("an app Illarin does not name returned %v", unknown.Names)
	}
}

func facetComputedAt(t *testing.T, pool *pgxpool.Pool, assetID string) time.Time {
	t.Helper()
	var computedAt time.Time
	if err := pool.QueryRow(context.Background(), `
		select facet_computed_at from asset_projections where asset_id = $1
	`, assetID).Scan(&computedAt); err != nil {
		t.Fatalf("read the facet projection: %v", err)
	}
	return computedAt
}

func TestAHiddenSectionAnswersNoFacetAndStillExports(t *testing.T) {
	r, session, assets, pool := newCharacterIngestRouterWithPool(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	giveExpressions(t, r, session, assetID)
	publishCharacter(t, r, session, assetID)

	shown := readBrowse(t, r, "/v1/assets?kind=character&facet=expressions%3Dtrue")
	if !slices.Equal(shown.Names, []string{"Ana"}) {
		t.Fatalf("a shown expression set answered %v, want Ana", shown.Names)
	}

	exportedAt := projectionComputedAt(t, pool, assetID)
	measuredAt := facetComputedAt(t, pool, assetID)
	generation := contentGeneration(t, pool, assetID)

	page := fetchStartedAsset(t, r, session, assetID)
	arrangement := make([]arrangedBlock, 0, len(page.Blocks))
	for _, holder := range page.Blocks {
		arrangement = append(arrangement, arrangedBlock{
			ID: holder.ID, Hidden: holder.Definition == "expressions", Width: holder.Width,
		})
	}
	if hidden := arrangeBlocks(t, r, session, assetID, arrangement); hidden.Code != http.StatusOK {
		t.Fatalf("hide the expressions: %d %s", hidden.Code, hidden.Body.String())
	}

	if after := projectionComputedAt(t, pool, assetID); !after.Equal(exportedAt) {
		t.Error("hiding a section moved the export section of the projection")
	}
	if after := facetComputedAt(t, pool, assetID); !after.After(measuredAt) {
		t.Error("hiding a section left the facet section of the projection alone")
	}
	if after := contentGeneration(t, pool, assetID); after != generation {
		t.Errorf("content generation = %d, want %d after a hide", after, generation)
	}

	carried := readBrowse(t, r, "/v1/assets?kind=character&facet=expressions%3Dtrue")
	if len(carried.Names) != 0 {
		t.Fatalf("a hidden expression set still answered the facet: %v", carried.Names)
	}
	none := readBrowse(t, r, "/v1/assets?kind=character&facet=expressions%3Dfalse")
	if !slices.Equal(none.Names, []string{"Ana"}) {
		t.Fatalf("a hidden expression set answered %v, want the none bucket", none.Names)
	}

	export, err := assets.OpenExport(
		context.Background(), uuid.MustParse(assetID), nil, "chara_card_v3",
	)
	if err != nil {
		t.Fatalf("export a card with a hidden section: %v", err)
	}
	if !containsBytes(export.Body, []byte("happy")) {
		t.Fatal("a hidden section's content did not travel in the download")
	}
}

func TestSignedInBrowseUsesTheReadersSavedContentPreference(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Veiled Garden")
	metadata["filename"] = "veiled-garden.lumitheme"
	metadata["isNsfw"] = true
	created := uploadAndFinish(t, router, session, assets, metadata, []byte("garden"))
	assetID := assetIDFromIngest(t, created)
	added := send(t, router, authorized(mediaUploadRequest(
		t, assetID, "avatar", httpTestPNG(t, 80, 120),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add cover status = %d, want 201: %s", added.Code, added.Body.String())
	}

	saved := send(t, router, authorizedJSONRequest(
		t, http.MethodPut, "/v1/account/nsfw-visibility", `{"visibility":"hidden"}`, session,
	))
	if saved.Code != http.StatusNoContent {
		t.Fatalf("save visibility status = %d, want 204: %s", saved.Code, saved.Body.String())
	}

	type response struct {
		Items []struct {
			Cover *struct {
				URL string `json:"url"`
			} `json:"cover"`
		} `json:"items"`
		Total      int     `json:"total"`
		Suppressed int     `json:"suppressed"`
		Visibility string  `json:"visibility"`
		EmptyState *string `json:"emptyState"`
	}
	read := func(request *http.Request) response {
		t.Helper()
		answer := send(t, router, request)
		if answer.Code != http.StatusOK {
			t.Fatalf("browse status = %d, want 200: %s", answer.Code, answer.Body.String())
		}
		var body response
		if err := json.Unmarshal(answer.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode browse response: %v", err)
		}
		return body
	}

	signedIn := read(authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets", nil), session,
	))
	if signedIn.Total != 0 || signedIn.Suppressed != 1 || signedIn.Visibility != "hidden" ||
		signedIn.EmptyState == nil || *signedIn.EmptyState != "suppressed" {
		t.Fatalf("signed-in browse = %#v, want the saved hidden preference", signedIn)
	}
	signedOut := read(httptest.NewRequest(http.MethodGet, "/v1/assets", nil))
	if signedOut.Total != 1 || signedOut.Visibility != "blurred" || len(signedOut.Items) != 1 ||
		signedOut.Items[0].Cover == nil || !strings.Contains(signedOut.Items[0].Cover.URL, "/grid_blurred/") {
		t.Fatalf("signed-out browse = %#v, want one blurred card", signedOut)
	}
}
