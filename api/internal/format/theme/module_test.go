package theme

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/preset"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

const sillyTavernTheme = `{
	"name":"Midnight violet",
	"main_text_color":"rgba(244,241,246,1)",
	"italics_text_color":"rgba(214,203,236,1)",
	"underline_text_color":"rgba(169,139,234,1)",
	"quote_text_color":"rgba(185,179,192,1)",
	"blur_tint_color":"rgba(13,12,17,.9)",
	"chat_tint_color":"rgba(21,20,27,.94)",
	"user_mes_blur_tint_color":"rgba(35,30,45,.92)",
	"bot_mes_blur_tint_color":"rgba(25,23,32,.92)",
	"shadow_color":"rgba(0,0,0,.4)",
	"border_color":"rgba(55,52,64,1)",
	"blur_strength":8,
	"shadow_width":2,
	"font_scale":1,
	"avatar_style":1,
	"chat_display":1,
	"toastr_position":"toast-top-right",
	"chat_width":56,
	"fast_ui_mode":false,
	"waifuMode":false,
	"noShadows":false,
	"timer_enabled":true,
	"timestamps_enabled":true,
	"timestamp_model_icon":false,
	"mesIDDisplay_enabled":true,
	"hideChatAvatars_enabled":false,
	"message_token_count_enabled":false,
	"expand_message_actions":true,
	"enableZenSliders":false,
	"enableLabMode":false,
	"hotswap_enabled":true,
	"bogus_folders":false,
	"zoomed_avatar_magnification":false,
	"reduced_motion":false,
	"compact_input_area":false,
	"custom_css":"body { letter-spacing: .01em; }",
	"future_setting":"kept"
}`

const lumiverseTheme = `{
	"format":3,
	"name":"Violet archive",
	"author":"A creator",
	"description":"A graphite theme with a quiet violet accent.",
	"createdAt":"2026-01-15T14:30:00Z",
	"bundleId":"violet-archive",
	"theme":{
		"id":"violet-archive",
		"name":"Violet archive",
		"mode":"dark",
		"accent":{"h":262,"s":64,"l":62},
		"statusColors":{"online":"#72c49a"},
		"baseColorsByMode":{"dark":{
			"primary":"#a98bea",
			"secondary":"#776391",
			"background":"#0d0c11",
			"text":"#f4f1f6",
			"danger":"#ef8da8",
			"success":"#72c49a",
			"warning":"#e3b86a",
			"speech":"#f4f1f6",
			"thoughts":"#b9b3c0"
		}},
		"radiusScale":1,
		"enableGlass":false,
		"characterAware":true
	},
	"globalCSS":"@font-face { font-family: Archive; src: url(assets/archive.woff2); }",
	"components":{
		"messages":{"css":".message { padding: 1rem; }","enabled":true,"future":"kept"},
		"composer":{"css":".composer { opacity: .8; }","enabled":false}
	},
	"assets":[{
		"slug":"archive-font",
		"originalFilename":"Archive.woff2",
		"mimeType":"font/woff2",
		"tags":["font"],
		"metadata":{"family":"Archive"},
		"archivePath":"assets/archive.woff2"
	}]
}`

func TestBothThemeModulesDeclareTheirPublicContract(t *testing.T) {
	for _, module := range Modules() {
		declaration := module.Declaration()
		if declaration.Kind != Kind || declaration.ID != module.ID() {
			t.Errorf("declaration identity = %s/%s, want %s/%s",
				declaration.Kind, declaration.ID, Kind, module.ID())
		}
		if !declaration.Direction.Read || !declaration.Direction.Write {
			t.Errorf("%s direction = %+v, want read and write", module.ID(), declaration.Direction)
		}
		if _, ok := module.(format.Writer); !ok {
			t.Errorf("%s declares a writer it does not have", module.ID())
		}
		if err := format.ValidateDeclaration(declaration); err != nil {
			t.Errorf("%s declaration: %v", module.ID(), err)
		}
		for _, role := range []block.Role{
			block.RoleThemeTokens, block.RoleThemeControls, block.RoleStylesheets,
		} {
			if _, declared := declaration.Roles[role]; !declared {
				t.Errorf("%s does not declare %s", module.ID(), role)
			}
		}
	}

	registry := testRegistry(t)
	for _, module := range preset.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("theme signature overlaps %s: %v", module.ID(), err)
		}
	}
}

