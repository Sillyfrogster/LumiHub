package v1

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	characterformat "github.com/Sillyfrogster/Illarin/api/internal/format/character"
	lorebookformat "github.com/Sillyfrogster/Illarin/api/internal/format/lorebook"
	packformat "github.com/Sillyfrogster/Illarin/api/internal/format/pack"
	presetformat "github.com/Sillyfrogster/Illarin/api/internal/format/preset"
	themeformat "github.com/Sillyfrogster/Illarin/api/internal/format/theme"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

func TestV1IsAnInternalReadOnlyModule(t *testing.T) {
	declaration := (Module{}).Declaration()
	if declaration.ID != ID || !declaration.Direction.Read || declaration.Direction.Write {
		t.Fatalf("declaration = %+v, want an internal reader", declaration)
	}
	if declaration.Input != format.InputDatabaseRow || len(declaration.Recognition) != 0 {
		t.Errorf("input = %q and recognition = %v, want rows with no file recognition",
			declaration.Input, declaration.Recognition)
	}
	if !slices.Equal(declaration.Kinds, []string{
		CharacterKind, LorebookKind, PresetKind, ThemeKind, PackKind,
	}) {
		t.Errorf("kinds = %v, want all five asset kinds", declaration.Kinds)
	}
	if _, writes := any(Module{}).(format.Writer); writes {
		t.Fatal("v1 implements a writer")
	}

	registry := format.NewRegistry()
	if err := registry.Register(Module{}); err != nil {
		t.Fatalf("register v1: %v", err)
	}
	if err := registry.ValidateDeclarations(); err != nil {
		t.Fatalf("validate v1: %v", err)
	}
	if slices.Contains(registry.ReadableLabels(), "v1") {
		t.Errorf("readable labels = %v, want no v1 upload format", registry.ReadableLabels())
	}
	file := probe.Inspection{Container: probe.JSON, Payloads: []probe.Payload{{
		ID: 1, Locator: probe.Locator{Container: probe.JSON},
		Root: map[string]json.RawMessage{"name": json.RawMessage(`"legacy"`)},
	}}}
	if _, claimed, err := registry.Resolve(file); err != nil || claimed {
		t.Fatalf("resolve = claimed %v, error %v; v1 must not recognise a file", claimed, err)
	}
}

