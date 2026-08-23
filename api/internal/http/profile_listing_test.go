package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/google/uuid"
)

type profileListingResponse struct {
	Items []struct {
		Name       string  `json:"name"`
		IsNsfw     *bool   `json:"isNsfw"`
		OwnerState *string `json:"ownerState"`
		Withhold   *struct {
			Reason string    `json:"reason"`
			Actor  string    `json:"actor"`
			At     time.Time `json:"at"`
		} `json:"withhold"`
	} `json:"items"`
	Total      int `json:"total"`
	Suppressed int `json:"suppressed"`
}

func TestCreatorProfileScopesTheBrowseListing(t *testing.T) {
	router, _, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	var firstID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`select id from users where username = $1`, "verified.creator").Scan(&firstID); err != nil {
		t.Fatalf("read first creator: %v", err)
	}
	secondID := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username) values ($1, $2)`, secondID, "second.creator"); err != nil {
		t.Fatalf("insert second creator: %v", err)
	}
	createProfileAsset(t, assets, firstID, "First garden", false, asset.DiscoveryListed)
	createProfileAsset(t, assets, secondID, "Second garden", false, asset.DiscoveryListed)

	response := send(t, router, httptest.NewRequest(
		http.MethodGet, "/v1/assets?creator=verified.creator", nil,
	))
	if response.Code != http.StatusOK {
		t.Fatalf("profile listing status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var body profileListingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile listing: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Name != "First garden" {
		t.Fatalf("profile listing = %+v, want only the requested creator's asset", body)
	}
}

func TestCreatorProfileKeepsCreatorAndReaderAdultContentChoicesSeparate(t *testing.T) {
	router, _, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	var creatorID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`select id from users where username = $1`, "verified.creator").Scan(&creatorID); err != nil {
		t.Fatalf("read creator: %v", err)
	}
	createProfileAsset(t, assets, creatorID, "Open garden", false, asset.DiscoveryListed)
	createProfileAsset(t, assets, creatorID, "Midnight garden", true, asset.DiscoveryListed)

	browse := readProfileListing(t, router, "/v1/assets?nsfw=shown", nil)
	if names(browse) == nil || !slices.Equal(names(browse), []string{"Midnight garden", "Open garden"}) {
		t.Fatalf("browse returned %v, want both assets regardless of the profile setting", names(browse))
	}

	privateProfile := readProfileListing(
		t, router, "/v1/assets?creator=verified.creator&nsfw=shown", nil,
	)
	if !slices.Equal(names(privateProfile), []string{"Open garden"}) || privateProfile.Suppressed != 0 {
		t.Fatalf("profile with adult contributions disabled = %+v", privateProfile)
	}

	if _, err := pool.Exec(context.Background(), `
		update users set show_nsfw_contributions_on_profile = true where id = $1
	`, creatorID); err != nil {
		t.Fatalf("show adult contributions on profile: %v", err)
	}
	hidden := readProfileListing(
		t, router, "/v1/assets?creator=verified.creator&nsfw=hidden", nil,
	)
	if !slices.Equal(names(hidden), []string{"Open garden"}) || hidden.Suppressed != 1 {
		t.Fatalf("hidden reader preference = %+v", hidden)
	}
	blurred := readProfileListing(
		t, router, "/v1/assets?creator=verified.creator&nsfw=blurred", nil,
	)
	if !slices.Equal(names(blurred), []string{"Midnight garden", "Open garden"}) {
		t.Fatalf("blurred reader preference returned %v, want both profile assets", names(blurred))
	}
}

func TestOwnerProfileAlwaysListsActiveWorkWithoutChangingBrowse(t *testing.T) {
	router, session, assets, pool := newVerifiedIngestRouterWithPool(t, format.NewRegistry())
	var creatorID uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`select id from users where username = $1`, "verified.creator").Scan(&creatorID); err != nil {
		t.Fatalf("read creator: %v", err)
	}
	createProfileAsset(t, assets, creatorID, "Public garden", false, asset.DiscoveryListed)
	createProfileAsset(t, assets, creatorID, "Adult garden", true, asset.DiscoveryListed)
	createProfileAsset(t, assets, creatorID, "Unlisted garden", false, asset.DiscoveryUnlisted)
	withheldID := createProfileAsset(
		t, assets, creatorID, "Withheld garden", false, asset.DiscoveryListed,
	)
	deletedID := createProfileAsset(
		t, assets, creatorID, "Deleted garden", false, asset.DiscoveryListed,
	)
	if _, err := pool.Exec(context.Background(), `
		update assets
		   set withheld_at = now(), withheld_by = $2, withheld_reason = 'testing'
		 where id = $1
	`, withheldID, creatorID); err != nil {
		t.Fatalf("withhold asset: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`update assets set deleted_at = now(), recoverable_until = now() + interval '30 days' where id = $1`, deletedID); err != nil {
		t.Fatalf("soft-delete asset: %v", err)
	}

	saved := send(t, router, authorizedJSONRequest(
		t, http.MethodPut, "/v1/account/nsfw-visibility", `{"visibility":"hidden"}`, session,
	))
	if saved.Code != http.StatusNoContent {
		t.Fatalf("save hidden preference status = %d: %s", saved.Code, saved.Body.String())
	}

	stranger := readProfileListing(
		t, router, "/v1/assets?creator=verified.creator&nsfw=shown", nil,
	)
	if !slices.Equal(names(stranger), []string{"Public garden"}) {
		t.Fatalf("stranger profile returned %v, want only the public safe asset", names(stranger))
	}

	owner := readProfileListing(t, router, "/v1/assets?creator=verified.creator", session)
	if owner.Total != 4 || owner.Suppressed != 0 {
		t.Fatalf("owner profile counts = total %d, suppressed %d; want 4, 0",
			owner.Total, owner.Suppressed)
	}
	states := make(map[string]*string, len(owner.Items))
	for _, item := range owner.Items {
		states[item.Name] = item.OwnerState
	}
	if len(states) != 4 || states["Public garden"] != nil || states["Adult garden"] != nil ||
		states["Unlisted garden"] == nil || *states["Unlisted garden"] != "unlisted" ||
		states["Withheld garden"] == nil || *states["Withheld garden"] != "withheld" {
		t.Fatalf("owner profile states = %#v", states)
	}
	for _, item := range owner.Items {
		if item.Name == "Withheld garden" &&
			(item.Withhold == nil || item.Withhold.Reason != "testing" ||
				item.Withhold.Actor != "verified.creator" || item.Withhold.At.IsZero()) {
			t.Fatalf("owner profile withhold = %+v", item.Withhold)
		}
	}
	if _, exists := states["Deleted garden"]; exists {
		t.Fatalf("soft-deleted asset appeared in the active owner listing")
	}

	browse := readProfileListing(t, router, "/v1/assets", session)
	if !slices.Equal(names(browse), []string{"Public garden"}) || browse.Suppressed != 1 {
		t.Fatalf("ordinary browse changed for the owner: %+v", browse)
	}
}

func readProfileListing(
	t *testing.T,
	router http.Handler,
	path string,
	session *http.Cookie,
) profileListingResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if session != nil {
		request.AddCookie(session)
	}
	response := send(t, router, request)
	if response.Code != http.StatusOK {
		t.Fatalf("profile listing status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body profileListingResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile listing: %v", err)
	}
	return body
}

func names(list profileListingResponse) []string {
	result := make([]string, len(list.Items))
	for i, item := range list.Items {
		result[i] = item.Name
	}
	return result
}

func createProfileAsset(
	t *testing.T,
	assets *asset.Service,
	ownerID uuid.UUID,
	name string,
	isNSFW bool,
	discovery asset.Discovery,
) uuid.UUID {
	t.Helper()
	created, err := assets.Create(context.Background(), asset.CreateInput{
		OwnerID: ownerID, Kind: "theme", Filename: name + ".lumitheme",
		File: bytes.NewReader([]byte(name)), Name: name, IsNSFW: isNSFW,
		Discovery: discovery,
	})
	if err != nil {
		t.Fatalf("create profile asset %q: %v", name, err)
	}
	return created.ID
}
