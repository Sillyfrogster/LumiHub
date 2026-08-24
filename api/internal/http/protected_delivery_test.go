package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ordinaryPreset = `{
	"schemaVersion": 1,
	"name": "Ordinary preset",
	"blocks": [
		{"id":"public","name":"Public","role":"system","content":"Visible prompt.","enabled":true}
	]
}`

// sealEveryFragment marks the saved prompt fragments protected, keeping their ids.
func sealEveryFragment(t *testing.T, body saveBlockBody, apps []string) saveBlockBody {
	t.Helper()
	var list struct {
		Groups    []json.RawMessage            `json:"groups"`
		Fragments []map[string]json.RawMessage `json:"fragments"`
	}
	if err := json.Unmarshal(body.Elements[0].Content, &list); err != nil {
		t.Fatalf("read the prompt list to seal: %v", err)
	}
	for index := range list.Fragments {
		list.Fragments[index]["protected"] = json.RawMessage("true")
	}
	sealed, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("write the sealed prompt list: %v", err)
	}
	body.Elements[0].Content = sealed
	body.AllowedApps = &apps
	return body
}

// publishSealedPreset builds one linked-install-only preset from nothing.
func publishSealedPreset(
	t *testing.T,
	router *gin.Engine,
	session *http.Cookie,
	name string,
	privateText string,
) string {
	t.Helper()
	started := startPreset(t, router, session, "lumiverse")
	core := editableBlock(blockNamed(t, started.Blocks, "preset_core"))
	core.Elements[0].Content = json.RawMessage(
		`{"groups":[],"fragments":[{"name":"Private instructions","role":"system","text":"` +
			privateText + `","protected":true,"enabled":true}]}`)
	core.AllowedApps = &[]string{"lumiverse"}
	if got := saveBlock(t, router, session, started.ID, started.Blocks[0].ID, core); got.Code != http.StatusOK {
		t.Fatalf("save sealed prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}
	if got := saveIdentity(
		t, router, session, started.ID, `{"name":"`+name+`","isNsfw":false}`,
	); got.Code != http.StatusNoContent {
		t.Fatalf("save identity status = %d, want 204: %s", got.Code, got.Body.String())
	}
	if got := publishAsset(t, router, session, started.ID); got.Code != http.StatusOK {
		t.Fatalf("publish status = %d, want 200: %s", got.Code, got.Body.String())
	}
	return started.ID
}

func settledDelivery(t *testing.T, pool *pgxpool.Pool, assetID string) (string, string) {
	t.Helper()
	var state, reason string
	if err := pool.QueryRow(context.Background(),
		`select state, coalesce(settled_reason, '') from instance_deliveries where asset_id = $1`,
		assetID,
	).Scan(&state, &reason); err != nil {
		t.Fatalf("read the delivery: %v", err)
	}
	return state, reason
}

func TestSealingAPromptStopsAQueuedDeliveryTheAppCanNoLongerReceive(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, testRegistry(t))
	metadata := exampleMetadata("Ordinary preset")
	metadata["filename"] = "ordinary.json"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte(ordinaryPreset)),
	)
	grant := linkDeviceInstance(t, router, session, "Paper Lantern", "desk", []string{receiveScope})
	declareTargets(t, router, grant.AccessToken, []string{"invented_by_the_client"})
	if queued := sendToInstance(t, router, session, assetID, grant.Instance.ID); queued.Code != http.StatusAccepted {
		t.Fatalf("queue status = %d, want 202: %s", queued.Code, queued.Body.String())
	}

	page := fetchStartedAsset(t, router, session, assetID)
	core := blockNamed(t, page.Blocks, "preset_core")
	sealed := sealEveryFragment(t, editableBlock(core), []string{"lumiverse"})
	if got := saveBlock(t, router, session, assetID, core.ID, sealed); got.Code != http.StatusOK {
		t.Fatalf("seal the prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}

	rec := collect(t, router, grant.AccessToken, nil)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("collect status = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	state, reason := settledDelivery(t, pool, assetID)
	if state != "failed" || reason != "unsupported" {
		t.Fatalf("delivery = %s/%s, want failed/unsupported", state, reason)
	}
	if got := downloadEventCount(t, pool, "linked_instance"); got != 0 {
		t.Fatalf("a stopped delivery recorded %d downloads, want 0", got)
	}
}

func TestAnArtifactAddressSignedBeforeSealingHandsOverNoBytesAfterwards(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, testRegistry(t))
	metadata := exampleMetadata("Ordinary preset")
	metadata["filename"] = "ordinary.json"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte(ordinaryPreset)),
	)
	grant := linkDeviceInstance(t, router, session, "Paper Lantern", "desk", []string{receiveScope})
	declareTargets(t, router, grant.AccessToken, []string{"invented_by_the_client"})
	sendToInstance(t, router, session, assetID, grant.Instance.ID)
	work := decodeResponse[deliveryWorkList](t, collect(t, router, grant.AccessToken, nil)).Deliveries[0]

	page := fetchStartedAsset(t, router, session, assetID)
	core := blockNamed(t, page.Blocks, "preset_core")
	sealed := sealEveryFragment(t, editableBlock(core), []string{"lumiverse"})
	if got := saveBlock(t, router, session, assetID, core.ID, sealed); got.Code != http.StatusOK {
		t.Fatalf("seal the prompt status = %d, want 200: %s", got.Code, got.Body.String())
	}

	fetched := fetchSigned(t, router, work.Artifacts[0].URL)

	if fetched.Code != http.StatusNotFound {
		t.Fatalf("fetch after sealing = %d, want 404: %s", fetched.Code, fetched.Body.String())
	}
	if fetched.Header().Get("X-Accel-Redirect") != "" {
		t.Fatalf("a refused artifact still pointed at %q", fetched.Header().Get("X-Accel-Redirect"))
	}
	if got := downloadEventCount(t, pool, "linked_instance"); got != 0 {
		t.Fatalf("a refused artifact recorded %d downloads, want 0", got)
	}
}

