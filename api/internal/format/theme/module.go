package theme

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

const (
	LumiverseID   = "theme_lumiverse"
	SillyTavernID = "theme_sillytavern"

	lumiverseNamespace          = LumiverseID
	lumiverseComponentNamespace = LumiverseID + "_component"
	lumiverseAssetNamespace     = LumiverseID + "_asset"
	sillyTavernNamespace        = SillyTavernID
)

type LumiverseModule struct{}
type SillyTavernModule struct{}

func Modules() []format.Reader {
	return []format.Reader{LumiverseModule{}, SillyTavernModule{}}
}

func (LumiverseModule) ID() string   { return LumiverseID }
func (SillyTavernModule) ID() string { return SillyTavernID }

func (LumiverseModule) Declaration() format.Declaration {
	return themeDeclaration(
		LumiverseID,
		"Lumiverse theme bundle",
		[]format.Recognition{{
			Kind: format.RecognitionDiscriminator, Containers: []probe.Container{probe.ZIP},
			Path: []string{"format"}, Values: []string{"3"},
		}},
		lumiverseColors,
		lumiverseControls,
		format.RoleSupport{Grade: format.SupportFull},
		[]format.HeaderField{format.HeaderName, format.HeaderBlurb, format.HeaderCreditedAuthor},
	)
}

func (SillyTavernModule) Declaration() format.Declaration {
	declaration := themeDeclaration(
		SillyTavernID,
		"SillyTavern theme",
		[]format.Recognition{{
			Kind: format.RecognitionSignature, Containers: []probe.Container{probe.JSON},
			Required: map[string]format.ValueType{
				"main_text_color": format.ValueString,
				"blur_strength":   format.ValueNumber,
			},
		}},
		sillyTavernColors,
		sillyTavernControls,
		format.RoleSupport{
			Grade: format.SupportPartial,
			Condition: &format.ContentCondition{
				Description: "the main sheet and switched-on component sheets are joined into one stylesheet; switched-off sheets and attached files are not included",
				Matches:     sillyTavernStylesReduced,
			},
			DropWhen: &format.ContentCondition{Matches: sillyTavernStylesDropped},
		},
		[]format.HeaderField{format.HeaderName},
	)
	tokens := declaration.Roles[block.RoleThemeTokens]
	tokens.Write = format.RoleSupport{
		Grade: format.SupportPartial,
		Condition: &format.ContentCondition{
			Description: "only the first colour mode and names this theme format understands are carried",
			Matches:     sillyTavernColorsReduced,
		},
		DropWhen: &format.ContentCondition{Matches: sillyTavernColorsDropped},
	}
	declaration.Roles[block.RoleThemeTokens] = tokens
	return declaration
}

func themeDeclaration(
	id string,
	label string,
	recognition []format.Recognition,
	colors []string,
	controls []namedSlot,
	styles format.RoleSupport,
	header []format.HeaderField,
) format.Declaration {
	return format.Declaration{
		ID: id, Label: label, Kind: Kind,
		Direction:   format.Direction{Read: true, Write: true},
		Recognition: recognition,
		Roles: map[block.Role]format.DirectionalRoleSupport{
			block.RoleThemeTokens: {
				Read: format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{
					Grade: format.SupportPartial,
					Condition: &format.ContentCondition{
						Description: "colour names this theme format does not understand",
						Matches:     hasUnknownColors(colors),
					},
					DropWhen: &format.ContentCondition{Matches: hasNoKnownColors(colors)},
				},
			},
			block.RoleThemeControls: {
				Read: format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{
					Grade: format.SupportPartial,
					Condition: &format.ContentCondition{
						Description: "controls this theme format does not understand",
						Matches:     hasUnknownControls(controls),
					},
				},
			},
			block.RoleStylesheets: {
				Read: format.RoleSupport{Grade: format.SupportFull}, Write: styles,
			},
		},
		Header: header,
		Slots:  append(declaredColorSlots(colors), declaredControlSlots(controls)...),
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys:  themeConsumedKeys(id, colors, controls),
		Boilerplate:   themeBoilerplate(id),
		Preservation:  format.PreservationDeclaration{Body: id},
		TestedOrigins: []string{id, format.OriginIllarin},
	}
}

func themeBoilerplate(id string) []format.Boilerplate {
	if id != LumiverseID {
		return nil
	}
	return []format.Boilerplate{{
		Namespace: LumiverseID,
		Unchosen:  []string{`{"theme":{"tsx":""}}`},
	}}
}

func declaredColorSlots(names []string) []format.SlotDeclaration {
	result := make([]format.SlotDeclaration, 0, len(names))
	for _, name := range names {
		result = append(result, format.SlotDeclaration{Name: name, Type: format.ValueString})
	}
	return result
}