func TestEveryOfferedWriterSerializesV1Content(t *testing.T) {
	registry := format.NewRegistry()
	modules := []format.Module{Module{}}
	for _, reader := range characterformat.Modules() {
		modules = append(modules, reader)
	}
	for _, reader := range lorebookformat.Modules() {
		modules = append(modules, reader)
	}
	for _, reader := range presetformat.Modules() {
		modules = append(modules, reader)
	}
	for _, reader := range themeformat.Modules() {
		modules = append(modules, reader)
	}
	for _, reader := range packformat.Modules() {
		modules = append(modules, reader)
	}
	for _, module := range modules {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	if err := registry.ValidateDeclarations(); err != nil {
		t.Fatalf("validate declarations: %v", err)
	}

	subjects := []struct {
		kind       string
		elements   []block.Element
		wantTarget []string
	}{
		{
			kind: CharacterKind,
			elements: []block.Element{
				prose(block.RoleDescription, "An archivist."),
				{ID: uuid.New(), Type: block.TypeTextSet, Role: block.RoleGreetings,
					Content: block.TextSet{Texts: []block.TextItem{{ID: block.NewItemID(), Text: "Welcome."}}}},
			},
			wantTarget: []string{characterformat.V2, characterformat.V3, characterformat.CharX},
		},
		{
			kind: LorebookKind,
			elements: []block.Element{{
				ID: uuid.New(), Type: block.TypeEntryTable, Role: block.RoleLorebookEntries,
				Content: block.EntryTable{Entries: []block.Entry{{
					ID: block.NewItemID(), Keys: []string{"archive"}, Text: "A sealed room.", Enabled: true,
				}}},
			}},
			wantTarget: []string{lorebookformat.ID},
		},
		{
			kind: PresetKind,
			elements: []block.Element{{
				ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
				Content: block.PromptList{Fragments: []block.PromptFragment{{
					ID: block.NewItemID(), Name: "Rules", Text: "Write plainly.",
					Role: block.PromptSystem, Enabled: true, Placement: block.BeforeHistory,
				}}},
			}},
			wantTarget: []string{presetformat.LumiverseID},
		},
		{
			kind: ThemeKind,
			elements: []block.Element{
				{ID: uuid.New(), Type: block.TypeColorSet, Role: block.RoleThemeTokens,
					Content: block.ColorSet{Modes: []block.ColorMode{{Name: "dark", Colors: []block.Color{{
						ID: block.NewItemID(), Name: "primary", Value: "#dde4ef",
					}}}}}},
				{ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
					Content: block.StylesheetSet{Global: "body { color: white; }"}},
			},
			wantTarget: []string{themeformat.LumiverseID},
		},
		{
			kind: PackKind,
			elements: []block.Element{{
				ID: uuid.New(), Type: block.TypeRecordList, Role: block.RolePackItems,
				Content: block.RecordList{Schema: block.LumiaRecordSchema, Records: []block.LumiaRecord{{
					ID: block.NewItemID(), LumiaName: "Aster", LumiaDefinition: "A courier",
					LumiaPersonality: "Restless", LumiaBehavior: "Takes the long road",
					GenderIdentity: 2, AuthorName: "Road Scribe", Version: 1,
				}}},
			}},
			wantTarget: []string{packformat.ID},
		},
	}
	for _, subject := range subjects {
		t.Run(subject.kind, func(t *testing.T) {
			targets := registry.OfferedTargets(format.CapabilitySubject{
				Kind: subject.kind, Origin: ID, Elements: subject.elements,
			})
			ids := make([]string, len(targets))
			for i, target := range targets {
				ids[i] = target.Format
			}
			if !reflect.DeepEqual(ids, subject.wantTarget) {
				t.Fatalf("targets = %v, want %v", ids, subject.wantTarget)
			}
			for _, target := range targets {
				module, _ := registry.ByID(target.Format)
				writer := module.(format.Writer)
				artifact, err := writer.Write(context.Background(), format.ExportAsset{
					Kind: subject.kind, Header: format.Header{Name: "Fixture"}, Elements: subject.elements,
				})
				if err != nil {
					t.Fatalf("write %s: %v", target.Format, err)
				}
				if len(artifact.Body) == 0 {
					t.Errorf("%s wrote an empty artifact", target.Format)
				}
			}
		})
	}
}

func TestCharacterImagesAndRemaindersAreReadOnce(t *testing.T) {
	result, err := (Module{}).Read(context.Background(), CharacterRow{
		Common: CommonRow{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000102"),
			OwnerID: uuid.MustParse("00000000-0000-0000-0000-000000000202"),
			Name:    "Mara", Description: "An archivist.", CreatedAt: time.Now(),
		},
		FirstMessage:       "Welcome.",
		AlternateGreetings: json.RawMessage(`[]`),
		CharacterBook: json.RawMessage(`{
			"name":"Archive notes",
			"scan_depth":4,
			"entries":[{
				"name":"The sealed wing",
				"keys":["sealed wing"],
				"content":"It opens at midnight.",
				"enabled":true,
				"insertion_order":20,
				"uid":"entry-one"
			}]
		}`),
		Extensions: json.RawMessage(`{
			"lumihub_art_display":"avatar",
			"landing_perspective_layers":["layer"],
			"_lumiverse_install_slug":"mara",
			"_lumiverse_install_source":"catalog",
			"_lumiverse_library_scope":"owner",
			"foreign_tool":{"kept":true},
			"lumiverse_modules":{
				"world_books":[{"duplicate":true}],
				"expressions":[{"duplicate":true}],
				"regex_scripts":[{"find":"x"}]
			}
		}`),
		Assets: json.RawMessage(`[{
			"type":"icon","name":"Main portrait","uri":"embeded://gallery/main.png"
		}]`),
		Images: []CharacterImageRow{
			imageRow("00000000-0000-0000-0000-000000000301", "avatar", "", 0),
			imageRow("00000000-0000-0000-0000-000000000302", "expression", "thinking", 2),
			imageRow("00000000-0000-0000-0000-000000000303", "gallery", "", 0),
			imageRow("00000000-0000-0000-0000-000000000304", "avatar_alt", "profile", 1),
			imageRow("00000000-0000-0000-0000-000000000305", "perspective_layer", "", 3),
		},
	})
	if err != nil {
		t.Fatalf("read character row: %v", err)
	}
	if result.Cover == nil || result.Cover.SourceID != uuid.MustParse("00000000-0000-0000-0000-000000000301") {
		t.Fatalf("cover = %+v, want the avatar image", result.Cover)
	}
	if len(result.Media) != 4 {
		t.Fatalf("content media = %d, want every non-cover image", len(result.Media))
	}
	expressions := contentFor(t, result.Parsed.Elements, block.RoleExpressions).(block.ImageSet)
	if len(expressions.Images) != 1 || expressions.Images[0].Name != "thinking" {
		t.Errorf("expressions = %+v, want the named expression", expressions.Images)
	}
	gallery := contentFor(t, result.Parsed.Elements, block.RoleGallery).(block.ImageSet)
	if len(gallery.Images) != 3 || gallery.Images[0].Name != "Main portrait" ||
		gallery.Images[1].Name != "profile" || gallery.Images[2].Name != "" {
		t.Errorf("gallery = %+v, want recovered, alternate and perspective images in order", gallery.Images)
	}

	book := contentFor(t, result.Parsed.Elements, block.RoleLorebookEntries).(block.EntryTable)
	if len(book.Entries) != 1 || book.Entries[0].Text != "It opens at midnight." {
		t.Fatalf("lorebook = %+v, want one editable entry", book.Entries)
	}
	if preservedNamespace(result.Parsed.Remainder, "foreign_tool") == nil {
		t.Error("the foreign namespace was not preserved")
	}
	modules := preservedObject(t, result.Parsed.Remainder, "lumiverse_modules")
	if _, held := modules["world_books"]; held {
		t.Error("the duplicate lorebook was preserved")
	}
	if _, held := modules["expressions"]; held {
		t.Error("the duplicate expression list was preserved")
	}
	if _, held := modules["regex_scripts"]; !held {
		t.Error("the unread regex scripts were dropped")
	}
	for _, namespace := range liftedNamespaces {
		if preservedNamespace(result.Parsed.Remainder, namespace) != nil {
			t.Errorf("LumiHub namespace %q survived", namespace)
		}
	}
	entryFields := preservedNamespace(result.Parsed.Remainder, characterBookNamespace)
	if entryFields == nil {
		t.Error("the lorebook entry's unread uid was dropped")
	}
}

func TestALorebookRowBecomesAnEntryTable(t *testing.T) {
	created := time.Date(2025, time.February, 3, 4, 5, 6, 0, time.UTC)
	result, err := (Module{}).Read(context.Background(), LorebookRow{
		Common: CommonRow{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000111"),
			OwnerID: uuid.MustParse("00000000-0000-0000-0000-000000000211"),
			Name:    "City guide", Description: "Places worth remembering.",
			Tags: []string{"Locations"}, CreatedAt: created,
		},
		Creator: "Mapmaker",
		Entries: json.RawMessage(`[
			{"name":"North gate","keys":["gate"],"content":"Closed at dusk.","enabled":true,"insertion_order":10,"uid":7},
			{"keys":["market"],"content":"Open every day.","enabled":true,"insertion_order":20}
		]`),
	})
	if err != nil {
		t.Fatalf("read lorebook row: %v", err)
	}
	if result.Parsed.Kind != LorebookKind || result.Parsed.Header.Name != "City guide" ||
		result.Parsed.Header.Blurb != "Places worth remembering." ||
		result.Parsed.Header.CreditedAuthor != "Mapmaker" {
		t.Errorf("parsed lorebook = %+v, want the row header", result.Parsed)
	}
	table := contentFor(t, result.Parsed.Elements, block.RoleLorebookEntries).(block.EntryTable)
	if len(table.Entries) != 2 || table.Entries[0].Name != "North gate" ||
		table.Entries[1].Text != "Open every day." {
		t.Errorf("entries = %+v, want both row entries", table.Entries)
	}
	if preservedNamespace(result.Parsed.Remainder, lorebookEntryNamespace) == nil {
		t.Error("the first entry's unread uid was dropped")
	}
	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil || len(blocks) != 1 || blocks[0].Definition != block.LorebookCore {
		t.Fatalf("blocks = %v, error %v; want the lorebook catalog", definitionIDs(blocks), err)
	}
}

func TestAPresetRowUsesTheRowHeaderAndKeepsVersionsAndSealedBlocks(t *testing.T) {
	assetID := uuid.MustParse("00000000-0000-0000-0000-000000000112")
	ownerID := uuid.MustParse("00000000-0000-0000-0000-000000000212")
	longDescription := strings.Repeat("Read the setup carefully. ", 22)
	result, err := (Module{}).Read(context.Background(), PresetRow{
		Common: CommonRow{
			ID: assetID, OwnerID: ownerID, Name: "Row name", Description: longDescription,
			Tags: []string{"Roleplay"}, CreatedAt: time.Now(),
		},
		LatestVersion: "7",
		Payload: json.RawMessage(`{
			"schemaVersion":1,
			"name":"Stale payload name",
			"description":"Stale payload description",
			"blocks":[{
				"id":"fragment-1","name":"Rules","role":"system",
				"content":"Write plainly.","enabled":true,"position":"pre_history"
			}]
		}`),
		Versions: []PresetVersionRow{
			{ID: 1, Version: "6", Changelog: "Added the plain-language rule.", Snapshot: json.RawMessage(`{"version":6}`)},
			{ID: 2, Version: "7", Snapshot: json.RawMessage(`{"version":7}`)},
		},
		SealedBlocks: []SealedBlockRow{{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000401"),
			Version: pointerTo("7"), Key: "private-rule", Content: "Do not publish this.",
			SHA256: strings.Repeat("a", 64), CreatedBy: &ownerID,
		}},
	})
	if err != nil {
		t.Fatalf("read preset row: %v", err)
	}
	if result.Parsed.Kind != PresetKind || result.Parsed.Header.Name != "Row name" ||
		result.Parsed.Header.Blurb != "" || result.Parsed.Header.AssetVersion != "7" {
		t.Errorf("header = %+v, want row values and an empty oversized blurb", result.Parsed.Header)
	}
	fragments := contentFor(t, result.Parsed.Elements, block.RolePromptFragments).(block.PromptList)
	if len(fragments.Fragments) != 1 || fragments.Fragments[0].Text != "Write plainly." {
		t.Errorf("fragments = %+v, want the payload fragment", fragments.Fragments)
	}
	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil {
		t.Fatalf("place preset: %v", err)
	}
	if !reflect.DeepEqual(definitionIDs(blocks), []block.DefinitionID{
		block.PresetCore, block.Usage, block.Changelog,
	}) {
		t.Errorf("blocks = %v, want core, usage and changelog in catalog order", definitionIDs(blocks))
	}
	usage := blocks[1].Elements[0].Content.(block.Prose)
	if usage.Text != longDescription {
		t.Errorf("usage = %q, want the complete description", usage.Text)
	}
	changes := blocks[2].Elements[0].Content.(block.TextSet)
	if len(changes.Texts) != 1 || changes.Texts[0].Name != "6" ||
		changes.Texts[0].Text != "Added the plain-language rule." {
		t.Errorf("changelog = %+v, want the creator-written version entry", changes.Texts)
	}
	if len(result.PreservedRecords) != 2 {
		t.Fatalf("version snapshots = %d, want both", len(result.PreservedRecords))
	}
	for _, record := range result.PreservedRecords {
		if record.AssetID != assetID || record.Table != "preset_versions" {
			t.Errorf("preserved record = %+v, want an asset-bound version snapshot", record)
		}
	}
	var preservedVersion map[string]json.RawMessage
	if json.Unmarshal(result.PreservedRecords[0].Payload, &preservedVersion) != nil ||
		preservedVersion["snapshot"] == nil || preservedVersion["blocks_added"] == nil ||
		preservedVersion["created_at"] == nil {
		t.Errorf("preserved version = %s, want the complete source row shape", result.PreservedRecords[0].Payload)
	}
	if len(result.SealedBlocks) != 1 || result.SealedBlocks[0].AssetID != assetID ||
		result.SealedBlocks[0].Content != "Do not publish this." {
		t.Errorf("sealed blocks = %+v, want the whole asset-bound row", result.SealedBlocks)
	}
}

func TestAThemeRowRecoversOnlyFontsFromItsGeneratedBundle(t *testing.T) {
	result, err := (Module{}).Read(context.Background(), ThemeRow{
		Common: CommonRow{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000113"),
			OwnerID: uuid.MustParse("00000000-0000-0000-0000-000000000213"),
			Name:    "Night paper", Description: "Ink on a dark page.", CreatedAt: time.Now(),
		},
		Config: json.RawMessage(`{
			"id":"night-paper",
			"accent":{"hue":210,"chroma":0.2},
			"baseColorsByMode":{"dark":{"primary":"#dde4ef","background":"#111827"}},
			"statusColors":{"danger":"#ef4444"},
			"mode":"dark","radiusScale":0.8,"enableGlass":true
		}`),
		CustomCSS: `@font-face { font-family: "Archive"; src: url("fonts/archive.woff2"); }`,
		Bundle: themeBundle(t, map[string][]byte{
			"fonts/archive.woff2": {0x77, 0x4f, 0x46, 0x32},
			"images/paper.png":    {0x89, 0x50, 0x4e, 0x47},
		}),
	})
	if err != nil {
		t.Fatalf("read theme row: %v", err)
	}
	if result.Parsed.Kind != ThemeKind || result.Parsed.Header.Name != "Night paper" ||
		result.Parsed.Header.Blurb != "Ink on a dark page." {
		t.Errorf("header = %+v, want the row values", result.Parsed.Header)
	}
	colors := contentFor(t, result.Parsed.Elements, block.RoleThemeTokens).(block.ColorSet)
	if colors.Empty() {
		t.Error("the row palette came out empty")
	}
	controls := contentFor(t, result.Parsed.Elements, block.RoleThemeControls).(block.SettingGroup)
	if controls.Empty() {
		t.Error("the row controls came out empty")
	}
	styles := contentFor(t, result.Parsed.Elements, block.RoleStylesheets).(block.StylesheetSet)
	if styles.Global != `@font-face { font-family: "Archive"; src: url("fonts/archive.woff2"); }` {
		t.Errorf("global stylesheet = %q, want the row CSS", styles.Global)
	}
	if len(styles.Assets) != 1 || styles.Assets[0].Path != "fonts/archive.woff2" ||
		styles.Assets[0].MediaType != "font/woff2" {
		t.Errorf("stylesheet assets = %+v, want only the font", styles.Assets)
	}
	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil || !reflect.DeepEqual(definitionIDs(blocks), []block.DefinitionID{
		block.ThemeCore, block.ThemeStylesheet,
	}) {
		t.Fatalf("blocks = %v, error %v; want the theme catalog", definitionIDs(blocks), err)
	}
}

func TestAPackRowBecomesLumiaRecordsWithoutFetchingItsImages(t *testing.T) {
	result, err := (Module{}).Read(context.Background(), PackRow{
		Common: CommonRow{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000114"),
			OwnerID: uuid.MustParse("00000000-0000-0000-0000-000000000214"),
			Name:    "Travelers", Description: "Two travelers for a campaign.", CreatedAt: time.Now(),
		},
		Author: "Road Scribe", Version: 3, CoverURL: "https://example.invalid/cover.webp",
		LumiaItems: json.RawMessage(`[
			{
				"lumiaName":"Aster","lumiaDefinition":"A courier",
				"lumiaPersonality":"Restless","lumiaBehavior":"Takes the long road",
				"avatarUrl":"https://example.invalid/aster.webp","genderIdentity":2,
				"authorName":"Road Scribe","version":1
			},
			{
				"lumiaName":"Bram","lumiaDefinition":"A guide",
				"lumiaPersonality":"Patient","lumiaBehavior":"Checks the map",
				"avatarUrl":"","genderIdentity":1,
				"authorName":"Road Scribe","version":2
			}
		]`),
	})
	if err != nil {
		t.Fatalf("read pack row: %v", err)
	}
	if result.Parsed.Kind != PackKind || result.Parsed.Header.Name != "Travelers" ||
		result.Parsed.Header.Blurb != "Two travelers for a campaign." ||
		result.Parsed.Header.CreditedAuthor != "Road Scribe" ||
		result.Parsed.Header.AssetVersion != "3" {
		t.Errorf("header = %+v, want the pack row", result.Parsed.Header)
	}
	records := contentFor(t, result.Parsed.Elements, block.RolePackItems).(block.RecordList)
	if len(records.Records) != 2 || records.Records[0].LumiaName != "Aster" ||
		records.Records[1].Version != 2 {
		t.Errorf("records = %+v, want both Lumia items", records.Records)
	}
	if len(result.ExternalMedia) != 2 || result.ExternalMedia[0].Owner != ExternalCover ||
		result.ExternalMedia[1].Owner != ExternalPackItem {
		t.Errorf("external media = %+v, want the cover and one item avatar", result.ExternalMedia)
	}
	if len(result.Parsed.Tags) != 0 {
		t.Errorf("tags = %v, want none because v1 packs had no tags column", result.Parsed.Tags)
	}
	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil || len(blocks) != 1 || blocks[0].Definition != block.PackCore {
		t.Fatalf("blocks = %v, error %v; want the pack catalog", definitionIDs(blocks), err)
	}
}

func themeBundle(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var body bytes.Buffer
	archive := zip.NewWriter(&body)
	for name, data := range files {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create theme entry: %v", err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write theme entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close theme bundle: %v", err)
	}
	return body.Bytes()
}

