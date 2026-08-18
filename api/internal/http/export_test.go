package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const aPlainCard = `{
	"spec":"chara_card_v3","spec_version":"3.0",
	"data":{"name":"Ana","description":"Keeps the archive.","first_mes":"Hello"}
}`

func downloadMenu(t *testing.T, r http.Handler, session *http.Cookie, assetID string) []downloadTarget {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/assets/"+assetID, nil)
	if session != nil {
		request = authorized(request, session)
	}
	response := send(t, r, request)
	if response.Code != http.StatusOK {
		t.Fatalf("read the asset: status = %d: %s", response.Code, response.Body.String())
	}
	var page startedAsset
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode the asset: %v", err)
	}
	return page.Downloads
}

func targetLine(t *testing.T, menu []downloadTarget, formatID string) downloadTarget {
	t.Helper()
	for _, target := range menu {
		if target.Format == formatID {
			return target
		}
	}
	t.Fatalf("%s is not on the menu: %+v", formatID, menu)
	return downloadTarget{}
}

func losses(target downloadTarget) []roleVerdict {
	lost := make([]roleVerdict, 0, len(target.Roles))
	for _, role := range target.Roles {
		if role.Verdict != "carried" {
			lost = append(lost, role)
		}
	}
	return lost
}

// A character offers all three card formats, and the loss report is checked
// against the asset rather than fixed per format: the same writer reports
// nothing for a plain card and a dropped role once the asset has expressions.
func TestTheLossReportIsCheckedAgainstTheAssetAndNotTheFormat(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)

	plain := downloadMenu(t, r, session, assetID)
	if len(plain) != 3 {
		t.Fatalf("menu = %+v, want all three character formats", plain)
	}
	if lost := losses(targetLine(t, plain, "chara_card_v2")); len(lost) != 0 {
		t.Fatalf("CCv2 reported %+v for a card that has none of what it drops", lost)
	}

	giveExpressions(t, r, session, assetID)

	withImages := downloadMenu(t, r, session, assetID)
	lost := losses(targetLine(t, withImages, "chara_card_v2"))
	if len(lost) != 1 || lost[0].Role != "expressions" || lost[0].Verdict != "dropped" {
		t.Fatalf("CCv2 losses = %+v, want the expressions dropped", lost)
	}
	if lost[0].Sample.Count != 1 || len(lost[0].Sample.Images) != 1 {
		t.Errorf("sample = %+v, want the picture that is at stake", lost[0].Sample)
	}
	// Nothing is withheld for losing optional content, so the target is still
	// on the menu with the loss stated.
	if len(withImages) != 3 {
		t.Fatalf("menu = %+v, want the lossy target still offered", withImages)
	}
}

// The recommendation is the widest-compatibility rule: CharX carries at least
// as much and is still not recommended, because SillyTavern cannot open one.
func TestTheRecommendationIsTheFormatMostAppsCanOpen(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	giveExpressions(t, r, session, assetID)
	givePictures(t, r, session, assetID, "gallery", "gallery")

	menu := downloadMenu(t, r, session, assetID)
	recommended := ""
	for _, target := range menu {
		if target.Recommended {
			recommended = target.Format
		}
	}
	if recommended != "chara_card_v3" {
		t.Fatalf("recommended = %q, want the format most apps can open", recommended)
	}
	if len(losses(targetLine(t, menu, "charx"))) >
		len(losses(targetLine(t, menu, "chara_card_v3"))) {
		t.Fatal("CharX lost more here, so least loss would have chosen CCv3 anyway")
	}
	// A gallery has no standard home in either, so its verdict carries a note
	// about where it actually lands.
	gallery := roleVerdictNamed(t, targetLine(t, menu, "chara_card_v3"), "gallery")
	if gallery.Verdict != "carried" || gallery.Destination == "" {
		t.Fatalf("gallery verdict = %+v, want carried with a destination note", gallery)
	}
}

func roleVerdictNamed(t *testing.T, target downloadTarget, role string) roleVerdict {
	t.Helper()
	for _, found := range target.Roles {
		if found.Role == role {
			return found
		}
	}
	t.Fatalf("%s has no verdict for %s: %+v", target.Format, role, target.Roles)
	return roleVerdict{}
}

// The creator's panel is the reader's, read from the same projection.
func TestTheDownloadMenuReadsTheSameForItsOwnerAndAStranger(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	publishCharacter(t, r, session, assetID)

	owner, stranger := downloadMenu(t, r, session, assetID), downloadMenu(t, r, nil, assetID)
	if !json.Valid(mustJSON(t, owner)) || string(mustJSON(t, owner)) != string(mustJSON(t, stranger)) {
		t.Fatalf("the owner reads %s and a reader reads %s",
			mustJSON(t, owner), mustJSON(t, stranger))
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode for comparison: %v", err)
	}
	return encoded
}