func declaredControlSlots(slots []namedSlot) []format.SlotDeclaration {
	result := make([]format.SlotDeclaration, 0, len(slots))
	for _, slot := range slots {
		kind := format.ValueString
		switch slot.settingType {
		case block.SettingNumber:
			kind = format.ValueNumber
		case block.SettingBoolean:
			kind = format.ValueBoolean
		case block.SettingStrings:
			kind = format.ValueArray
		}
		result = append(result, format.SlotDeclaration{Name: slot.name, Type: kind})
	}
	return result
}

func themeConsumedKeys(id string, colors []string, controls []namedSlot) []string {
	if id == LumiverseID {
		return []string{"format", "name", "author", "description", "theme", "globalCSS", "components", "assets"}
	}
	keys := []string{"name", "custom_css"}
	keys = append(keys, colors...)
	for _, control := range controls {
		keys = append(keys, control.name)
	}
	return keys
}

func hasUnknownColors(known []string) func(block.Content) bool {
	return func(content block.Content) bool {
		set, ok := content.(block.ColorSet)
		if !ok {
			return false
		}
		for _, mode := range set.Modes {
			for _, color := range mode.Colors {
				if color.Value != "" && !slices.Contains(known, color.Name) {
					return true
				}
			}
		}
		return false
	}
}

func hasNoKnownColors(known []string) func(block.Content) bool {
	return func(content block.Content) bool {
		set, ok := content.(block.ColorSet)
		if !ok {
			return false
		}
		for _, mode := range set.Modes {
			for _, color := range mode.Colors {
				if color.Value != "" && slices.Contains(known, color.Name) {
					return false
				}
			}
		}
		return true
	}
}

func sillyTavernColorsReduced(content block.Content) bool {
	set, ok := content.(block.ColorSet)
	if !ok {
		return false
	}
	if hasUnknownColors(sillyTavernColors)(content) {
		return true
	}
	for _, mode := range set.Modes[1:] {
		for _, color := range mode.Colors {
			if color.Value != "" {
				return true
			}
		}
	}
	return false
}

func sillyTavernColorsDropped(content block.Content) bool {
	set, ok := content.(block.ColorSet)
	if !ok || len(set.Modes) == 0 {
		return false
	}
	for _, color := range set.Modes[0].Colors {
		if color.Value != "" && slices.Contains(sillyTavernColors, color.Name) {
			return false
		}
	}
	return true
}

func hasUnknownControls(known []namedSlot) func(block.Content) bool {
	return func(content block.Content) bool {
		group, ok := content.(block.SettingGroup)
		if !ok {
			return false
		}
		for _, setting := range group.Settings {
			if setting.Value != nil && !slices.ContainsFunc(known, func(slot namedSlot) bool {
				return slot.name == setting.Name
			}) {
				return true
			}
		}
		return false
	}
}

func sillyTavernStylesReduced(content block.Content) bool {
	styles, ok := content.(block.StylesheetSet)
	if !ok {
		return false
	}
	if len(styles.Assets) > 0 {
		return styles.Global != "" || hasEnabledStylesheet(styles)
	}
	return len(styles.Stylesheets) > 0 && (styles.Global != "" || hasEnabledStylesheet(styles))
}

func sillyTavernStylesDropped(content block.Content) bool {
	styles, ok := content.(block.StylesheetSet)
	return ok && styles.Global == "" && !hasEnabledStylesheet(styles)
}

func hasEnabledStylesheet(styles block.StylesheetSet) bool {
	return slices.ContainsFunc(styles.Stylesheets, func(sheet block.Stylesheet) bool {
		return sheet.Enabled && sheet.CSS != ""
	})
}

func (m LumiverseModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func (m SillyTavernModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func payloadFor(file probe.Inspection, claim format.Claim, id string) (probe.Payload, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return probe.Payload{}, fmt.Errorf("%s payload: the claimed payload is missing", id)
	}
	return payload, nil
}

func (m LumiverseModule) Parse(ctx context.Context, file probe.Inspection, claim format.Claim) (format.Parsed, error) {
	payload, err := payloadFor(file, claim, m.ID())
	if err != nil {
		return format.Parsed{}, err
	}
	return readLumiverse(ctx, file, payload)
}

func (m SillyTavernModule) Parse(_ context.Context, file probe.Inspection, claim format.Claim) (format.Parsed, error) {
	payload, err := payloadFor(file, claim, m.ID())
	if err != nil {
		return format.Parsed{}, err
	}
	return readSillyTavern(payload)
}

func raw(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("encode theme field: %v", err))
	}
	return encoded
}
