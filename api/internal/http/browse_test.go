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

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

type browseModule struct{}

func (browseModule) ID() string { return "browse_card" }
func (browseModule) Claim(file probe.Inspection) (format.Claim, bool) {
	if len(file.Payloads) == 0 {
		return format.Claim{}, false
	}
	return format.CompatibilityClaim(file.Payloads[0]), true
}
func (browseModule) Parse(_ context.Context, file probe.Inspection, _ format.Claim) (format.Parsed, error) {
	tone := "gentle"
	if declared, ok := file.Payloads[0].String("tone"); ok {
		tone = declared
	}
	return format.Parsed{
		Kind: "character", Format: "browse_card",
		Facets: []format.Facet{{Key: "tone", Value: tone}, {Key: "client_feature", Value: "lorebook"}},
	}, nil
}
func (browseModule) BrowseDefinition() format.BrowseDefinition {
	return format.BrowseDefinition{
		Kind: "character",
		ExportTargets: []format.BrowseOption{
			{Value: "sillytavern", Label: "SillyTavern"},
		},
		Facets: []format.BrowseFacet{
			{
				Key: "tone", Label: "Tone",
				Options: []format.BrowseOption{
					{Value: "gentle", Label: "Gentle"},
					{Value: "dramatic", Label: "Dramatic"},
				},
			},
			{
				Key: "client_feature", Label: "SillyTavern features", Platforms: []string{"sillytavern"},
				Options: []format.BrowseOption{{Value: "lorebook", Label: "Embedded lorebook"}},
			},
		},
	}
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
		body.Items[0]["kind"] != "theme" || body.Items[0]["isNsfw"] != true {
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

func TestBrowseFacetsComeFromModulesAndAppearOnlyInTheirScope(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(browseModule{}); err != nil {
		t.Fatalf("register browse module: %v", err)
	}
	router, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Aster")
	metadata["filename"] = "aster.json"
	uploadAndFinish(t, router, session, assets, metadata, []byte(`{"card":true}`))
	metadata = exampleMetadata("Storm")
	metadata["filename"] = "storm.json"
	uploadAndFinish(t, router, session, assets, metadata, []byte(`{"tone":"dramatic"}`))

	type option struct {
		Value    string `json:"value"`
		Count    int    `json:"count"`
		Selected bool   `json:"selected"`
	}
	type facet struct {
		Key     string   `json:"key"`
		Options []option `json:"options"`
	}
	read := func(path string) (platforms []option, facets []facet, names []string) {
		t.Helper()
		response := send(t, router, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("browse %s status = %d: %s", path, response.Code, response.Body.String())
		}
		var body struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Platforms []option `json:"platforms"`
			Facets    []facet  `json:"facets"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode browse response: %v", err)
		}
		for _, item := range body.Items {
			names = append(names, item.Name)
		}
		return body.Platforms, body.Facets, names
	}

	platforms, facets, _ := read("/v1/assets")
	if len(platforms) < 2 || len(facets) != 0 {
		t.Fatalf("all catalog platforms = %#v, facets = %#v; want raw and declared platforms, no facets", platforms, facets)
	}
	_, facets, _ = read("/v1/assets?kind=character")
	if len(facets) != 1 || facets[0].Key != "tone" || len(facets[0].Options) != 2 {
		t.Fatalf("character facets = %#v, want only the two-option tone vocabulary", facets)
	}
	if facets[0].Options[0].Count != 1 || facets[0].Options[1].Count != 1 {
		t.Errorf("tone counts = %#v, want one gentle and one dramatic", facets[0].Options)
	}
	_, facets, names := read("/v1/assets?kind=character&platform=sillytavern&facet=tone=gentle")
	if len(facets) != 2 || !facets[0].Options[0].Selected || !slices.Equal(names, []string{"Aster"}) {
		t.Fatalf("scoped facets = %#v, names = %v", facets, names)
	}
	if facets[0].Options[1].Count != 0 {
		t.Fatalf("dramatic count after selecting gentle = %d, want 0 for conjunctive selection",
			facets[0].Options[1].Count)
	}
	_, _, names = read("/v1/assets?kind=character&platform=lumiverse")
	if len(names) != 0 {
		t.Fatalf("unsupported platform returned %v", names)
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
		t, assetID, "gallery", httpTestPNG(t, 80, 120),
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
