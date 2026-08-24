package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/google/uuid"
)

func TestASealedPromptLeavesOnlyThroughAnAllowedLinkedInstance(t *testing.T) {
	router, session, pool := newLinkingRouter(t)
	grant := linkDeviceInstance(t, router, session, "Lumiverse", "desk", []string{receiveScope})
	declareTargets(t, router, grant.AccessToken, []string{"preset_lumiverse"})

	started := startPreset(t, router, session, "lumiverse")
	core := editableBlock(blockNamed(t, started.Blocks, "preset_core"))
	const privateText = "Install this complete prompt."
	core.Elements[0].Content = json.RawMessage(`{"groups":[],"fragments":[{"name":"Private instructions","role":"system","text":"` + privateText + `","protected":true,"enabled":true}]}`)
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if got := saveBlock(t, router, session, started.ID, started.Blocks[0].ID, core); got.Code != http.StatusOK {
		t.Fatalf("save sealed prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}
	if got := saveIdentity(t, router, session, started.ID, `{"name":"Linked preset","isNsfw":false}`); got.Code != http.StatusNoContent {
		t.Fatalf("save identity status = %d, want 204: %s", got.Code, got.Body.String())
	}
	if got := publishAsset(t, router, session, started.ID); got.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", got.Code, got.Body.String())
	}
	unsupported := linkDeviceInstance(t, router, session, "Other app", "tablet", []string{receiveScope})
	declareTargets(t, router, unsupported.AccessToken, []string{"portable-card-v1"})
	for _, state := range assetInstances(t, router, session, started.ID).Items {
		if state.InstanceID == unsupported.Instance.ID && state.CanReceive {
			t.Fatal("an instance without an allowed target was offered sealed content")
		}
	}
	if got := sendToInstance(t, router, session, started.ID, unsupported.Instance.ID); got.Code != http.StatusConflict {
		t.Fatalf("queue incompatible instance = %d, want 409: %s", got.Code, got.Body.String())
	}

	ordinary := send(t, router, httptest.NewRequest(http.MethodGet, "/download/"+started.ID+"/preset_lumiverse", nil))
	if ordinary.Code != http.StatusNotFound || strings.Contains(ordinary.Body.String(), privateText) {
		t.Fatalf("ordinary export = %d %s", ordinary.Code, ordinary.Body.String())
	}
	store, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("create export store: %v", err)
	}
	linkedExport, err := asset.NewService(pool, testRegistry(t), store).DownloadExportForLinkedInstance(
		context.Background(), uuid.MustParse(started.ID), "preset_lumiverse",
	)
	if err != nil {
		t.Fatalf("write linked export: %v", err)
	}
	if !strings.Contains(string(linkedExport.Body), privateText) {
		t.Fatal("the linked export did not restore the protected prompt")
	}

	queued := sendToInstance(t, router, session, started.ID, grant.Instance.ID)
	if queued.Code != http.StatusAccepted {
		t.Fatalf("queue status = %d, want 202: %s", queued.Code, queued.Body.String())
	}
	work := decodeResponse[deliveryWorkList](t, collect(t, router, grant.AccessToken, nil)).Deliveries[0]
	if work.Format != "preset_lumiverse" {
		t.Fatalf("delivery format = %q, want Lumiverse", work.Format)
	}
	artifact := fetchSigned(t, router, work.Artifacts[0].URL)
	if artifact.Code != http.StatusOK {
		t.Fatalf("artifact status = %d, want 200: %s", artifact.Code, artifact.Body.String())
	}
	if artifact.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("artifact cache policy = %q", artifact.Header().Get("Cache-Control"))
	}
	if !strings.Contains(artifact.Body.String(), privateText) || strings.Contains(artifact.Body.String(), `"protected"`) {
		t.Fatalf("delivery did not contain one ordinary complete preset: %s", artifact.Body.String())
	}
}