// The upload is its own group, labelled by what it is and when it arrived, and
// an asset built from nothing simply has none.
func TestTheOriginalUploadStandsApartAndOnlyWhereThereIsOne(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)

	uploaded := fetchStartedAsset(t, r, session, assetID)
	if uploaded.Original == nil {
		t.Fatal("an uploaded card has no original upload group")
	}
	if uploaded.Original.Label != "Character Card V3" || uploaded.Original.ArrivedAt == "" {
		t.Fatalf("original = %+v, want it labelled by what it is and when it came", uploaded.Original)
	}
	for _, target := range uploaded.Downloads {
		if target.Format == "raw" {
			t.Fatal("the upload was listed beside the generated downloads")
		}
	}

	built := startCharacter(t, r, session)
	fromNothing := fetchStartedAsset(t, r, session, built.ID)
	if fromNothing.Original != nil {
		t.Fatalf("an asset built from nothing carries %+v", fromNothing.Original)
	}
	// The builder is an origin in its own right, so all three targets stand.
	if len(fromNothing.Downloads) != 3 {
		t.Fatalf("menu = %+v, want all three character targets", fromNothing.Downloads)
	}
}

// The projection is written with the change that caused it, for a draft as
// much as a published asset, and publishing computes nothing.
func TestTheProjectionIsWrittenWithTheChangeAndPublishingComputesNothing(t *testing.T) {
	r, session, assets, pool := newCharacterIngestRouterWithPool(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)

	before := projectionComputedAt(t, pool, assetID)
	giveExpressions(t, r, session, assetID)
	afterEdit := projectionComputedAt(t, pool, assetID)
	if !afterEdit.After(before) {
		t.Fatal("editing a block left the export projection where it was")
	}

	publishCharacter(t, r, session, assetID)
	if afterPublish := projectionComputedAt(t, pool, assetID); !afterPublish.Equal(afterEdit) {
		t.Fatal("publishing recomputed the export projection")
	}
}

// Hiding is a promise about a page and an export is a promise about a file, so
// a hidden block leaves the download alone and its content still travels.
func TestHidingASectionLeavesTheDownloadAlone(t *testing.T) {
	r, session, assets, pool := newCharacterIngestRouterWithPool(t)
	assetID := uploadedCharacterID(t, r, session, assets, aPlainCard)
	giveExpressions(t, r, session, assetID)
	publishCharacter(t, r, session, assetID)

	before := projectionComputedAt(t, pool, assetID)
	page := fetchStartedAsset(t, r, session, assetID)
	arrangement := make([]arrangedBlock, 0, len(page.Blocks))
	for _, holder := range page.Blocks {
		arrangement = append(arrangement, arrangedBlock{
			ID: holder.ID, Hidden: holder.Definition == "expressions", Width: holder.Width,
		})
	}
	arranged := arrangeBlocks(t, r, session, assetID, arrangement)
	if arranged.Code != http.StatusOK {
		t.Fatalf("hide the expressions: %d %s", arranged.Code, arranged.Body.String())
	}
	if after := projectionComputedAt(t, pool, assetID); !after.Equal(before) {
		t.Fatal("hiding a section moved the export section of the projection")
	}

	export, err := assets.OpenExport(
		context.Background(), uuid.MustParse(assetID), nil, "chara_card_v3",
	)
	if err != nil {
		t.Fatalf("export a card with a hidden section: %v", err)
	}
	if !containsBytes(export.Body, []byte("emotion")) {
		t.Fatal("a hidden section's content did not travel in the download")
	}
}

// giveExpressions adds a picture and puts it in an expressions section, which
// is a role two of the three character formats have nowhere to put.
func giveExpressions(t *testing.T, r http.Handler, session *http.Cookie, assetID string) {
	t.Helper()
	givePictures(t, r, session, assetID, "expression", "expressions")
}

// givePictures adds one picture in a role and puts it in the section that
// carries that role.
func givePictures(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID, mediaRole, definition string,
) {
	t.Helper()
	added := send(t, r, authorized(mediaUploadRequest(
		t, assetID, mediaRole, httpTestPNG(t, 64, 64),
	), session))
	if added.Code != http.StatusCreated {
		t.Fatalf("add an expression: %d %s", added.Code, added.Body.String())
	}
	var picture struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(added.Body.Bytes(), &picture); err != nil {
		t.Fatalf("decode the added picture: %v", err)
	}
	section := addedBlock(t, addBlock(t, r, session, assetID, definition, "image_set"))
	body := editableBlock(section)
	body.Elements[0].Content = json.RawMessage(
		`{"images":[{"mediaId":"` + picture.ID + `","name":"happy"}]}`,
	)
	if saved := saveBlock(t, r, session, assetID, section.ID, body); saved.Code != http.StatusOK {
		t.Fatalf("save the %s section: %d %s", definition, saved.Code, saved.Body.String())
	}
}

// publishCharacter fills the floor an uploaded character still misses and
// publishes it.
func publishCharacter(t *testing.T, r http.Handler, session *http.Cookie, assetID string) {
	t.Helper()
	if got := saveIdentity(t, r, session, assetID,
		`{"name":"Ana","isNsfw":false}`); got.Code != http.StatusNoContent {
		t.Fatalf("save identity: %d %s", got.Code, got.Body.String())
	}
	if got := publishAsset(t, r, session, assetID); got.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", got.Code, got.Body.String())
	}
}

func newCharacterIngestRouterWithPool(
	t *testing.T,
) (*gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	return newVerifiedIngestRouterWithPool(t, registry)
}

func projectionComputedAt(t *testing.T, pool *pgxpool.Pool, assetID string) time.Time {
	t.Helper()
	var computedAt time.Time
	if err := pool.QueryRow(context.Background(), `
		select computed_at from asset_projections where asset_id = $1
	`, assetID).Scan(&computedAt); err != nil {
		t.Fatalf("read the export projection: %v", err)
	}
	return computedAt
}

func containsBytes(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