func TestAnInstancesApplicationNameGrantsNoProtectedDelivery(t *testing.T) {
	router, session, _ := newLinkingRouter(t)
	assetID := publishSealedPreset(t, router, session, "Named app preset", "Sealed for allowed apps only.")
	borrowedName := linkDeviceInstance(t, router, session, "Lumiverse", "desk", []string{receiveScope})
	declareTargets(t, router, borrowedName.AccessToken, []string{"invented_by_the_client"})
	otherName := linkDeviceInstance(t, router, session, "Some Other App", "tablet", []string{receiveScope})
	declareTargets(t, router, otherName.AccessToken, []string{"preset_lumiverse"})

	offered := map[string]bool{}
	for _, state := range assetInstances(t, router, session, assetID).Items {
		offered[state.InstanceID] = state.CanReceive
	}

	if offered[borrowedName.Instance.ID] {
		t.Fatal("an instance calling itself Lumiverse was offered a sealed preset")
	}
	if !offered[otherName.Instance.ID] {
		t.Fatal("an instance accepting the allowed target was not offered a sealed preset")
	}
	if got := sendToInstance(t, router, session, assetID, borrowedName.Instance.ID); got.Code != http.StatusConflict {
		t.Fatalf("queue by borrowed name = %d, want 409: %s", got.Code, got.Body.String())
	}
	if got := sendToInstance(t, router, session, assetID, otherName.Instance.ID); got.Code != http.StatusAccepted {
		t.Fatalf("queue by accepted target = %d, want 202: %s", got.Code, got.Body.String())
	}
	work := decodeResponse[deliveryWorkList](t, collect(t, router, otherName.AccessToken, nil)).Deliveries[0]
	if work.Format != "preset_lumiverse" {
		t.Fatalf("released format = %q, want preset_lumiverse", work.Format)
	}
}

func TestAnyReadersAllowedInstanceReceivesTheCompleteProtectedPreset(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, testRegistry(t))
	metadata := exampleMetadata("Keyed sealed preset")
	metadata["filename"] = "keyed.json"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte(keyedSealedPreset)),
	)
	reader := addVerifiedLinkingUser(t, router, pool, "reader@example.com", "reader.creator")
	grant := linkDeviceInstance(t, router, reader, "Lumiverse", "reader desk", []string{receiveScope})
	declareTargets(t, router, grant.AccessToken, []string{"preset_lumiverse"})

	if queued := sendToInstance(t, router, reader, assetID, grant.Instance.ID); queued.Code != http.StatusAccepted {
		t.Fatalf("a reader could not queue the preset: %d %s", queued.Code, queued.Body.String())
	}
	if before := downloadEventCount(t, pool, "linked_instance"); before != 0 {
		t.Fatalf("queueing recorded %d downloads, want 0", before)
	}
	work := decodeResponse[deliveryWorkList](t, collect(t, router, grant.AccessToken, nil)).Deliveries[0]
	artifact := fetchSigned(t, router, work.Artifacts[0].URL)

	if artifact.Code != http.StatusOK {
		t.Fatalf("artifact status = %d, want 200: %s", artifact.Code, artifact.Body.String())
	}
	if !strings.Contains(artifact.Body.String(), "Exact private prompt.") {
		t.Fatal("a reader's allowed instance did not receive the complete preset")
	}
	if got := downloadEventCount(t, pool, "linked_instance"); got != 1 {
		t.Fatalf("the handoff recorded %d downloads, want 1", got)
	}
	if got := collect(t, router, grant.AccessToken, []string{work.ID}); got.Code != http.StatusNoContent {
		t.Fatalf("acknowledge status = %d, want 204: %s", got.Code, got.Body.String())
	}
}
