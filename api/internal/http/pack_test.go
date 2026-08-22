package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	packformat "github.com/Sillyfrogster/LumiHub/api/internal/format/pack"
)

func startPack(t *testing.T, r http.Handler, session *http.Cookie) startedAsset {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/assets",
		strings.NewReader(`{"kind":"pack"}`))
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusCreated {
		t.Fatalf("start a Pack: status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var started startedAsset
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode the started Pack: %v", err)
	}
	return started
}

func TestAPackBuiltFromNothingHasOneRequiredRecordListAndPublishesWithAnItem(t *testing.T) {
	r, session := newVerifiedTestRouter(t)
	started := startPack(t, r, session)

	if started.Kind != "pack" || len(started.Blocks) != 1 {
		t.Fatalf("started Pack = %+v, want one Pack block", started)
	}
	core := blockNamed(t, started.Blocks, "pack_core")
	if !core.Required || core.Hideable || core.Layout != "single" || core.Width != "full" {
		t.Errorf("Pack core = %+v, want a fixed full-width single block", core)
	}
	if len(core.Elements) != 1 || core.Elements[0].Type != "record_list" ||
		core.Elements[0].Role != "pack_items" || !core.Elements[0].Pinned {
		t.Fatalf("Pack core elements = %+v", core.Elements)
	}

	refused := publishAsset(t, r, session, started.ID)
	if refused.Code != http.StatusConflict {
		t.Fatalf("publish an empty Pack = %d, want 409: %s", refused.Code, refused.Body.String())
	}
	var refusal publishRefusal
	if err := json.Unmarshal(refused.Body.Bytes(), &refusal); err != nil {
		t.Fatalf("decode Pack readiness: %v", err)
	}
	if itemNamed(t, refusal.Readiness, "items").Met {
		t.Error("an empty Pack meets the item floor")
	}

	body := editableBlock(core)
	body.Elements[0].Content = json.RawMessage(`{
		"schema":"lumia",
		"records":[{
			"lumiaName":"Archivist","lumiaDefinition":"Keeps an archive.",
			"lumiaPersonality":"Patient.","lumiaBehavior":"Answers carefully.",
			"genderIdentity":2,"authorName":"A creator","version":1
		}]
	}`)
	if saved := saveBlock(t, r, session, started.ID, core.ID, body); saved.Code != http.StatusOK {
		t.Fatalf("save a Pack item = %d, want 200: %s", saved.Code, saved.Body.String())
	}
	if saved := saveIdentity(t, r, session, started.ID,
		`{"name":"Archive companions","isNsfw":false}`); saved.Code != http.StatusNoContent {
		t.Fatalf("save Pack identity = %d, want 204: %s", saved.Code, saved.Body.String())
	}
	if published := publishAsset(t, r, session, started.ID); published.Code != http.StatusOK {
		t.Fatalf("publish a ready Pack = %d, want 200: %s", published.Code, published.Body.String())
	}
}

