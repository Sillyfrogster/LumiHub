package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

func TestUploadAcceptsDiscoveryAndDefaultsToListed(t *testing.T) {
	for _, test := range []struct {
		name      string
		discovery asset.Discovery
		want      string
	}{
		{name: "omitted", want: "listed"},
		{name: "explicit unlisted", discovery: asset.DiscoveryUnlisted, want: "unlisted"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
			assetID := uploadDiscoveryTestAsset(t, router, session, assets, test.discovery)

			page := fetchAssetPage(t, router, "/v1/assets/"+assetID)
			if page.Discovery != test.want {
				t.Fatalf("discovery = %q, want %q", page.Discovery, test.want)
			}
		})
	}
}

func TestCreatorChangesAssetDiscovery(t *testing.T) {
	router, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

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
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)

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
	assetID := uploadDiscoveryTestAsset(t, router, session, assets, asset.DiscoveryListed)
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

func uploadDiscoveryTestAsset(
	t *testing.T,
	router http.Handler,
	session *http.Cookie,
	assets *asset.Service,
	discovery asset.Discovery,
) string {
	t.Helper()
	metadata := exampleMetadata("A quiet draft")
	metadata["filename"] = "quiet-draft.lumitheme"
	if discovery == "" {
		delete(metadata, "discovery")
	} else {
		metadata["discovery"] = discovery
	}
	return assetIDFromIngest(
		t, uploadAndFinish(t, router, session, assets, metadata, []byte("theme")),
	)
}
