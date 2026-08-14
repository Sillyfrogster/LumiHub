package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

type assetPageResponse struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Blurb     string `json:"blurb"`
	Creator   string `json:"creator"`
	IsNSFW    bool   `json:"isNsfw"`
	Discovery string `json:"discovery"`
	CreatedAt string `json:"createdAt"`
	Tags      []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	} `json:"tags"`
	Media []struct {
		ID        string `json:"id"`
		Role      string `json:"role"`
		DetailURL string `json:"detailUrl"`
		ThumbURL  string `json:"thumbUrl"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"media"`
	Preview    *string `json:"preview"`
	Visibility string  `json:"visibility"`
	Withhold   *struct {
		Reason string    `json:"reason"`
		Actor  string    `json:"actor"`
		At     time.Time `json:"at"`
	} `json:"withhold"`
}

func fetchAssetPage(t *testing.T, r http.Handler, path string) assetPageResponse {
	t.Helper()
	response := send(t, r, httptest.NewRequest(http.MethodGet, path, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200: %s", path, response.Code, response.Body.String())
	}
	var page assetPageResponse
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode asset page: %v", err)
	}
	return page
}

func TestAssetPageCarriesItsMediaTagsAndBlurb(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("The Quiet Archivist")
	metadata["filename"] = "archivist.lumitheme"
	metadata["blurb"] = "She closes the book on a ribbon."
	metadata["tags"] = []string{"Slow Burn", " Modern "}
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, []byte("theme")))

	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "avatar", httpTestPNG(t, 800, 1000),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add media status = %d, want 201: %s", added.Code, added.Body.String())
	}

	page := fetchAssetPage(t, r, "/v1/assets/"+assetID)

	if page.ID != assetID || page.Name != "The Quiet Archivist" || page.Kind != "theme" {
		t.Fatalf("asset page identity = %+v", page)
	}
	if page.Blurb != "She closes the book on a ribbon." {
		t.Errorf("blurb = %q, want the catalog blurb", page.Blurb)
	}
	if page.Creator != "verified.creator" {
		t.Errorf("creator = %q, want the owner's handle", page.Creator)
	}
	want := []struct{ label, value string }{{"Slow Burn", "slow burn"}, {" Modern ", "modern"}}
	if len(page.Tags) != len(want) {
		t.Fatalf("tags = %+v, want %d", page.Tags, len(want))
	}
	for i, tag := range want {
		if page.Tags[i].Label != tag.label || page.Tags[i].Value != tag.value {
			t.Errorf("tag %d = %+v, want %q shown and %q matched", i, page.Tags[i], tag.label, tag.value)
		}
	}
	if len(page.Media) != 1 {
		t.Fatalf("media = %+v, want the one image", page.Media)
	}
	image := page.Media[0]
	if image.Role != "avatar" || image.Width != 800 || image.Height != 1000 {
		t.Errorf("cover = %+v", image)
	}
	if image.DetailURL != "/media/"+image.ID+"/detail/1" {
		t.Errorf("detailUrl = %q", image.DetailURL)
	}
	if image.ThumbURL != "/media/"+image.ID+"/thumb/1" {
		t.Errorf("thumbUrl = %q", image.ThumbURL)
	}
	if page.Preview == nil || *page.Preview != "/media/"+image.ID+"/og/1" {
		t.Errorf("preview = %v, want the composed social preview", page.Preview)
	}
}

func TestAssetPageShowsNoTotals(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Countless")
	metadata["filename"] = "countless.lumitheme"
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, []byte("theme")))

	response := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil))
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode asset page: %v", err)
	}

	wantKeys := map[string]bool{
		"id": true, "kind": true, "name": true, "blurb": true, "tags": true,
		"creator": true, "isNsfw": true, "discovery": true, "createdAt": true,
		"media": true, "preview": true, "visibility": true,
	}
	for key := range body {
		if !wantKeys[key] {
			t.Errorf("asset page carries %q, which is not part of the page", key)
		}
	}
}

