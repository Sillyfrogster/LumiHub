package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

func readSillyTavern(payload probe.Payload) (format.Parsed, error) {
	source := maps.Clone(payload.Root)
	header := format.Header{}
	keys.Take(source, "name", &header.Name)

	colors := make([]block.Color, 0, len(sillyTavernColors))
	for _, name := range sillyTavernColors {
		var value string
		if keys.Take(source, name, &value) {
			colors = append(colors, block.Color{ID: block.NewItemID(), Name: name, Value: value})
		}
	}
	elements := []block.Element{{
		ID: uuid.New(), Type: block.TypeColorSet, Role: block.RoleThemeTokens,
		Content: block.ColorSet{Modes: []block.ColorMode{{Colors: colors}}},
	}}

	settings := make([]block.Setting, 0, len(sillyTavernControls))
	for _, slot := range sillyTavernControls {
		value, present := source[slot.name]
		if !present {
			continue
		}
		read, ok := readSetting(value, slot.settingType)
		if !ok {
			continue
		}
		delete(source, slot.name)
		settings = append(settings, block.Setting{
			ID: block.NewItemID(), Name: slot.name, Type: slot.settingType, Value: read,
		})
	}
	if len(settings) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeSettingGroup, Role: block.RoleThemeControls,
			Content: block.SettingGroup{Settings: settings},
		})
	}
	var css string
	if keys.Take(source, "custom_css", &css) {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
			Content: block.StylesheetSet{Global: css},
		})
	}

	return format.Parsed{
		Kind: Kind, Format: SillyTavernID, Header: header, Elements: elements,
		Remainder: themeRemainder(sillyTavernNamespace, source),
	}, nil
}

func (SillyTavernModule) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	body := keepTheme(asset.Preserved).body(sillyTavernNamespace)
	body["name"] = raw(asset.Header.Name)

	if content, ok := asset.Content(block.RoleThemeTokens); ok {
		if palette, isPalette := content.(block.ColorSet); isPalette {
			for _, mode := range palette.Modes {
				for _, color := range mode.Colors {
					if color.Value != "" && slices.Contains(sillyTavernColors, color.Name) {
						body[color.Name] = raw(color.Value)
					}
				}
			}
		}
	}
	if content, ok := asset.Content(block.RoleThemeControls); ok {
		for _, setting := range themeSettings(content) {
			if setting.Value != nil && slices.ContainsFunc(sillyTavernControls, func(slot namedSlot) bool {
				return slot.name == setting.Name
			}) {
				body[setting.Name] = writeSetting(setting)
			}
		}
	}
	if content, ok := asset.Content(block.RoleStylesheets); ok {
		if styles, isStyles := content.(block.StylesheetSet); isStyles {
			if css := sillyTavernCSS(styles); css != "" {
				body["custom_css"] = raw(css)
			}
		}
	}

	document, err := json.Marshal(body)
	if err != nil {
		return format.Artifact{}, fmt.Errorf("write the SillyTavern theme: %w", err)
	}
	return format.Artifact{Body: document, MediaType: "application/json", Extension: ".json"}, nil
}

func sillyTavernCSS(styles block.StylesheetSet) string {
	enabled := make([]block.Stylesheet, 0, len(styles.Stylesheets))
	for _, sheet := range styles.Stylesheets {
		if sheet.Enabled && sheet.CSS != "" {
			enabled = append(enabled, sheet)
		}
	}
	if len(enabled) == 0 {
		return styles.Global
	}
	parts := make([]string, 0, len(enabled)+1)
	if styles.Global != "" {
		parts = append(parts, "/* Main stylesheet */\n"+styles.Global)
	}
	for _, sheet := range enabled {
		name := strings.TrimSpace(strings.ReplaceAll(sheet.Name, "*/", "* /"))
		if name == "" {
			name = "Component stylesheet"
		}
		parts = append(parts, fmt.Sprintf("/* Component: %s */\n%s", name, sheet.CSS))
	}
	return strings.Join(parts, "\n\n")
}
