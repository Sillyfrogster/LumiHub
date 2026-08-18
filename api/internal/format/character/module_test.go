package character

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

func TestEveryModuleDeclaresTheCharacterKindAndItsExportTargets(t *testing.T) {
	for _, module := range Modules() {
		declaration := module.Declaration()
		if declaration.Kind != Kind {
			t.Errorf("module %q kind = %q, want %q", module.ID(), declaration.Kind, Kind)
		}
		if !declaration.Direction.Read || !declaration.Direction.Write {
			t.Errorf("module %q direction = %+v, want read and write", module.ID(), declaration.Direction)
		}
		if err := format.ValidateDeclaration(declaration); err != nil {
			t.Errorf("module %q declaration: %v", module.ID(), err)
		}
	}
}

func TestCharacterDeclarationsTellTheTruthAboutVersionedRoles(t *testing.T) {
	v2 := CCv2Module{}.Declaration()
	for _, role := range []block.Role{
		block.RoleGroupGreetings, block.RoleGallery, block.RoleExpressions,
	} {
		if support := v2.Roles[role]; support.Read.Grade != format.SupportNone ||
			support.Write.Grade != format.SupportNone {
			t.Errorf("CCv2 %s support = %+v, want none in both directions", role, support)
		}
	}
	v3 := CCv3Module{}.Declaration()
	for _, role := range []block.Role{block.RoleGallery, block.RoleExpressions} {
		if support := v3.Roles[role]; support.Read.Grade != format.SupportNone ||
			support.Write.Grade != format.SupportFull {
			t.Errorf("CCv3 %s support = %+v, want read none and write full", role, support)
		}
	}
	if len(v2.Slots) != 0 || len(v3.Slots) != 0 || len((CharXModule{}).Declaration().Slots) != 0 {
		t.Error("character header bindings were declared as named format slots")
	}
	if slices.Contains(v2.ConsumedKeys, "group_only_greetings") ||
		!slices.Contains(v3.ConsumedKeys, "group_only_greetings") {
		t.Error("declared consumed keys do not match the versioned character readers")
	}
	// A CharX asset list is read for its pictures and never consumed, because
	// it also names a reader's own icon and files that live somewhere else.
	if slices.Contains((CharXModule{}).Declaration().ConsumedKeys, "assets") {
		t.Error("CharX declared the asset list consumed")
	}
}

func TestDeclarationRejectsARoleOutsideTheSharedVocabulary(t *testing.T) {
	declaration := CCv3Module{}.Declaration()
	declaration.Roles[block.Role("invented_role")] = format.DirectionalRoleSupport{
		Read:  format.RoleSupport{Grade: format.SupportFull},
		Write: format.RoleSupport{Grade: format.SupportFull},
	}
	if err := format.ValidateDeclaration(declaration); err == nil ||
		!strings.Contains(err.Error(), "invented_role") {
		t.Fatalf("validation error = %v, want the unknown role", err)
	}
}