func TestPackUploadBuildsAPageAndExportsEditedItemImages(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range packformat.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	r, session, assets, pool := newVerifiedIngestRouterWithPool(t, registry)
	metadata := exampleMetadata("Archive companions")
	metadata["filename"] = "companions.json"
	metadata["_keepDraft"] = true
	source := []byte(`{
		"packName":"Archive companions","packAuthor":"A creator","version":2,
		"coverUrl":"https://images.example/pack.png","futureTop":{"kept":true},
		"lumiaItems":[{
			"lumiaName":"Archivist","lumiaDefinition":"Keeps an archive.",
			"lumiaPersonality":"Patient.","lumiaBehavior":"Answers carefully.",
			"avatarUrl":"https://images.example/item.png","genderIdentity":2,
			"authorName":"A creator","version":3,"futureItem":{"kept":true}
		}],"loomItems":[]
	}`)
	assetID := assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, source))
	page := fetchStartedAsset(t, r, session, assetID)
	if page.Kind != "pack" || page.Lifecycle != "draft" || len(page.Blocks) != 1 ||
		len(page.Media) != 0 {
		t.Fatalf("imported Pack page = %+v", page)
	}
	core := blockNamed(t, page.Blocks, "pack_core")
	if core.Elements[0].Facts[0] != "1 item" {
		t.Errorf("Pack facts = %v, want one computed item", core.Elements[0].Facts)
	}

	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "pack_item", httpTestPNG(t, 96, 96),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add Pack item image = %d, want 201: %s", added.Code, added.Body.String())
	}
	var itemImage struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(added.Body.Bytes(), &itemImage); err != nil {
		t.Fatalf("decode Pack item image: %v", err)
	}
	if got := contentGeneration(t, pool, assetID); got != 1 {
		t.Fatalf("unreferenced Pack image moved content generation to %d", got)
	}

	body := editableBlock(core)
	var content struct {
		Schema  string                   `json:"schema"`
		Records []map[string]interface{} `json:"records"`
	}
	if err := json.Unmarshal(body.Elements[0].Content, &content); err != nil {
		t.Fatalf("decode Pack records: %v", err)
	}
	content.Records[0]["lumiaBehavior"] = "Answers with source notes."
	content.Records[0]["avatarUrl"] = itemImage.ID
	body.Elements[0].Content = mustJSON(t, content)
	if saved := saveBlock(t, r, session, assetID, core.ID, body); saved.Code != http.StatusOK {
		t.Fatalf("save edited Pack item = %d, want 200: %s", saved.Code, saved.Body.String())
	}
	if got := contentGeneration(t, pool, assetID); got != 2 {
		t.Fatalf("Pack item edit moved content generation to %d, want 2", got)
	}

	cover := send(t, r, authorized(mediaUploadRequest(
		t, assetID, "avatar", httpTestPNG(t, 800, 1000),
	), session))
	if cover.Code != http.StatusCreated {
		t.Fatalf("add Pack cover = %d, want 201: %s", cover.Code, cover.Body.String())
	}
	var coverImage struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cover.Body.Bytes(), &coverImage); err != nil {
		t.Fatalf("decode Pack cover: %v", err)
	}
	if got := contentGeneration(t, pool, assetID); got != 3 {
		t.Fatalf("Pack cover moved content generation to %d, want 3", got)
	}

	request := httptest.NewRequest(
		http.MethodGet, "/download/"+assetID+"/"+packformat.ID, nil,
	)
	download := send(t, r, authorized(request, session))
	if download.Code != http.StatusOK {
		t.Fatalf("download edited Pack = %d, want 200: %s", download.Code, download.Body.String())
	}
	var document struct {
		CoverURL string `json:"coverUrl"`
		Future   struct {
			Kept bool `json:"kept"`
		} `json:"futureTop"`
		Items []struct {
			Behavior  string `json:"lumiaBehavior"`
			AvatarURL string `json:"avatarUrl"`
			Future    struct {
				Kept bool `json:"kept"`
			} `json:"futureItem"`
		} `json:"lumiaItems"`
	}
	if err := json.Unmarshal(download.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode downloaded Pack: %v", err)
	}
	if len(document.Items) != 1 {
		t.Fatalf("downloaded Pack items = %+v, want one", document.Items)
	}
	coverPath, coverQuery, coverSigned := strings.Cut(document.CoverURL, "?")
	itemPath, itemQuery, itemSigned := strings.Cut(document.Items[0].AvatarURL, "?")
	if coverPath != "/media/"+coverImage.ID+"/detail/1" || !coverSigned ||
		!strings.Contains(coverQuery, "signature=") ||
		itemPath != "/media/"+itemImage.ID+"/detail/1" || !itemSigned ||
		!strings.Contains(itemQuery, "signature=") ||
		document.Items[0].Behavior != "Answers with source notes." ||
		!document.Future.Kept || !document.Items[0].Future.Kept {
		t.Fatalf("downloaded Pack lost edits, media URLs, or preserved fields: %+v", document)
	}
	if served := send(t, r, httptest.NewRequest(http.MethodGet, document.Items[0].AvatarURL, nil)); served.Code != http.StatusOK {
		t.Fatalf("downloaded Pack item image status = %d, want 200: %s", served.Code, served.Body.String())
	}
}
