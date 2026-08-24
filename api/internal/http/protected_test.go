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