func TestCharacterReaderReturnsHeaderFieldsAndRoleTaggedElements(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"name":"Ana","nickname":"Archivist","character_version":"main","creator":"A. Writer",
			"description":"Keeps the quiet archive.","personality":"Patient","scenario":"After closing",
			"first_mes":"Welcome back.","alternate_greetings":["You found me."],
			"group_only_greetings":["All of you made it."],
			"mes_example":"<START>\n{{char}}: Mind the dust.\n{{user}}: I will.",
			"system_prompt":"Stay in character.","post_history_instructions":"Answer softly.",
			"creator_notes":"Made for quiet scenes."
		}
	}`)

	parsed := resolveAndParse(t, file)
	if parsed.Header.Name != "Ana" || parsed.Header.Nickname != "Archivist" ||
		parsed.Header.AssetVersion != "main" || parsed.Header.CreditedAuthor != "A. Writer" {
		t.Fatalf("header = %+v", parsed.Header)
	}
	want := map[block.Role]block.Content{
		block.RoleDescription:             block.Prose{Text: "Keeps the quiet archive."},
		block.RolePersonality:             block.Prose{Text: "Patient"},
		block.RoleScenario:                block.Prose{Text: "After closing"},
		block.RoleSystemPrompt:            block.Prose{Text: "Stay in character."},
		block.RolePostHistoryInstructions: block.Prose{Text: "Answer softly."},
		block.RoleCreatorNotes:            block.Prose{Text: "Made for quiet scenes."},
	}
	for role, content := range want {
		got, ok := elementContent(parsed.Elements, role)
		if !ok {
			t.Errorf("no %s element in %+v", role, parsed.Elements)
			continue
		}
		if !reflect.DeepEqual(got, content) {
			t.Errorf("%s content = %#v, want %#v", role, got, content)
		}
	}
	// A text item also carries an id Illarin minted, which has its own test.
	wantTexts := map[block.Role][]string{
		block.RoleGreetings:      {"Welcome back.", "You found me."},
		block.RoleGroupGreetings: {"All of you made it."},
	}
	for role, texts := range wantTexts {
		got, ok := elementContent(parsed.Elements, role)
		if !ok {
			t.Errorf("no %s element in %+v", role, parsed.Elements)
			continue
		}
		read := make([]string, 0, len(texts))
		for _, item := range got.(block.TextSet).Texts {
			read = append(read, item.Text)
		}
		if !slices.Equal(read, texts) {
			t.Errorf("%s texts = %v, want %v", role, read, texts)
		}
	}
	if _, ok := elementContent(parsed.Elements, block.RoleExampleDialogue); !ok {
		t.Error("no example_dialogue element")
	}
}

func elementContent(elements []block.Element, role block.Role) (block.Content, bool) {
	for _, element := range elements {
		if element.Role == role {
			return element.Content, true
		}
	}
	return nil, false
}

func TestKindComesFromTheModuleForEveryCharacterFormat(t *testing.T) {
	for _, test := range []struct {
		name   string
		file   probe.Inspection
		format string
	}{
		{name: "CCv2 json", file: jsonCard(t, `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`), format: V2},
		{name: "CCv2 png", file: pngCard(t, "chara", `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`), format: V2},
		{name: "CCv3 json", file: jsonCard(t, `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`), format: V3},
		{name: "CCv3 png", file: pngCard(t, "ccv3", `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`), format: V3},
		{name: "CharX", file: charxCard(t, `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`, nil), format: CharX},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed := resolveAndParse(t, test.file)
			if parsed.Format != test.format {
				t.Errorf("format = %q, want %q", parsed.Format, test.format)
			}
			if parsed.Kind != Kind {
				t.Errorf("kind = %q, want %q", parsed.Kind, Kind)
			}
			if parsed.Header.Name != "Ana" {
				t.Errorf("name = %q, want Ana", parsed.Header.Name)
			}
		})
	}
}

func TestAnEmbeddedLorebookIsAFacetAndNotASecondAsset(t *testing.T) {
	withBook := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","character_book":{"name":"Ana's world","entries":[]}}
	}`)
	if got := facetValue(t, resolveAndParse(t, withBook), "has_lorebook"); got != "true" {
		t.Errorf("has_lorebook = %q, want true", got)
	}

	without := jsonCard(t, `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`)
	if got := facetValue(t, resolveAndParse(t, without), "has_lorebook"); got != "false" {
		t.Errorf("has_lorebook = %q, want false", got)
	}
}

func TestEachExtensionNamespaceBecomesItsOwnFacet(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","extensions":{
			"depth_prompt":{"depth":4,"prompt":"stay in character"},
			"lumiverse_modules":{"version":1},
			"talkativeness":"0.5"
		}}
	}`)

	var namespaces []string
	for _, facet := range resolveAndParse(t, file).Facets {
		if facet.Key == "extension" {
			namespaces = append(namespaces, facet.Value)
		}
	}
	want := []string{"depth_prompt", "lumiverse_modules", "talkativeness"}
	if !slices.Equal(namespaces, want) {
		t.Errorf("extension facets = %v, want %v", namespaces, want)
	}
}

func TestACardsDescriptionNeverBecomesTheBlurb(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v2","spec_version":"2.0",
		"data":{
			"name":"Ana",
			"description":"You are Ana. Never break character.",
			"creator_notes":"A quiet archivist. Works best with a slow scene."
		}
	}`)

	parsed := resolveAndParse(t, file)
	if parsed.Blurb != "A quiet archivist. Works best with a slow scene." {
		t.Errorf("blurb = %q, want the creator's notes", parsed.Blurb)
	}
}