func TestSillyTavernThemeReadsAndWritesItsFlatVocabulary(t *testing.T) {
	parsed := parse(t, inspect(t, []byte(sillyTavernTheme), "theme.json"))
	if parsed.Header.Name != "Midnight violet" {
		t.Errorf("name = %q, want the theme name", parsed.Header.Name)
	}
	palette := elementFor(t, parsed.Elements, block.RoleThemeTokens).(block.ColorSet)
	if len(palette.Modes) != 1 || len(palette.Modes[0].Colors) != 10 {
		t.Fatalf("palette = %+v, want the ten flat colours", palette)
	}
	controls := elementFor(t, parsed.Elements, block.RoleThemeControls).(block.SettingGroup)
	if len(sillyTavernControls) != 24 || controls.Supplied() != len(sillyTavernControls) {
		t.Errorf("supplied controls = %d of %d, want all 24", controls.Supplied(), len(sillyTavernControls))
	}
	styles := elementFor(t, parsed.Elements, block.RoleStylesheets).(block.StylesheetSet)
	if styles.Global != "body { letter-spacing: .01em; }" || len(styles.Stylesheets) != 0 {
		t.Errorf("stylesheets = %+v, want the one custom CSS string", styles)
	}

	written := write(t, SillyTavernModule{}, parsed)
	var document map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &document); err != nil {
		t.Fatalf("decode written theme: %v", err)
	}
	if string(document["future_setting"]) != `"kept"` ||
		string(document["main_text_color"]) != `"rgba(244,241,246,1)"` {
		t.Errorf("written theme did not restore known and preserved fields: %s", written.Body)
	}
}

func TestSillyTavernReportsAndAvoidsFlatteningExtraColourModes(t *testing.T) {
	registry := testRegistry(t)
	palette := block.ColorSet{Modes: []block.ColorMode{
		{Name: "dark", Colors: []block.Color{{
			ID: block.NewItemID(), Name: "main_text_color", Value: "#111111",
		}}},
		{Name: "light", Colors: []block.Color{{
			ID: block.NewItemID(), Name: "main_text_color", Value: "#eeeeee",
		}}},
	}}
	elements := []block.Element{{
		ID: uuid.New(), Type: block.TypeColorSet, Role: block.RoleThemeTokens, Content: palette,
	}}
	targets := registry.OfferedTargets(format.CapabilitySubject{Kind: Kind, Elements: elements})
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want the SillyTavern target", targets)
	}
	loss, ok := roleLoss(targets[0], block.RoleThemeTokens)
	if !ok || loss.Verdict != format.Reduced || !strings.Contains(loss.Reason, "first") {
		t.Errorf("palette loss = %+v, want reduced with the first mode named", loss)
	}

	written := write(t, SillyTavernModule{}, format.Parsed{Elements: elements})
	var document map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &document); err != nil {
		t.Fatalf("decode written theme: %v", err)
	}
	if string(document["main_text_color"]) != `"#111111"` {
		t.Errorf("written main colour = %s, want the first mode without flattening", document["main_text_color"])
	}
}

func TestLumiverseEmptyTSXIsDeclaredAsDisplayBoilerplate(t *testing.T) {
	declaration := (LumiverseModule{}).Declaration()
	if !declaration.RecordsNothing(LumiverseID, []byte(`{"theme":{"tsx":""}}`)) {
		t.Error("an empty tsx stamp would appear in the creator's preserved-data panel")
	}
	if declaration.RecordsNothing(LumiverseID, []byte(`{"theme":{"tsx":"return <Theme />"}}`)) {
		t.Error("a tsx value with content was hidden as boilerplate")
	}
	if declaration.RecordsNothing(LumiverseID, []byte(`{"future":"kept","theme":{"tsx":""}}`)) {
		t.Error("meaningful data beside an empty tsx stamp was hidden as boilerplate")
	}
}

