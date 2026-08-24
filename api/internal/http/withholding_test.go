package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
)

func TestOnlyAnAdminCanWithholdAnAssetAndTheDecisionIsRecordedTogether(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, "")

	request := func() *http.Request {
		return authorizedJSONRequest(
			t,
			http.MethodPut,
			"/v1/assets/"+assetID+"/withhold",
			`{"reason":"Copyright report under review"}`,
			session,
		)
	}

	refused := send(t, router, request())
	if refused.Code != http.StatusForbidden {
		t.Fatalf("creator withhold status = %d, want 403: %s", refused.Code, refused.Body.String())
	}
	if _, err := pool.Exec(context.Background(), `
		update users set role = 'moderator' where username = 'verified.creator'
	`); err != nil {
		t.Fatalf("make test account a moderator: %v", err)
	}
	refused = send(t, router, request())
	if refused.Code != http.StatusForbidden {
		t.Fatalf("moderator withhold status = %d, want 403: %s", refused.Code, refused.Body.String())
	}

	var adminID uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		update users set role = 'admin' where username = 'verified.creator' returning id
	`).Scan(&adminID); err != nil {
		t.Fatalf("make test account an admin: %v", err)
	}

	withheld := send(t, router, request())
	if withheld.Code != http.StatusNoContent {
		t.Fatalf("admin withhold status = %d, want 204: %s", withheld.Code, withheld.Body.String())
	}

	var actor uuid.UUID
	var reason string
	var at time.Time
	if err := pool.QueryRow(context.Background(), `
		select withheld_by, withheld_reason, withheld_at from assets where id = $1
	`, assetID).Scan(&actor, &reason, &at); err != nil {
		t.Fatalf("read withhold decision: %v", err)
	}
	if actor != adminID || reason != "Copyright report under review" || at.IsZero() {
		t.Fatalf("withhold = actor %s, reason %q, at %v", actor, reason, at)
	}

	clear := httptest.NewRequest(http.MethodDelete, "/v1/assets/"+assetID+"/withhold", nil)
	clear.AddCookie(session)
	cleared := send(t, router, clear)
	if cleared.Code != http.StatusNoContent {
		t.Fatalf("admin clear status = %d, want 204: %s", cleared.Code, cleared.Body.String())
	}
	var decisionCleared bool
	if err := pool.QueryRow(context.Background(), `
		select withheld_by is null and withheld_reason is null and withheld_at is null
		  from assets where id = $1
	`, assetID).Scan(&decisionCleared); err != nil {
		t.Fatalf("read cleared withhold: %v", err)
	}
	if !decisionCleared {
		t.Fatal("clearing did not remove the complete withhold decision")
	}
}

func TestOwnerCanViewAndDownloadAWithheldAssetWithItsDecision(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, "")
	mediaID := addWithholdingTestMedia(t, router, session, assetID)

	if _, err := pool.Exec(context.Background(), `
		update assets asset
		   set withheld_at = '2026-08-14 12:30:00+00',
		       withheld_by = owner.id,
		       withheld_reason = 'Copyright report under review'
		  from users owner
		 where asset.id = $1 and owner.username = 'verified.creator'
	`, assetID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}

	pageRequest, err := http.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil)
	if err != nil {
		t.Fatalf("make owner page request: %v", err)
	}
	pageRequest.AddCookie(session)
	pageResponse := send(t, router, pageRequest)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("owner page status = %d, want 200: %s", pageResponse.Code, pageResponse.Body.String())
	}
	var page assetPageResponse
	if err := json.Unmarshal(pageResponse.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode owner page: %v", err)
	}
	if page.Withhold == nil || page.Withhold.Reason != "Copyright report under review" ||
		page.Withhold.Actor != "verified.creator" || page.Withhold.At.IsZero() {
		t.Fatalf("withhold shown to owner = %+v", page.Withhold)
	}

	downloadRequest, err := http.NewRequest(http.MethodGet, "/download/"+assetID, nil)
	if err != nil {
		t.Fatalf("make owner download request: %v", err)
	}
	downloadRequest.AddCookie(session)
	download := send(t, router, downloadRequest)
	if download.Code != http.StatusOK {
		t.Fatalf("owner download status = %d, want 200: %s", download.Code, download.Body.String())
	}

	for _, path := range []string{
		"/v1/assets/" + assetID + "/media",
		"/media/" + mediaID + "/grid/1",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(session)
		response := send(t, router, request)
		if response.Code != http.StatusOK {
			t.Fatalf("owner GET %s status = %d, want 200: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestUnavailableAssetsAnswerTheSameAcrossEveryPublicRead(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	withheldID := uploadDiscoveryTestAsset(t, router, session, assets, "")
	deletedID := uploadDiscoveryTestAsset(t, router, session, assets, "")
	withheldMediaID := addWithholdingTestMedia(t, router, session, withheldID)
	deletedMediaID := addWithholdingTestMedia(t, router, session, deletedID)

	if _, err := pool.Exec(context.Background(), `
		update assets asset
		   set withheld_at = now(), withheld_by = owner.id, withheld_reason = 'Review'
		  from users owner
		 where asset.id = $1 and owner.username = 'verified.creator'
	`, withheldID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`update assets set deleted_at = now(), recoverable_until = now() + interval '30 days' where id = $1`, deletedID,
	); err != nil {
		t.Fatalf("delete asset: %v", err)
	}

	missingAssetID := "22222222-2222-4222-8222-222222222222"
	missingMediaID := "33333333-3333-4333-8333-333333333333"
	for name, paths := range map[string][]string{
		"asset page": {
			"/v1/assets/" + withheldID,
			"/v1/assets/" + deletedID,
			"/v1/assets/" + missingAssetID,
		},
		"download": {
			"/download/" + withheldID,
			"/download/" + deletedID,
			"/download/" + missingAssetID,
		},
		"media list": {
			"/v1/assets/" + withheldID + "/media",
			"/v1/assets/" + deletedID + "/media",
			"/v1/assets/" + missingAssetID + "/media",
		},
		"media file": {
			"/media/" + withheldMediaID + "/grid/1",
			"/media/" + deletedMediaID + "/grid/1",
			"/media/" + missingMediaID + "/grid/1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var first *httptest.ResponseRecorder
			for _, path := range paths {
				response := send(t, router, httptest.NewRequest(http.MethodGet, path, nil))
				if response.Code != http.StatusNotFound {
					t.Fatalf("GET %s status = %d, want 404: %s", path, response.Code, response.Body.String())
				}
				if first == nil {
					first = response
					continue
				}
				if response.Body.String() != first.Body.String() ||
					!reflect.DeepEqual(response.Header(), first.Header()) {
					t.Fatalf("GET %s response differs: body %q headers %v; want body %q headers %v",
						path, response.Body.String(), response.Header(), first.Body.String(), first.Header())
				}
			}
		})
	}
}

func TestWithheldAssetRefusesCreatorMutations(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, "")
	if _, err := pool.Exec(context.Background(), `
		update assets asset
		   set withheld_at = now(), withheld_by = owner.id, withheld_reason = 'Review'
		  from users owner
		 where asset.id = $1 and owner.username = 'verified.creator'
	`, assetID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}

	changes := []*http.Request{
		authorizedJSONRequest(
			t, http.MethodPut, "/v1/assets/"+assetID+"/discovery",
			`{"discovery":"unlisted"}`, session,
		),
		authorized(mediaUploadRequest(
			t, assetID, "gallery", httpTestPNG(t, 2, 2),
		), session),
	}
	for _, request := range changes {
		response := send(t, router, request)
		if response.Code != http.StatusConflict {
			t.Fatalf("%s %s status = %d, want 409: %s",
				request.Method, request.URL.Path, response.Code, response.Body.String())
		}
	}

	clear := httptest.NewRequest(http.MethodDelete, "/v1/assets/"+assetID+"/withhold", nil)
	clear.AddCookie(session)
	response := send(t, router, clear)
	if response.Code != http.StatusForbidden {
		t.Fatalf("creator clear status = %d, want 403: %s", response.Code, response.Body.String())
	}
}

func TestWithheldAssetRefusesEveryProtectedPromptMutation(t *testing.T) {
	_, router, session, _, pool := newVerifiedTestRoutersWithPool(t, 1<<20, DefaultDeadlines())
	started := startPreset(t, router, session, "lumiverse")
	coreBlock := blockNamed(t, started.Blocks, "preset_core")
	core := editableBlock(coreBlock)
	const privateText = "This prompt stays frozen."
	core.Elements[0].Content = json.RawMessage(`{"groups":[],"fragments":[
		{"name":"Frozen","role":"system","text":"` + privateText + `","protected":true,"enabled":true}
	]}`)
	apps := []string{"lumiverse"}
	core.AllowedApps = &apps
	if response := saveBlock(t, router, session, started.ID, coreBlock.ID, core); response.Code != http.StatusOK {
		t.Fatalf("save sealed prompt: %d %s", response.Code, response.Body.String())
	}
	before := contentGeneration(t, pool, started.ID)
	owner := fetchStartedAsset(t, router, session, started.ID)
	core = editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	if _, err := pool.Exec(t.Context(), `
		update assets asset
		   set withheld_at = now(), withheld_by = owner.id, withheld_reason = 'Review'
		  from users owner
		 where asset.id = $1 and owner.username = 'verified.creator'
	`, started.ID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}

	textChange := editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	textChange.Elements[0].Content = json.RawMessage(strings.Replace(
		string(core.Elements[0].Content), privateText, "This prompt tried to change.", 1,
	))
	stateChange := editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	stateChange.Elements[0].Content = json.RawMessage(strings.Replace(
		string(core.Elements[0].Content), `,"protected":true`, "", 1,
	))
	stateChange.AllowedApps = &[]string{}
	policyChange := editableBlock(blockNamed(t, owner.Blocks, "preset_core"))
	policyChange.AllowedApps = &[]string{}
	mutations := map[string]saveBlockBody{
		"protected text":   textChange,
		"protection state": stateChange,
		"allowed apps":     policyChange,
	}

	for name, mutation := range mutations {
		t.Run(name, func(t *testing.T) {
			response := saveBlock(t, router, session, started.ID, coreBlock.ID, mutation)
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, want 409: %s", response.Code, response.Body.String())
			}
		})
	}

	var storedText string
	if err := pool.QueryRow(t.Context(), `
		select payload ->> 'text' from protected_content where asset_id = $1
	`, started.ID).Scan(&storedText); err != nil {
		t.Fatalf("read protected prompt after refused saves: %v", err)
	}
	if storedText != privateText {
		t.Fatalf("protected prompt after refused saves = %q, want %q", storedText, privateText)
	}
	if payloads, policies := protectedCounts(t, pool, started.ID); payloads != 1 || policies != 1 {
		t.Fatalf("after refused saves: %d payloads and %d policy rows, want 1 and 1", payloads, policies)
	}
	if got := contentGeneration(t, pool, started.ID); got != before {
		t.Fatalf("content generation after refused saves = %d, want %d", got, before)
	}
}

func addWithholdingTestMedia(
	t *testing.T,
	router http.Handler,
	session *http.Cookie,
	assetID string,
) string {
	t.Helper()
	response := send(t, router, authorized(mediaUploadRequest(
		t, assetID, "gallery", httpTestPNG(t, 1, 1),
	), session))
	if response.Code != http.StatusCreated {
		t.Fatalf("add media status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var media struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &media); err != nil {
		t.Fatalf("decode media: %v", err)
	}
	return media.ID
}