func TestAPictureCarryingACardIsExtractedAsTheAvatar(t *testing.T) {
	file := pngCard(t, "chara", `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`)

	parsed := resolveAndParse(t, file)
	if len(parsed.Media) != 1 {
		t.Fatalf("media count = %d, want the picture itself", len(parsed.Media))
	}
	if parsed.Media[0].Role != media.Avatar {
		t.Errorf("media role = %q, want avatar", parsed.Media[0].Role)
	}
}

func TestCharXNamesEachArchivedPictureByWhatTheCardCallsIt(t *testing.T) {
	file := charxCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","assets":[
			{"type":"icon","uri":"embeded://assets/icon/main.png","name":"main","ext":"png"},
			{"type":"icon","uri":"embeded://assets/icon/spare.png","name":"spare","ext":"png"},
			{"type":"emotion","uri":"embeded://assets/emotion/happy.png","name":"happy","ext":"png"},
			{"type":"user_icon","uri":"embeded://assets/user/you.png","name":"you","ext":"png"},
			{"type":"icon","uri":"https://example.invalid/remote.png","name":"remote","ext":"png"}
		]}
	}`, []string{
		"assets/icon/main.png",
		"assets/icon/spare.png",
		"assets/emotion/happy.png",
		"assets/user/you.png",
	})

	parsed := resolveAndParse(t, file)
	want := []media.Role{media.Avatar, media.AvatarAlt, media.Expression}
	if len(parsed.Media) != len(want) {
		t.Fatalf("media = %+v, want %d pictures", parsed.Media, len(want))
	}
	for i, role := range want {
		if parsed.Media[i].Role != role {
			t.Errorf("media %d role = %q, want %q", i, parsed.Media[i].Role, role)
		}
	}
	var cardRemainder []byte
	for _, remainder := range parsed.Remainder {
		if remainder.Namespace == "card" {
			cardRemainder = remainder.Payload
		}
	}
	if !bytes.Contains(cardRemainder, []byte(`"assets"`)) ||
		!bytes.Contains(cardRemainder, []byte(`"user_icon"`)) ||
		!bytes.Contains(cardRemainder, []byte(`"remote"`)) {
		t.Fatalf("CharX asset remainder = %s, want the complete source structure", cardRemainder)
	}
}

func TestCCv2DoesNotConsumeV3OnlyGroupGreetingsOrAssets(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v2","spec_version":"2.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"group_only_greetings":["For the group"],
			"assets":[{"type":"emotion","uri":"remote://happy","name":"happy"}]}
	}`)
	parsed := resolveAndParse(t, file)
	if _, ok := elementContent(parsed.Elements, block.RoleGroupGreetings); ok {
		t.Error("CCv2 materialized a V3-only group greeting")
	}
	if len(parsed.Remainder) != 1 ||
		!bytes.Contains(parsed.Remainder[0].Payload, []byte(`"group_only_greetings"`)) ||
		!bytes.Contains(parsed.Remainder[0].Payload, []byte(`"assets"`)) {
		t.Fatalf("CCv2 remainder = %+v", parsed.Remainder)
	}
}

func TestAVersionPastTheOneWeImplementIsRefusedRatherThanGuessedAt(t *testing.T) {
	later := jsonCard(t, `{"spec":"chara_card_v3","spec_version":"4.0","data":{"name":"Ana"}}`)
	_, err := CCv3Module{}.Parse(context.Background(), later, claimFor(t, CCv3Module{}, later))
	reason, classified := format.FailureOf(err)
	if !classified || reason != format.FailureUnsupportedVersion {
		t.Fatalf("parse error = %v, want an unsupported version", err)
	}

	additive := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.7",
		"data":{"name":"Ana","something_new":{"added":true}}
	}`)
	parsed := resolveAndParse(t, additive)
	if parsed.Header.Name != "Ana" {
		t.Errorf("an additive later minor version was refused: %+v", parsed)
	}
}

func TestARequiredRoleWithTheWrongTypeRefusesTheCardAndNamesThePart(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":17,"first_mes":"Hello"}
	}`)
	_, err := CCv3Module{}.Parse(context.Background(), file, claimFor(t, CCv3Module{}, file))
	if err == nil || !strings.Contains(err.Error(), "chara_card_v3") ||
		!strings.Contains(err.Error(), "description") || !strings.Contains(err.Error(), "string") {
		t.Fatalf("parse error = %v, want the module, required role and reason", err)
	}
}