func TestLumiverseThemeKeepsItsHeaderPaletteComponentsAndFont(t *testing.T) {
	bundle := themeBundle(t, lumiverseTheme, map[string][]byte{
		"assets/archive.woff2": []byte("font fixture"),
	})
	parsed := parse(t, inspect(t, bundle, "violet.lumitheme"))
	if parsed.Header.Name != "Violet archive" ||
		parsed.Header.Blurb != "A graphite theme with a quiet violet accent." ||
		parsed.Header.CreditedAuthor != "A creator" {
		t.Errorf("header = %+v, want name, blurb and credited author", parsed.Header)
	}
	palette := elementFor(t, parsed.Elements, block.RoleThemeTokens).(block.ColorSet)
	if len(palette.Modes) != 1 || palette.Modes[0].Name != "dark" || len(palette.Modes[0].Colors) != 9 {
		t.Fatalf("palette = %+v, want the nine dark-mode colours", palette)
	}
	styles := elementFor(t, parsed.Elements, block.RoleStylesheets).(block.StylesheetSet)
	if len(styles.Stylesheets) != 2 || len(styles.Assets) != 1 ||
		!bytes.Equal(styles.Assets[0].Data, []byte("font fixture")) {
		t.Fatalf("stylesheets = %+v, want two components and the font", styles)
	}
	encodedStyles, err := json.Marshal(styles)
	if err != nil || bytes.Contains(encodedStyles, []byte("null")) {
		t.Errorf("stylesheets JSON = %s, want arrays at the API boundary", encodedStyles)
	}

	written := write(t, LumiverseModule{}, parsed)
	entries := archiveEntries(t, written.Body)
	if !bytes.Equal(entries["assets/archive.woff2"], []byte("font fixture")) {
		t.Errorf("written font = %q, want the source bytes", entries["assets/archive.woff2"])
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(entries["theme.json"], &document); err != nil {
		t.Fatalf("decode written theme.json: %v", err)
	}
	if string(document["description"]) != `"A graphite theme with a quiet violet accent."` ||
		!bytes.Contains(document["theme"], []byte(`"statusColors"`)) ||
		!bytes.Contains(document["components"], []byte(`"future":"kept"`)) {
		t.Errorf("written bundle lost bound or preserved fields: %s", entries["theme.json"])
	}
}

func TestSillyTavernStylesheetLossMatchesWhatItCanWrite(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	seeded, err := Seed(SillyTavern)
	if err != nil {
		t.Fatalf("seed theme: %v", err)
	}
	palette := seeded[0].Content.(block.ColorSet)
	palette.Modes[0].Colors[0].Value = "rgba(244,241,246,1)"
	seeded[0].Content = palette

	tests := []struct {
		name       string
		styles     block.StylesheetSet
		verdict    format.Verdict
		reasonPart string
	}{
		{
			name:    "one stylesheet",
			styles:  block.StylesheetSet{Global: "body {}"},
			verdict: format.Carried,
		},
		{
			name: "enabled component",
			styles: block.StylesheetSet{Global: "body {}", Stylesheets: []block.Stylesheet{{
				ID: block.NewItemID(), Name: "messages", CSS: ".message {}", Enabled: true,
			}}},
			verdict: format.Reduced, reasonPart: "joined",
		},
		{
			name: "disabled component alone",
			styles: block.StylesheetSet{Stylesheets: []block.Stylesheet{{
				ID: block.NewItemID(), Name: "messages", CSS: ".message {}", Enabled: false,
			}}},
			verdict: format.Dropped,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			elements := append(slices.Clone(seeded), block.Element{
				ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
				Content: test.styles,
			})
			targets := registry.OfferedTargets(format.CapabilitySubject{
				Kind: Kind, Elements: elements,
			})
			if len(targets) != 1 || targets[0].Format != SillyTavernID {
				t.Fatalf("targets = %+v, want only the SillyTavern theme", targets)
			}
			loss, ok := roleLoss(targets[0], block.RoleStylesheets)
			if !ok || loss.Verdict != test.verdict ||
				(test.reasonPart != "" && !strings.Contains(loss.Reason, test.reasonPart)) {
				t.Errorf("stylesheet loss = %+v, want %s containing %q",
					loss, test.verdict, test.reasonPart)
			}
		})
	}
}

