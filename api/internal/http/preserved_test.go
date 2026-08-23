package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/character"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// aCardCarryingThirdPartyNamespaces exercises body, extension, and entry data.
const aCardCarryingThirdPartyNamespaces = `{
	"spec":"chara_card_v3","spec_version":"3.0",
	"data":{
		"name":"Ana","description":"Keeps the archive.","personality":"Patient",
		"scenario":"After closing","first_mes":"Welcome back.",
		"tags":["archivist"],
		"character_book":{"name":"Ana's world","scan_depth":4,
			"entries":[
				{"keys":["ledger"],"content":"A debt.","uid":91,"probability":75},
				{"keys":["mira"],"content":"A name.","uid":92,"group":"people"}]},
		"extensions":{
			"chub":{"full_path":"ana/quiet","related_lorebooks":[]},
			"tavern_helper":{"scripts":[{"name":"Opening"}]},
			"depth_prompt":{"depth":4,"prompt":"","role":"system"},
			"talkativeness":"0.5","fav":false,"world":""
		}
	}
}`

func newCharacterIngestRouter(t *testing.T) (*gin.Engine, *http.Cookie, *asset.Service) {
	t.Helper()
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	return newVerifiedIngestRouter(t, registry)
}

func uploadedCharacterID(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assets *asset.Service,
	card string,
) string {
	t.Helper()
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.json"
	metadata["_keepDraft"] = true
	return assetIDFromIngest(t, uploadAndFinish(t, r, session, assets, metadata, []byte(card)))
}