func TestPublicPresetResponsesCarrySealedShapeWithoutProtectedText(t *testing.T) {
	router, session, _ := newLinkingRouter(t)
	started := startPreset(t, router, session, "lumiverse")
	core := editableBlock(blockNamed(t, started.Blocks, "preset_core"))
	groupID := uuid.NewString()
	const privateText = "disclosure-canary-7bb627e4"
	core.Elements[0].Content = json.RawMessage(`{
		"groups":[{"id":"` + groupID + `","name":"Reasoning"}],
		"fragments":[
			{"name":"Visible shape","role":"user","placement":"pre_history","text":"Public prompt.","enabled":true,"groupId":"` + groupID + `"},
			{"name":"Private shape","role":"assistant","placement":"post_history","text":"` + privateText + `","protected":true,"enabled":false,"groupId":"` + groupID + `"}
		]
	}`)
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if got := saveBlock(t, router, session, started.ID, started.Blocks[0].ID, core); got.Code != http.StatusOK {
		t.Fatalf("save sealed prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}
	if got := saveIdentity(t, router, session, started.ID, `{"name":"Reader-safe preset","isNsfw":false}`); got.Code != http.StatusNoContent {
		t.Fatalf("save identity status = %d, want 204: %s", got.Code, got.Body.String())
	}
	if got := publishAsset(t, router, session, started.ID); got.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", got.Code, got.Body.String())
	}

	reader := send(t, router, httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil))
	if reader.Code != http.StatusOK {
		t.Fatalf("reader page status = %d, want 200: %s", reader.Code, reader.Body.String())
	}
	publicBody := reader.Body.String()
	if strings.Contains(publicBody, privateText) {
		t.Fatal("the complete reader response contains protected text")
	}
	for _, visible := range []string{
		`"linkedInstallOnly":true`, `"allowedApps":["lumiverse"]`,
		`"name":"Private shape"`, `"role":"assistant"`,
		`"placement":"post_history"`, `"protected":true`,
		`"enabled":false`, `"groupId":"` + groupID + `"`, `"text":""`,
		`"facts":["2 fragments","1 switched on"]`, `"downloads":[]`,
	} {
		if !strings.Contains(publicBody, visible) {
			t.Errorf("reader response is missing %s: %s", visible, publicBody)
		}
	}

	strangerSession := signUp(t, router, "reader@example.com", "signed.reader")
	stranger := send(t, router, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/assets/"+started.ID, nil), strangerSession,
	))
	if stranger.Code != http.StatusOK || strings.Contains(stranger.Body.String(), privateText) {
		t.Fatalf("signed-in reader response = %d %s", stranger.Code, stranger.Body.String())
	}

	search := send(t, router, httptest.NewRequest(
		http.MethodGet, "/v1/assets?q="+privateText, nil,
	))
	if search.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200: %s", search.Code, search.Body.String())
	}
	var results struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(search.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(results.Items) != 0 || strings.Contains(search.Body.String(), privateText) {
		t.Fatalf("protected text matched or appeared in search: %s", search.Body.String())
	}
}

func TestProtectedAssetsRefuseEveryOrdinaryExportWithoutRecordingAHandoff(t *testing.T) {
	router, session, pool := newLinkingRouter(t)
	started := startPreset(t, router, session, "lumiverse")
	if len(started.Downloads) == 0 {
		t.Fatal("the ordinary preset has no generated export target to protect")
	}

	core := editableBlock(blockNamed(t, started.Blocks, "preset_core"))
	const privateText = "ordinary-export-canary-86fd7431"
	core.Elements[0].Content = json.RawMessage(`{"groups":[],"fragments":[{
		"name":"Private instructions","role":"system","text":"` + privateText + `","protected":true,"enabled":true
	}]}`)
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if got := saveBlock(t, router, session, started.ID, started.Blocks[0].ID, core); got.Code != http.StatusOK {
		t.Fatalf("save sealed prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}
	if got := saveIdentity(t, router, session, started.ID, `{"name":"No ordinary exports","isNsfw":false}`); got.Code != http.StatusNoContent {
		t.Fatalf("save identity status = %d, want 204: %s", got.Code, got.Body.String())
	}
	if got := publishAsset(t, router, session, started.ID); got.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", got.Code, got.Body.String())
	}

	for _, target := range started.Downloads {
		response := send(t, router, httptest.NewRequest(
			http.MethodGet, "/download/"+started.ID+"/"+target.Format, nil,
		))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s export status = %d, want 404", target.Format, response.Code)
		}
		if response.Body.String() != `{"error":"no such download"}` ||
			strings.Contains(response.Body.String(), privateText) {
			t.Errorf("%s export refusal = %s", target.Format, response.Body.String())
		}
		for _, header := range []string{
			"Content-Disposition", "X-Accel-Redirect", "X-Illarin-Export-Target",
		} {
			if value := response.Header().Get(header); value != "" {
				t.Errorf("%s export set %s to %q", target.Format, header, value)
			}
		}
	}

	var events int
	if err := pool.QueryRow(t.Context(),
		`select count(*) from download_events where asset_id = $1`, started.ID,
	).Scan(&events); err != nil {
		t.Fatalf("count refused download events: %v", err)
	}
	if events != 0 {
		t.Fatalf("refused ordinary exports recorded %d download events", events)
	}
}