func imageRow(id, imageType, label string, position int) CharacterImageRow {
	return CharacterImageRow{
		ID: uuid.MustParse(id), Type: imageType, Label: label,
		Path: "/archive/image.webp", MediaType: "image/webp", ByteSize: 120, Position: position,
	}
}

func pointerTo[T any](value T) *T { return &value }

func preservedNamespace(rows []format.Remainder, namespace string) []byte {
	for _, row := range rows {
		if row.Namespace == namespace {
			return row.Payload
		}
	}
	return nil
}

func preservedObject(t *testing.T, rows []format.Remainder, namespace string) map[string]json.RawMessage {
	t.Helper()
	payload := preservedNamespace(rows, namespace)
	if payload == nil {
		t.Fatalf("no preserved %q namespace", namespace)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		t.Fatalf("decode preserved %q namespace: %v", namespace, err)
	}
	return object
}

func TestACharacterRowBecomesHeaderFieldsAndSemanticRoles(t *testing.T) {
	created := time.Date(2025, time.January, 4, 5, 6, 7, 0, time.UTC)
	assetID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	ownerID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	result, err := (Module{}).Read(context.Background(), CharacterRow{
		Common: CommonRow{
			ID: assetID, OwnerID: ownerID, Name: "Mara", Description: "A patient archivist.",
			Downloads: 12, Views: 34, Favorites: 5,
			Tags: []string{"Science Fiction", "science fiction"}, IsNSFW: true,
			CreatedAt: created, UpdatedAt: created.Add(24 * time.Hour),
		},
		Nickname: "Mars", Personality: "Methodical", Scenario: "A sealed library",
		FirstMessage: "Mind the dust.", AlternateGreetings: json.RawMessage(`["You came back."]`),
		ExampleDialogue: "<START>\nMara: The index is wrong.\nYou: Show me.",
		Creator:         "Paper Lantern", CreatorNotes: "Built for slow mysteries.",
		CharacterVersion: "main", SystemPrompt: "Keep the clues consistent.",
		PostHistoryInstructions: "Do not reveal the culprit early.", Tagline: "A mystery archivist.",
	})
	if err != nil {
		t.Fatalf("read character row: %v", err)
	}

	if result.AssetID != assetID || result.OwnerID != ownerID {
		t.Fatalf("identity = %s/%s, want the row ids", result.AssetID, result.OwnerID)
	}
	if result.OriginFormat != ID {
		t.Errorf("origin = %q, want %q", result.OriginFormat, ID)
	}
	if !result.CreatedAt.Equal(created) || !result.ContentUpdatedAt.Equal(created) {
		t.Errorf("times = %v/%v, want the row creation time", result.CreatedAt, result.ContentUpdatedAt)
	}
	if result.ContentGeneration != 1 || result.Legacy.Downloads != 12 ||
		result.Legacy.Views != 34 || result.Legacy.Favorites != 5 ||
		!result.Legacy.UpdatedAt.Equal(created.Add(24*time.Hour)) {
		t.Errorf("legacy = %+v at generation %d, want the frozen row counters", result.Legacy, result.ContentGeneration)
	}
	if result.Parsed.Format != ID || result.Parsed.Kind != CharacterKind {
		t.Fatalf("parsed = %q/%q, want %q/%q", result.Parsed.Format, result.Parsed.Kind, ID, CharacterKind)
	}
	if result.Parsed.Header.Name != "Mara" || result.Parsed.Header.Blurb != "A mystery archivist." ||
		result.Parsed.Header.AssetVersion != "main" ||
		result.Parsed.Header.CreditedAuthor != "Paper Lantern" ||
		result.Parsed.Header.Nickname != "Mars" {
		t.Errorf("header = %+v, want the row's identity fields", result.Parsed.Header)
	}
	if !reflect.DeepEqual(result.Parsed.Tags, []string{"Science Fiction", "science fiction"}) {
		t.Errorf("tags = %v, want their original spelling and order", result.Parsed.Tags)
	}
	if result.Parsed.IsNSFW == nil || !*result.Parsed.IsNSFW {
		t.Errorf("NSFW answer = %v, want yes", result.Parsed.IsNSFW)
	}

	wantProse := map[block.Role]string{
		block.RoleDescription: "A patient archivist.", block.RolePersonality: "Methodical",
		block.RoleScenario: "A sealed library", block.RoleSystemPrompt: "Keep the clues consistent.",
		block.RolePostHistoryInstructions: "Do not reveal the culprit early.",
		block.RoleCreatorNotes:            "Built for slow mysteries.",
	}
	for role, want := range wantProse {
		content := contentFor(t, result.Parsed.Elements, role)
		prose, ok := content.(block.Prose)
		if !ok || prose.Text != want {
			t.Errorf("%s = %#v, want prose %q", role, content, want)
		}
	}
	greetings, ok := contentFor(t, result.Parsed.Elements, block.RoleGreetings).(block.TextSet)
	if !ok || len(greetings.Texts) != 2 || greetings.Texts[0].Text != "Mind the dust." ||
		greetings.Texts[1].Text != "You came back." {
		t.Errorf("greetings = %#v, want the first and alternate messages", greetings)
	}
	dialogue, ok := contentFor(t, result.Parsed.Elements, block.RoleExampleDialogue).(block.DialogueSample)
	if !ok || len(dialogue.Turns) != 2 || dialogue.Turns[0].Speaker != "Mara" ||
		dialogue.Turns[1].Text != "Show me." {
		t.Errorf("example dialogue = %#v, want two speaker-tagged turns", dialogue)
	}

	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil {
		t.Fatalf("place the character: %v", err)
	}
	if len(blocks) != 4 || blocks[0].Definition != block.CharacterCore ||
		blocks[1].Definition != block.Messages || blocks[2].Definition != block.ModelInstructions ||
		blocks[3].Definition != block.AuthorNotes {
		t.Errorf("blocks = %v, want the catalog's character order", definitionIDs(blocks))
	}
}

