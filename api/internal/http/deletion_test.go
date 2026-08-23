package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
)

func TestCreatorCanDeleteAndRestoreAnAssetDuringItsRecoveryWindow(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	assetID := assetIDFromIngest(t, uploadAndFinish(
		t, router, session, assets,
		withFilename(exampleMetadata("Recoverable garden"), "recoverable-garden"),
		[]byte("the retained source"),
	))

	deleted := send(t, router, authorized(
		httptest.NewRequest(http.MethodDelete, "/v1/assets/"+assetID, nil), session,
	))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204: %s", deleted.Code, deleted.Body.String())
	}

	page := send(t, router, httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil))
	if page.Code != http.StatusNotFound {
		t.Fatalf("deleted asset page status = %d, want 404: %s", page.Code, page.Body.String())
	}
	browse := readProfileListing(t, router, "/v1/assets?creator=verified.creator", session)
	if len(browse.Items) != 0 {
		t.Fatalf("active owner listing after delete = %+v, want empty", browse.Items)
	}

	listed := send(t, router, authorized(
		httptest.NewRequest(http.MethodGet, "/v1/profiles/verified.creator/deleted", nil), session,
	))
	if listed.Code != http.StatusOK {
		t.Fatalf("deleted listing status = %d, want 200: %s", listed.Code, listed.Body.String())
	}
	var recovery struct {
		Items []struct {
			ID               string    `json:"id"`
			Name             string    `json:"name"`
			Kind             string    `json:"kind"`
			DeletedAt        time.Time `json:"deletedAt"`
			RecoverableUntil time.Time `json:"recoverableUntil"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &recovery); err != nil {
		t.Fatalf("decode deleted listing: %v", err)
	}
	if len(recovery.Items) != 1 || recovery.Items[0].ID != assetID ||
		recovery.Items[0].Name != "Recoverable garden" || recovery.Items[0].Kind != "character" {
		t.Fatalf("deleted listing = %+v, want the deleted asset", recovery.Items)
	}
	if !recovery.Items[0].RecoverableUntil.After(recovery.Items[0].DeletedAt) {
		t.Fatalf("recovery deadline = %v, deleted at %v", recovery.Items[0].RecoverableUntil, recovery.Items[0].DeletedAt)
	}

	restored := send(t, router, authorized(
		httptest.NewRequest(http.MethodPost, "/v1/assets/"+assetID+"/restore", nil), session,
	))
	if restored.Code != http.StatusNoContent {
		t.Fatalf("restore status = %d, want 204: %s", restored.Code, restored.Body.String())
	}
	if got := fetchAssetPage(t, router, "/v1/assets/"+assetID); got.ID != assetID {
		t.Fatalf("restored asset id = %q, want %q", got.ID, assetID)
	}
	download := send(t, router, httptest.NewRequest(http.MethodGet, "/download/"+assetID, nil))
	if download.Code != http.StatusOK || download.Header().Get("X-Accel-Redirect") == "" {
		t.Fatalf("restored download = %d, headers %v", download.Code, download.Header())
	}
}

func TestUploadRefusesBytesNamedByAPurgeTombstone(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	file := []byte("bytes that cannot return")
	assetID := assetIDFromIngest(t, uploadAndFinish(
		t, router, session, assets,
		withFilename(exampleMetadata("Gone for good"), "gone-for-good"), file,
	))
	var digest []byte
	if err := pool.QueryRow(context.Background(), `
		select blob.sha256
		  from asset_revisions revision
		  join blobs blob on blob.id = revision.blob_id
		 where revision.asset_id = $1
	`, assetID).Scan(&digest); err != nil {
		t.Fatalf("read asset digest: %v", err)
	}
	var contentDigest [32]byte
	copy(contentDigest[:], digest)
	actorID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, 'purge.actor')`, actorID,
	); err != nil {
		t.Fatalf("insert purge actor: %v", err)
	}
	if err := assets.Purge(context.Background(), contentDigest, "legal_order", actorID); err != nil {
		t.Fatalf("purge: %v", err)
	}

	reupload := send(t, router, authorized(
		uploadRequest(t, withFilename(exampleMetadata("Attempted return"), "attempted-return"), file),
		session,
	))
	if reupload.Code != http.StatusUnprocessableEntity {
		t.Fatalf("purged re-upload status = %d, want 422: %s", reupload.Code, reupload.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(reupload.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	if body["error"] != "This file cannot be accepted." {
		t.Fatalf("purged re-upload error = %q", body["error"])
	}
}

func TestDeletedListingBelongsOnlyToItsOwner(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	assetID := assetIDFromIngest(t, uploadAndFinish(
		t, router, session, assets,
		withFilename(exampleMetadata("Private recovery"), "private-recovery"), []byte("source"),
	))
	response := send(t, router, authorized(
		httptest.NewRequest(http.MethodDelete, "/v1/assets/"+assetID, nil), session,
	))
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", response.Code, response.Body.String())
	}

	stranger := send(t, router, httptest.NewRequest(
		http.MethodGet, "/v1/profiles/verified.creator/deleted", nil,
	))
	if stranger.Code != http.StatusUnauthorized {
		t.Fatalf("stranger deleted listing status = %d, want 401", stranger.Code)
	}
}