func TestSillyTavernJoinsOnlyEnabledComponentStylesheetsAfterTheMainSheet(t *testing.T) {
	written := write(t, SillyTavernModule{}, format.Parsed{
		Header: format.Header{Name: "Violet archive"},
		Elements: []block.Element{{
			ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
			Content: block.StylesheetSet{
				Global: "body {}",
				Stylesheets: []block.Stylesheet{
					{ID: block.NewItemID(), Name: "messages", CSS: ".message {}", Enabled: true},
					{ID: block.NewItemID(), Name: "composer", CSS: ".composer {}", Enabled: false},
				},
			},
		}},
	})
	var document map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &document); err != nil {
		t.Fatalf("decode written theme: %v", err)
	}
	var css string
	if err := json.Unmarshal(document["custom_css"], &css); err != nil {
		t.Fatalf("decode custom CSS: %v", err)
	}
	if !strings.HasPrefix(css, "/* Main stylesheet */\nbody {}") ||
		!strings.Contains(css, "/* Component: messages */\n.message {}") ||
		strings.Contains(css, ".composer {}") {
		t.Errorf("joined CSS = %q, want named main and enabled component sources only", css)
	}
}

func TestLumiverseMarkerOutsideTheDeclaredSetIsNotClaimed(t *testing.T) {
	file := inspect(t, themeBundle(t, strings.Replace(lumiverseTheme, `"format":3`, `"format":4`, 1), nil), "future.lumitheme")
	if _, claimed := (LumiverseModule{}).Claim(file); claimed {
		t.Error("format marker 4 was claimed as though it were 3")
	}
}

func roleLoss(target format.Target, role block.Role) (format.RoleLoss, bool) {
	for _, loss := range target.Roles {
		if loss.Role == role {
			return loss, true
		}
	}
	return format.RoleLoss{}, false
}

func elementFor(t *testing.T, elements []block.Element, role block.Role) block.Content {
	t.Helper()
	for _, element := range elements {
		if element.Role == role {
			return element.Content
		}
	}
	t.Fatalf("no %s element in %+v", role, elements)
	return nil
}

func parse(t *testing.T, file probe.Inspection) format.Parsed {
	t.Helper()
	resolution, claimed, err := testRegistry(t).Resolve(file)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !claimed {
		t.Fatal("no module claimed the theme")
	}
	parsed, err := resolution.Module.Parse(context.Background(), file, resolution.Claim)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return parsed
}

func write(t *testing.T, module format.Reader, parsed format.Parsed) format.Artifact {
	t.Helper()
	writer := module.(format.Writer)
	written, err := writer.Write(context.Background(), format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements, Preserved: parsed.Remainder,
	})
	if err != nil {
		t.Fatalf("write %s: %v", module.ID(), err)
	}
	return written
}

func testRegistry(t *testing.T) *format.Registry {
	t.Helper()
	registry := format.NewRegistry()
	for _, module := range Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %s: %v", module.ID(), err)
		}
	}
	return registry
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

func themeBundle(t *testing.T, document string, assets map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	writeArchiveEntry(t, archive, "theme.json", []byte(document))
	for name, data := range assets {
		writeArchiveEntry(t, archive, name, data)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close theme bundle: %v", err)
	}
	return output.Bytes()
}

func writeArchiveEntry(t *testing.T, archive *zip.Writer, name string, data []byte) {
	t.Helper()
	entry, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func archiveEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open written archive: %v", err)
	}
	entries := make(map[string][]byte, len(archive.File))
	for _, entry := range archive.File {
		opened, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		entries[entry.Name], err = io.ReadAll(opened)
		opened.Close()
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name, err)
		}
	}
	return entries
}