func TestAssetPageAnswersNormallyForAnUnlistedAsset(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("Kept Back")
	metadata["filename"] = "kept-back.lumitheme"
	metadata["discovery"] = "unlisted"
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, []byte("theme")))

	page := fetchAssetPage(t, r, "/v1/assets/"+assetID)

	if page.Discovery != "unlisted" {
		t.Fatalf("discovery = %q, want unlisted", page.Discovery)
	}
}

func TestWithheldDeletedAndNeverExistedAssetsAnswerAlike(t *testing.T) {
	r, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	withhold := assetIDFromIngest(t, uploadAndFinish(
		t, r, session, assets, withFilename(exampleMetadata("Withheld"), "withheld"), []byte("a"),
	))
	deleted := assetIDFromIngest(t, uploadAndFinish(
		t, r, session, assets, withFilename(exampleMetadata("Deleted"), "deleted"), []byte("b"),
	))
	staff := "11111111-1111-1111-1111-111111111111"
	if _, err := pool.Exec(context.Background(), `
		insert into users (id, username) values ($1, 'staff')
	`, staff); err != nil {
		t.Fatalf("seed staff account: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		update assets
		   set withheld_at = now(), withheld_by = $2, withheld_reason = 'testing'
		 where id = $1
	`, withhold, staff); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`update assets set deleted_at = now(), recoverable_until = now() + interval '30 days' where id = $1`, deleted,
	); err != nil {
		t.Fatalf("delete asset: %v", err)
	}

	never := "22222222-2222-2222-2222-222222222222"
	var bodies []string
	for _, id := range []string{withhold, deleted, never} {
		response := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/assets/"+id, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET /v1/assets/%s status = %d, want 404: %s",
				id, response.Code, response.Body.String())
		}
		bodies = append(bodies, response.Body.String())
	}
	if bodies[0] != bodies[1] || bodies[1] != bodies[2] {
		t.Fatalf("the three refusals differ: %q", bodies)
	}
}

func TestBlurredReaderIsNeverHandedAClearVariant(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("After Dark")
	metadata["filename"] = "after-dark.lumitheme"
	metadata["isNsfw"] = true
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, []byte("theme")))
	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "gallery", httpTestPNG(t, 600, 600),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add media status = %d, want 201: %s", added.Code, added.Body.String())
	}

	for _, preference := range []string{"blurred", "hidden"} {
		page := fetchAssetPage(t, r, "/v1/assets/"+assetID+"?nsfw="+preference)
		if len(page.Media) != 1 {
			t.Fatalf("%s media = %+v", preference, page.Media)
		}
		encoded, err := json.Marshal(page)
		if err != nil {
			t.Fatalf("re-encode asset page: %v", err)
		}
		for _, clear := range []string{"/detail/", "/thumb/", "/og/"} {
			if strings.Contains(string(encoded), clear) {
				t.Errorf("a %s reader was handed a %s variant: %s", preference, clear, encoded)
			}
		}
		if page.Preview == nil || !strings.Contains(*page.Preview, "/og_blurred/") {
			t.Errorf("%s preview = %v, want the blurred social preview", preference, page.Preview)
		}
	}

	shown := fetchAssetPage(t, r, "/v1/assets/"+assetID+"?nsfw=shown")
	if !strings.HasSuffix(shown.Media[0].DetailURL, "/detail/1") {
		t.Errorf("a shown reader got %q", shown.Media[0].DetailURL)
	}
	if shown.Preview == nil || !strings.Contains(*shown.Preview, "/og_blurred/") {
		t.Errorf("preview = %v; a link preview has no reader to ask, so it stays blurred", shown.Preview)
	}
}

func withFilename(metadata map[string]any, name string) map[string]any {
	metadata["filename"] = name + ".lumitheme"
	return metadata
}
