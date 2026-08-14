package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

func TestUploadAcceptsDiscoveryAndDefaultsToListed(t *testing.T) {
	for _, test := range []struct {
		name      string
		discovery any
		want      string
	}{
		{name: "omitted", want: "listed"},
		{name: "explicit unlisted", discovery: "unlisted", want: "unlisted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
			metadata := exampleMetadata("A quiet draft")
			metadata["filename"] = "quiet-draft.lumitheme"
			if test.discovery == nil {
				delete(metadata, "discovery")
			} else {
				metadata["discovery"] = test.discovery
			}
			assetID := assetIDFromIngest(
				t, uploadAndFinish(t, router, session, assets, metadata, []byte("theme")),
			)

			page := fetchAssetPage(t, router, "/v1/assets/"+assetID)
			if page.Discovery != test.want {
				t.Fatalf("discovery = %q, want %q", page.Discovery, test.want)
			}
		})
	}
}

func TestCreatorChangesAssetDiscovery(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("A quiet draft")
	metadata["filename"] = "quiet-draft.lumitheme"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte("theme")),
	)

	changed := send(t, router, authorizedJSONRequest(
		t,
		http.MethodPut,
		"/v1/assets/"+assetID+"/discovery",
		`{"discovery":"unlisted"}`,
		session,
	))
	if changed.Code != http.StatusNoContent {
		t.Fatalf("change discovery status = %d, want 204: %s", changed.Code, changed.Body.String())
	}

	page := fetchAssetPage(t, router, "/v1/assets/"+assetID)
	if page.Discovery != "unlisted" {
		t.Fatalf("discovery = %q, want unlisted", page.Discovery)
	}
}

func TestChangingDiscoveryRequiresTheCreator(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	metadata := exampleMetadata("A quiet draft")
	metadata["filename"] = "quiet-draft.lumitheme"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte("theme")),
	)

	changed := send(t, router, httptest.NewRequest(
		http.MethodPut,
		"/v1/assets/"+assetID+"/discovery",
		nil,
	))
	if changed.Code != http.StatusUnauthorized {
		t.Fatalf("change discovery status = %d, want 401: %s", changed.Code, changed.Body.String())
	}
}

func TestWithheldAssetDiscoveryIsFrozen(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	metadata := exampleMetadata("A quiet draft")
	metadata["filename"] = "quiet-draft.lumitheme"
	assetID := assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte("theme")),
	)
	var ownerID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`select id from users where username = 'verified.creator'`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("read creator: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		update assets
		   set withheld_at = now(), withheld_by = $2, withheld_reason = 'testing'
		 where id = $1
	`, assetID, ownerID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}

	changed := send(t, router, authorizedJSONRequest(
		t,
		http.MethodPut,
		"/v1/assets/"+assetID+"/discovery",
		`{"discovery":"unlisted"}`,
		session,
	))
	if changed.Code != http.StatusConflict {
		t.Fatalf("change discovery status = %d, want 409: %s", changed.Code, changed.Body.String())
	}
}
