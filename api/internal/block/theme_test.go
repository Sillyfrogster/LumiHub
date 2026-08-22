package block

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func TestAnEmptyThemeHasItsPaletteAndStylesheetBlocks(t *testing.T) {
	blocks, err := Place("theme", nil)
	if err != nil {
		t.Fatalf("place a theme: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("placed %d blocks, want the two the kind requires", len(blocks))
	}

	core := blocks[0]
	if core.Definition != ThemeCore || core.Layout != Duo || core.Width != Full {
		t.Fatalf("theme core = %s using %s at %s, want theme_core using duo at full width",
			core.Definition, core.Layout, core.Width)
	}
	if len(core.Elements) != 2 {
		t.Fatalf("theme core holds %d elements, want colours and controls", len(core.Elements))
	}
	if core.Elements[0].Role != RoleThemeTokens || core.Elements[0].Type != TypeColorSet {
		t.Errorf("first element = %s/%s, want theme tokens in a colour set",
			core.Elements[0].Role, core.Elements[0].Type)
	}
	if core.Elements[1].Role != RoleThemeControls || core.Elements[1].Type != TypeSettingGroup {
		t.Errorf("second element = %s/%s, want theme controls in a setting group",
			core.Elements[1].Role, core.Elements[1].Type)
	}
	if !core.Pinned(RoleThemeTokens, "theme") || !core.Pinned(RoleThemeControls, "theme") {
		t.Error("a theme can lose part of its required core")
	}

	stylesheet := blocks[1]
	if stylesheet.Definition != ThemeStylesheet || stylesheet.Layout != Single || stylesheet.Width != Full {
		t.Fatalf("stylesheet = %s using %s at %s, want stylesheet using single at full width",
			stylesheet.Definition, stylesheet.Layout, stylesheet.Width)
	}
	if len(stylesheet.Elements) != 1 ||
		stylesheet.Elements[0].Role != RoleStylesheets ||
		stylesheet.Elements[0].Type != TypeStylesheetSet {
		t.Fatalf("stylesheet elements = %+v, want one stylesheet set", stylesheet.Elements)
	}
	if !stylesheet.Pinned(RoleStylesheets, "theme") {
		t.Error("a theme can lose its required stylesheet element")
	}

	coreDefinition, _ := ThemeCore.Definition("theme")
	if !coreDefinition.Required || coreDefinition.Hideable ||
		!slices.Equal(coreDefinition.Layouts, []Layout{Duo, Stack2}) {
		t.Errorf("theme core definition = %+v, want required, visible, duo or stack-2", coreDefinition)
	}
	stylesheetDefinition, _ := ThemeStylesheet.Definition("theme")
	if !stylesheetDefinition.Required || !stylesheetDefinition.Hideable {
		t.Errorf("stylesheet definition = %+v, want required and hideable", stylesheetDefinition)
	}
}

func TestAThemeOffersOnlyItsOwnBlocksAndTheSharedLibrary(t *testing.T) {
	definitions, ok := Catalog("theme")
	if !ok {
		t.Fatal("there is no theme catalog")
	}
	wanted := []DefinitionID{
		ThemeCore, ThemeStylesheet, Gallery, Usage, Changelog, Attributes,
		AuthorNotes, RunsBestWith, CustomSection,
	}
	for _, id := range wanted {
		if !slices.ContainsFunc(definitions, func(definition Definition) bool {
			return definition.ID == id
		}) {
			t.Errorf("a theme cannot use %s", id)
		}
	}
	for _, definition := range definitions {
		if definition.ID == "files" {
			t.Error("the theme catalog has a Files block")
		}
	}
}

func TestAThemeNeedsOneWrittenColourBeforeItPublishes(t *testing.T) {
	blocks, err := Place("theme", nil)
	if err != nil {
		t.Fatalf("place a theme: %v", err)
	}
	check := ContentFloor("theme", blocks)
	if len(check) != 1 || check[0].Role != RoleThemeTokens || check[0].Met || check[0].BlockID == nil {
		t.Fatalf("empty theme floor = %+v, want one unmet palette check", check)
	}
	blocks[0].Elements[0].Content = ColorSet{Modes: []ColorMode{{
		Name:   "dark",
		Colors: []Color{{ID: NewItemID(), Name: "background", Value: "#101017"}},
	}}}
	if !ContentFloor("theme", blocks)[0].Met {
		t.Error("a theme with a written colour does not meet its content floor")
	}
}

func TestAStylesheetSetKeepsTheFilesItsCSSUses(t *testing.T) {
	font := []byte("a font fixture")
	raw, err := json.Marshal(StylesheetSet{
		Global: "@font-face { src: url(assets/host.woff2); }",
		Stylesheets: []Stylesheet{{
			ID: NewItemID(), Name: "messages", CSS: ".message {}", Enabled: true,
		}},
		Assets: []StylesheetAsset{{
			ID: NewItemID(), Path: "assets/host.woff2", MediaType: "font/woff2", Data: font,
		}},
	})
	if err != nil {
		t.Fatalf("write stylesheet fixture: %v", err)
	}
	decoded, err := DecodeContent(TypeStylesheetSet, raw)
	if err != nil {
		t.Fatalf("read stylesheet set: %v", err)
	}
	set := decoded.(StylesheetSet)
	if len(set.Assets) != 1 || !bytes.Equal(set.Assets[0].Data, font) {
		t.Errorf("stylesheet assets = %+v, want the font bytes", set.Assets)
	}
}