func preservedNamespaces(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID string,
) []struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
} {
	t.Helper()
	response := send(t, r, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+assetID+"/preserved", nil,
	), session))
	if response.Code != http.StatusOK {
		t.Fatalf("read preserved data: status = %d: %s", response.Code, response.Body.String())
	}
	var found []struct {
		Name  string `json:"name"`
		Bytes int    `json:"bytes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode preserved namespaces: %v", err)
	}
	return found
}

func namespaceNames(rows []struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
}) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}

// The panel names what an asset carries and leaves out the namespaces the
// module calls boilerplate. Those are still stored; only the panel skips them.
func TestThePanelNamesTheNamespacesAnAssetCarries(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aCardCarryingThirdPartyNamespaces)

	names := namespaceNames(preservedNamespaces(t, r, session, assetID))
	for _, want := range []string{"card", "character_book", "chub", "tavern_helper"} {
		if !contains(names, want) {
			t.Errorf("the panel does not name %s: %v", want, names)
		}
	}
	for _, hidden := range []string{"depth_prompt", "fav", "world", "talkativeness"} {
		if contains(names, hidden) {
			t.Errorf("the panel shows %s, which records nothing: %v", hidden, names)
		}
	}
}

// Deleting is the only thing the panel does besides naming, and it is
// permanent.
func TestACreatorDeletesOneNamespaceAndKeepsTheRest(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aCardCarryingThirdPartyNamespaces)

	response := send(t, r, authorized(httptest.NewRequest(
		http.MethodDelete, "/v1/assets/"+assetID+"/preserved/chub", nil,
	), session))
	if response.Code != http.StatusNoContent {
		t.Fatalf("delete chub: status = %d: %s", response.Code, response.Body.String())
	}

	names := namespaceNames(preservedNamespaces(t, r, session, assetID))
	if contains(names, "chub") {
		t.Errorf("chub survived its deletion: %v", names)
	}
	if !contains(names, "tavern_helper") {
		t.Errorf("deleting chub cost the namespace beside it: %v", names)
	}

	again := send(t, r, authorized(httptest.NewRequest(
		http.MethodDelete, "/v1/assets/"+assetID+"/preserved/chub", nil,
	), session))
	if again.Code != http.StatusNotFound {
		t.Errorf("deleting chub twice = %d, want 404", again.Code)
	}
}

// Preserved data belongs to the file and never to the page. Nobody but the
// owner can even ask what an asset carries.
func TestPreservedDataNeverRendersOnThePage(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aCardCarryingThirdPartyNamespaces)

	page := send(t, r, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+assetID, nil,
	), session))
	body := page.Body.String()
	for _, namespace := range []string{"chub", "tavern_helper", "ana/quiet", "uid"} {
		if strings.Contains(body, namespace) {
			t.Errorf("the page carries preserved data: %s is in it", namespace)
		}
	}

	stranger := send(t, r, httptest.NewRequest(
		http.MethodGet, "/v1/assets/"+assetID+"/preserved", nil,
	))
	if stranger.Code != http.StatusUnauthorized {
		t.Errorf("a signed-out reader asked what an asset preserves and got %d", stranger.Code)
	}
}

func TestEditingABlockLeavesEveryPreservedKeyUntouched(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aCardCarryingThirdPartyNamespaces)
	before := preservedNamespaces(t, r, session, assetID)

	page := fetchStartedAsset(t, r, session, assetID)
	core := editableBlock(blockNamed(t, page.Blocks, "character_core"))
	core.Elements[0].Content = json.RawMessage(`{"text":"Keeps the archive, and the ledger."}`)
	saved := saveBlock(t, r, session, assetID, blockNamed(t, page.Blocks, "character_core").ID, core)
	if saved.Code != http.StatusOK {
		t.Fatalf("save the description: status = %d: %s", saved.Code, saved.Body.String())
	}

	after := preservedNamespaces(t, r, session, assetID)
	if len(before) != len(after) {
		t.Fatalf("preserved namespaces = %v, were %v", after, before)
	}
	for index := range before {
		if before[index] != after[index] {
			t.Errorf("%s changed from %d bytes to %d",
				before[index].Name, before[index].Bytes, after[index].Bytes)
		}
	}
}

// Deleting an entry deletes its preserved data with it, and the entry beside
// it keeps its own.
func TestDeletingAnEntryDeletesItsPreservedDataWithIt(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	assetID := uploadedCharacterID(t, r, session, assets, aCardCarryingThirdPartyNamespaces)

	before := namespaceBytes(t, r, session, assetID, "character_book")
	page := fetchStartedAsset(t, r, session, assetID)
	lorebook := blockNamed(t, page.Blocks, "lorebook")
	body := editableBlock(lorebook)

	var book struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(body.Elements[0].Content, &book); err != nil {
		t.Fatalf("read the imported book: %v", err)
	}
	if len(book.Entries) != 2 {
		t.Fatalf("the book arrived with %d entries, want both", len(book.Entries))
	}
	kept, err := json.Marshal(map[string]any{"entries": book.Entries[1:]})
	if err != nil {
		t.Fatalf("write the shortened book: %v", err)
	}
	body.Elements[0].Content = kept
	if saved := saveBlock(t, r, session, assetID, lorebook.ID, body); saved.Code != http.StatusOK {
		t.Fatalf("save the shortened book: status = %d: %s", saved.Code, saved.Body.String())
	}

	after := namespaceBytes(t, r, session, assetID, "character_book")
	if after == 0 {
		t.Fatal("deleting one entry took the whole book's preserved data")
	}
	if after >= before {
		t.Errorf("the book preserves %d bytes and preserved %d before the deletion", after, before)
	}
}

func namespaceBytes(
	t *testing.T,
	r http.Handler,
	session *http.Cookie,
	assetID, namespace string,
) int {
	t.Helper()
	for _, row := range preservedNamespaces(t, r, session, assetID) {
		if row.Name == namespace {
			return row.Bytes
		}
	}
	return 0
}

func contains(names []string, wanted string) bool {
	for _, name := range names {
		if name == wanted {
			return true
		}
	}
	return false
}

// smallLimitModule reads a card-shaped payload and holds very little of it, so
// a limit refusal is reachable without a nine-megabyte upload.
type smallLimitModule struct{}

const smallPayloadLimit = 512

func (smallLimitModule) ID() string { return "small_limit" }

func (smallLimitModule) Declaration() format.Declaration {
	declaration := testReaderDeclaration("small_limit", "character")
	declaration.Limits.PayloadBytes = smallPayloadLimit
	declaration.Preservation = format.PreservationDeclaration{
		Body: "card", Container: []string{"extensions"},
	}
	return declaration
}

func (m smallLimitModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func (smallLimitModule) Parse(
	context.Context,
	probe.Inspection,
	format.Claim,
) (format.Parsed, error) {
	return format.Parsed{}, errors.New("an over-limit file never reaches the reader")
}

// A file over the limit is refused, and the refusal says where the weight is
// rather than only that the file is too big. Nothing about the asset is
// stored: preservation never keeps less of a file than arrived in it.
func TestAnOverLimitFileIsRefusedAndNamesWhereTheWeightIs(t *testing.T) {
	registry := format.NewRegistry()
	if err := registry.Register(smallLimitModule{}); err != nil {
		t.Fatalf("register the small-limit module: %v", err)
	}
	r, session, assets := newVerifiedIngestRouter(t, registry)
	metadata := exampleMetadata("Heavy")
	metadata["filename"] = "heavy.json"
	oversized, err := json.Marshal(map[string]any{
		"payload": true,
		"extensions": map[string]any{
			"chub":              map[string]any{"full_path": "ana/quiet"},
			"lumiverse_modules": strings.Repeat("x", smallPayloadLimit),
		},
	})
	if err != nil {
		t.Fatalf("write the oversized file: %v", err)
	}

	finished := uploadAndFinish(t, r, session, assets, metadata, oversized)
	var operation struct {
		Status  string `json:"status"`
		Failure *struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"failure"`
	}
	if err := json.Unmarshal(finished.Body.Bytes(), &operation); err != nil {
		t.Fatalf("decode the refusal: %v", err)
	}
	if operation.Status != "failed" || operation.Failure == nil {
		t.Fatalf("an oversized file finished as %q: %s", operation.Status, finished.Body.String())
	}
	if operation.Failure.Reason != "limit_exceeded" {
		t.Errorf("refusal reason = %q, want limit_exceeded", operation.Failure.Reason)
	}
	if !strings.Contains(operation.Failure.Message, "lumiverse_modules") {
		t.Errorf("refusal = %q, want it to name the namespace holding the weight",
			operation.Failure.Message)
	}
	if !strings.Contains(operation.Failure.Message, strconv.Itoa(smallPayloadLimit)) {
		t.Errorf("refusal = %q, want it to name the limit", operation.Failure.Message)
	}

	listed := send(t, r, authorized(httptest.NewRequest(
		http.MethodGet, "/v1/assets?mine=true", nil,
	), session))
	if strings.Contains(listed.Body.String(), "Heavy") {
		t.Error("a refused file left an asset behind")
	}
}