func TestAProtectedOriginalUploadIsRecoveryAccessForItsOwnerAlone(t *testing.T) {
	router, ownerSession, assets, pool := newVerifiedIngestRouterWithPool(
		t, lumiverseIngestRegistry(t),
	)
	metadata := exampleMetadata("Protected original")
	metadata["filename"] = "protected-original.json"
	finished := uploadAndFinish(t, router, ownerSession, assets, metadata, []byte(keyedSealedPreset))
	assetID := assetIDFromIngest(t, finished)
	readerSession := signUp(t, router, "original-reader@example.com", "original.reader")

	for name, request := range map[string]*http.Request{
		"signed out": httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil),
		"non-owner": authorized(
			httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil), readerSession,
		),
	} {
		response := send(t, router, request)
		if response.Code != http.StatusNotFound || response.Header().Get("X-Accel-Redirect") != "" ||
			strings.Contains(response.Body.String(), "Exact private prompt.") {
			t.Errorf("%s source response = %d, headers %v, body %s",
				name, response.Code, response.Header(), response.Body.String())
		}
	}

	owner := send(t, router, authorized(
		httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil), ownerSession,
	))
	if owner.Code != http.StatusOK || owner.Header().Get("X-Accel-Redirect") == "" || owner.Body.Len() != 0 {
		t.Fatalf("owner recovery response = %d, headers %v, body %s",
			owner.Code, owner.Header(), owner.Body.String())
	}

	var events int
	var class string
	if err := pool.QueryRow(t.Context(), `
		select count(*), coalesce(max(authorization_class), '')
		  from download_events where asset_id = $1
	`, assetID).Scan(&events, &class); err != nil {
		t.Fatalf("read protected source events: %v", err)
	}
	if events != 1 || class != "owner" {
		t.Fatalf("protected source events = %d with class %q, want one owner recovery", events, class)
	}
}

func TestAReplacementUploadRemovesProtectedContentWithoutAnOwningPrompt(t *testing.T) {
	_, router, session, assets, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startPreset(t, router, session, "lumiverse")
	coreBlock := blockNamed(t, started.Blocks, "preset_core")
	core := editableBlock(coreBlock)
	core.Elements[0].Content = json.RawMessage(`{"groups":[],"fragments":[
		{"name":"Old sealed prompt","role":"system","text":"Private text with an old owner.","protected":true,"enabled":true}
	]}`)
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("save sealed prompt: %d %s", response.Code, response.Body.String())
	}

	replacement := []byte(`{
		"schemaVersion": 1,
		"name": "Replacement preset",
		"blocks": [
			{"id":"new-public-prompt","name":"New public prompt","role":"system","content":"Public replacement text.","enabled":true}
		]
	}`)
	accepted := send(t, router, authorized(
		revisionRequest(t, started.ID, "replacement.json", replacement), session,
	))
	if accepted.Code != http.StatusAccepted {
		t.Fatalf("replacement upload status = %d, want 202: %s", accepted.Code, accepted.Body.String())
	}
	if processed, err := assets.ProcessNextIngest(t.Context()); err != nil || !processed {
		t.Fatalf("process replacement = %t, %v; want true, nil", processed, err)
	}
	updated := pollIngestAsset(t, router, session, accepted.Header().Get("Location"))
	if updated.ID != started.ID {
		t.Fatalf("replacement asset = %s, want %s", updated.ID, started.ID)
	}
	if payloads, policies := protectedCounts(t, pool, started.ID); payloads != 0 || policies != 0 {
		t.Fatalf("after replacement: %d payloads and %d policy rows, want none", payloads, policies)
	}
	owner := fetchStartedAsset(t, router, session, started.ID)
	if owner.LinkedInstallOnly || len(owner.AllowedApps) != 0 {
		t.Fatalf("replacement kept protected delivery policy: linked install only %t, apps %v",
			owner.LinkedInstallOnly, owner.AllowedApps)
	}
}