func TestARecognizedCardWithNoReadableDataNamesTheModuleAndPart(t *testing.T) {
	file := jsonCard(t, `{"spec":"chara_card_v3","spec_version":"3.0","data":17}`)
	_, err := CCv3Module{}.Parse(context.Background(), file, claimFor(t, CCv3Module{}, file))
	if err == nil || !strings.Contains(err.Error(), V3) || !strings.Contains(err.Error(), "data") ||
		!strings.Contains(err.Error(), "object") {
		t.Fatalf("parse error = %v, want module, data and reason", err)
	}
}

func TestAnOptionalValueWithTheWrongTypeDegradesIntoTheRemainder(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"name":"Ana","description":"Quiet","first_mes":"Hello",
			"creator_notes":{"unexpected":true},"future_structure":{"kept":"whole"}
		}
	}`)
	parsed := resolveAndParse(t, file)
	if _, ok := elementContent(parsed.Elements, block.RoleCreatorNotes); ok {
		t.Error("the unreadable creator notes became an element")
	}
	if len(parsed.Remainder) != 1 || parsed.Remainder[0].Namespace != "card" ||
		!bytes.Contains(parsed.Remainder[0].Payload, []byte(`"creator_notes"`)) ||
		!bytes.Contains(parsed.Remainder[0].Payload, []byte(`"future_structure"`)) {
		t.Fatalf("remainder = %+v", parsed.Remainder)
	}
}

func TestBadLorebookValuesCostOnlyThoseValues(t *testing.T) {
	entries := make([]map[string]any, 285)
	for index := range entries {
		entries[index] = map[string]any{
			"name": "Entry", "keys": []string{"key"}, "enabled": true,
			"insertion_order": index, "content": fmt.Sprintf("entry %d", index),
		}
	}
	entries[4]["keys"] = "wrong"
	entries[100]["insertion_order"] = "wrong"
	entries[250]["enabled"] = "wrong"
	body, err := json.Marshal(map[string]any{
		"spec": "chara_card_v3", "spec_version": "3.0",
		"data": map[string]any{
			"description": "Quiet", "first_mes": "Hello",
			"character_book": map[string]any{"entries": entries},
		},
	})
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	parsed := resolveAndParse(t, jsonCard(t, string(body)))
	content, ok := elementContent(parsed.Elements, block.RoleLorebookEntries)
	if !ok {
		t.Fatal("the readable lorebook did not become an element")
	}
	book := content.(block.EntryTable)
	if len(book.Entries) != 285 {
		t.Fatalf("entries = %d, want all 285", len(book.Entries))
	}
	if book.Entries[3].Text != "entry 3" || book.Entries[281].Text != "entry 281" {
		t.Fatal("valid entries around the bad values changed")
	}
	if len(book.Entries[4].Keys) != 0 || book.Entries[100].Order != 0 || !book.Entries[250].Enabled {
		t.Fatalf("bad values were not degraded locally: %+v, %+v, %+v",
			book.Entries[4], book.Entries[100], book.Entries[250])
	}
	// Each unread value stays with the entry it came from, keyed against that
	// entry's id rather than its place in the book.
	byEntry := make(map[uuid.UUID][]byte)
	for _, remainder := range parsed.Remainder {
		if remainder.Owner == format.OwnerItem && remainder.Namespace == "character_book" {
			byEntry[remainder.OwnerID] = remainder.Payload
		}
	}
	if len(byEntry) != 3 {
		t.Fatalf("preserved entries = %d, want the three carrying an unread value", len(byEntry))
	}
	for _, index := range []int{4, 100, 250} {
		payload, ok := byEntry[book.Entries[index].ID]
		if !ok || !bytes.Contains(payload, []byte(`"wrong"`)) {
			t.Errorf("entry %d preserved %s, want its unread value", index, payload)
		}
	}
}

func TestAChunkNameNeverOverridesWhatTheCardSaysItIs(t *testing.T) {
	file := pngCard(t, "ccv3", `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`)
	if parsed := resolveAndParse(t, file); parsed.Format != V2 {
		t.Errorf("format = %q, want %q from the card's own spec", parsed.Format, V2)
	}
}

// CCv3 asks a writer to keep the v2 copy, so nearly every v3 card is this file.
func TestAV3CardCarryingItsV2CopyIsReadAsV3(t *testing.T) {
	file := pngCardChunks(t,
		textChunk{name: "ccv3", body: `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`},
		textChunk{name: "chara", body: `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`},
	)
	if _, ok := (CCv2Module{}).Claim(file); ok {
		t.Error("CCv2 claimed the copy of itself a v3 card carries")
	}
	if parsed := resolveAndParse(t, file); parsed.Format != V3 {
		t.Errorf("format = %q, want %q", parsed.Format, V3)
	}
}

// Standing down for v3 must not take a plain v2 card away from CCv2.
func TestAV2CardWithNoV3CopyIsStillReadAsV2(t *testing.T) {
	file := pngCardChunks(t,
		textChunk{name: "chara", body: `{"spec":"chara_card_v2","spec_version":"2.0","data":{"name":"Ana"}}`},
	)
	if parsed := resolveAndParse(t, file); parsed.Format != V2 {
		t.Errorf("format = %q, want %q", parsed.Format, V2)
	}
}

func TestAV3CardInAnArchiveIsCharXAndNotCCv3(t *testing.T) {
	file := charxCard(t, `{"spec":"chara_card_v3","spec_version":"3.0","data":{"name":"Ana"}}`, nil)
	if _, ok := (CCv3Module{}).Claim(file); ok {
		t.Error("CCv3 claimed a card inside an archive")
	}
	if _, ok := (CharXModule{}).Claim(file); !ok {
		t.Error("CharX did not claim its own archive")
	}
}

// resolveAndParse runs a file through the registry the server builds, so a test
// exercises module selection rather than naming the module itself.
func resolveAndParse(t *testing.T, file probe.Inspection) format.Parsed {
	t.Helper()
	registry := format.NewRegistry()
	for _, module := range Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	resolution, claimed, err := registry.Resolve(file)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !claimed {
		t.Fatal("no module claimed the card")
	}
	parsed, err := resolution.Module.Parse(context.Background(), file, resolution.Claim)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return parsed
}

func claimFor(t *testing.T, module format.Reader, file probe.Inspection) format.Claim {
	t.Helper()
	claim, ok := module.Claim(file)
	if !ok {
		t.Fatalf("module %q did not claim the card", module.ID())
	}
	return claim
}

func facetValue(t *testing.T, parsed format.Parsed, key string) string {
	t.Helper()
	for _, facet := range parsed.Facets {
		if facet.Key == key {
			return facet.Value
		}
	}
	t.Fatalf("no %s facet in %+v", key, parsed.Facets)
	return ""
}

// The helpers below build real containers and inspect them, so the tests read
// what the probe actually produces rather than a hand-made structure.

func jsonCard(t *testing.T, body string) probe.Inspection {
	t.Helper()
	return inspect(t, []byte(body), "card.json")
}

func pngCard(t *testing.T, chunk, body string) probe.Inspection {
	t.Helper()
	return pngCardChunks(t, textChunk{name: chunk, body: body})
}

type textChunk struct{ name, body string }

func pngCardChunks(t *testing.T, chunks ...textChunk) probe.Inspection {
	t.Helper()
	file := testPNG(t)
	// The card sits in a text chunk before IEND, where a real card does.
	end := len(file) - 12
	withCards := slices.Clone(file[:end])
	for _, chunk := range chunks {
		withCards = append(withCards,
			pngChunk("tEXt", slices.Concat([]byte(chunk.name), []byte{0}, []byte(chunk.body)))...)
	}
	return inspect(t, append(withCards, file[end:]...), "card.png")
}

func charxCard(t *testing.T, body string, pictures []string) probe.Inspection {
	t.Helper()
	var file bytes.Buffer
	archive := zip.NewWriter(&file)
	write := func(name string, content []byte) {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create archive entry %q: %v", name, err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("write archive entry %q: %v", name, err)
		}
	}
	write("card.json", []byte(body))
	for _, name := range pictures {
		write(name, testPNG(t))
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return inspect(t, file.Bytes(), "card.charx")
}

type memoryStore struct{ data []byte }

func (s memoryStore) ReadRange(_ context.Context, _ uuid.UUID, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 || offset+length > int64(len(s.data)) {
		return nil, errors.New("range outside the blob")
	}
	return io.NopCloser(bytes.NewReader(s.data[offset : offset+length])), nil
}

func inspect(t *testing.T, data []byte, filename string) probe.Inspection {
	t.Helper()
	file, err := probe.Inspect(
		context.Background(), memoryStore{data: data}, uuid.New(), int64(len(data)), filename,
	)
	if err != nil {
		t.Fatalf("inspect %s: %v", filename, err)
	}
	return file
}