// The round trip, end to end: a card carrying third-party namespaces is
// uploaded, an unrelated block is edited, and an export in the same format
// brings every preserved key back byte for byte.
// The round trip this whole effort exists for: a card carrying third-party
// namespaces is read, an unrelated block is edited, and every preserved key
// comes back byte-identical.
func TestAnExportInTheSameFormatBringsEveryPreservedKeyBack(t *testing.T) {
	r, session, assets := newCharacterIngestRouter(t)
	metadata := exampleMetadata("Ana")
	metadata["filename"] = "ana.json"
	assetID := assetIDFromIngest(t, uploadAndFinish(
		t, r, session, assets, metadata, []byte(aCardCarryingThirdPartyNamespaces),
	))

	page := fetchStartedAsset(t, r, session, assetID)
	core := editableBlock(blockNamed(t, page.Blocks, "character_core"))
	core.Elements[0].Content = json.RawMessage(`{"text":"Keeps the archive, and the ledger."}`)
	saved := saveBlock(t, r, session, assetID, blockNamed(t, page.Blocks, "character_core").ID, core)
	if saved.Code != http.StatusOK {
		t.Fatalf("save the description: status = %d: %s", saved.Code, saved.Body.String())
	}

	export, err := assets.OpenExport(
		context.Background(), uuid.MustParse(assetID), nil, "chara_card_v3",
	)
	if err != nil {
		t.Fatalf("export the card: %v", err)
	}

	exported := cardBodyOf(t, export.Body)
	source := cardBodyOf(t, []byte(aCardCarryingThirdPartyNamespaces))
	// Every key the reader could not model, back where it came from.
	for _, key := range []string{"tags"} {
		if !bytes.Equal(compactJSON(t, exported[key]), compactJSON(t, source[key])) {
			t.Errorf("%s came back as %s, want %s", key, exported[key], source[key])
		}
	}
	for namespace, value := range namespacesOf(t, source["extensions"]) {
		got := namespacesOf(t, exported["extensions"])[namespace]
		if !bytes.Equal(compactJSON(t, got), compactJSON(t, value)) {
			t.Errorf("%s came back as %s, want %s", namespace, got, value)
		}
	}
	// The book's own keys and each entry's own keys come back too, keyed
	// against the entry Illarin minted an id for.
	book := namespacesOf(t, exported["character_book"])
	if !bytes.Equal(compactJSON(t, book["scan_depth"]), []byte("4")) {
		t.Errorf("the book's own keys did not come back: %s", exported["character_book"])
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(book["entries"], &entries); err != nil {
		t.Fatalf("read the written entries: %v", err)
	}
	if len(entries) == 0 || string(entries[0]["uid"]) == "" {
		t.Errorf("an entry lost the identifier its format gave it: %+v", entries)
	}
}

func namespacesOf(t *testing.T, raw json.RawMessage) map[string]json.RawMessage {
	t.Helper()
	found := make(map[string]json.RawMessage)
	if len(raw) == 0 {
		return found
	}
	if err := json.Unmarshal(raw, &found); err != nil {
		t.Fatalf("read %s as an object: %v", raw, err)
	}
	return found
}

func cardBodyOf(t *testing.T, card []byte) map[string]json.RawMessage {
	t.Helper()
	var read struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(card, &read); err != nil {
		t.Fatalf("read a card: %v", err)
	}
	return read.Data
}