func TestOnlyAnExplicitlyVerifiedMissingGreetingIsRecovered(t *testing.T) {
	row := CharacterRow{
		Common: CommonRow{
			ID:      uuid.MustParse("00000000-0000-0000-0000-000000000115"),
			OwnerID: uuid.MustParse("00000000-0000-0000-0000-000000000215"),
			Name:    "Mara", Description: "An archivist.", CreatedAt: time.Now(),
		},
		FirstMessage: "Welcome.", AlternateGreetings: json.RawMessage(`[]`),
	}
	reader := Module{Recoveries: RecoveryAllowlist{
		row.Common.ID: {AlternateGreeting: "The recovered greeting."},
	}}
	result, err := reader.Read(context.Background(), row)
	if err != nil {
		t.Fatalf("read verified recovery: %v", err)
	}
	greetings := contentFor(t, result.Parsed.Elements, block.RoleGreetings).(block.TextSet)
	if len(greetings.Texts) != 2 || greetings.Texts[1].Text != "The recovered greeting." {
		t.Errorf("greetings = %+v, want the verified recovery", greetings.Texts)
	}
	if !slices.Equal(result.Events, []Event{{Kind: RecoveredAlternateGreeting}}) {
		t.Errorf("events = %+v, want the recovery recorded", result.Events)
	}

	row.AlternateGreetings = json.RawMessage(`["A row greeting."]`)
	if _, err := reader.Read(context.Background(), row); err == nil {
		t.Fatal("a file recovery overwrote non-empty row content")
	}
}

func contentFor(t *testing.T, elements []block.Element, role block.Role) block.Content {
	t.Helper()
	for _, element := range elements {
		if element.Role == role {
			return element.Content
		}
	}
	t.Fatalf("no %s element", role)
	return nil
}

func definitionIDs(blocks []block.Block) []block.DefinitionID {
	ids := make([]block.DefinitionID, len(blocks))
	for i, item := range blocks {
		ids[i] = item.Definition
	}
	return ids
}
