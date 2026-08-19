package preset

import (
	"encoding/json"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/google/uuid"
)

// Kind is what a preset is to a person.
const Kind = "preset"

// Modules returns every preset module, so the server registers the set rather
// than remembering to add each one.
func Modules() []format.Reader { return []format.Reader{LumiverseModule{}, SillyTavernModule{}} }

// declaredSlots is what a module says about the named settings it fills: every
// slot of its app, in the three groups, plus the nudges it sends on its own.
// It is built from the same table the from-nothing seed is built from, so a
// module and a seeded preset can never disagree about what an app is called.
func declaredSlots(app App) []format.SlotDeclaration {
	named := slotsByApp[app]
	declared := make([]format.SlotDeclaration, 0,
		len(named.samplers)+len(named.completion)+len(named.advanced)+len(named.nudges))
	for _, group := range [][]slot{named.samplers, named.completion, named.advanced} {
		for _, named := range group {
			declared = append(declared, format.SlotDeclaration{
				Name: named.name, Type: slotValueTypes[named.settingType],
			})
		}
	}
	for _, name := range named.nudges {
		declared = append(declared, format.SlotDeclaration{Name: name, Type: format.ValueString})
	}
	return declared
}

// slotValueTypes says what a setting's type looks like in a file.
var slotValueTypes = map[block.SettingType]format.ValueType{
	block.SettingNumber:  format.ValueNumber,
	block.SettingBoolean: format.ValueBoolean,
	block.SettingText:    format.ValueString,
	block.SettingStrings: format.ValueArray,
}

// settingsElement reads the slot names this app knows out of an object of
// named values, and takes them out of the object. What is left is a name this
// module does not read, which travels back out where it came from.
//
// A slot the file wrote as null is a slot nobody filled in. It is still a
// named slot on the page, because the app reads it whether or not anyone has
// put anything in it.
func settingsElement(
	role block.Role,
	values map[string]json.RawMessage,
	named []slot,
) (block.Element, bool) {
	settings := make([]block.Setting, 0, len(named))
	for _, slot := range named {
		raw, present := values[slot.name]
		if !present {
			continue
		}
		value, readable := readValue(raw, slot.settingType)
		if !readable {
			continue
		}
		delete(values, slot.name)
		settings = append(settings, block.Setting{
			ID: block.NewItemID(), Name: slot.name, Type: slot.settingType, Value: value,
		})
	}
	if len(settings) == 0 {
		return block.Element{}, false
	}
	return block.Element{
		ID: uuid.New(), Type: block.TypeSettingGroup, Role: role,
		Content: block.SettingGroup{Settings: settings},
	}, true
}

// nudgesElement reads the short prompts an app sends on its own. They are text
// rather than settings, so they are one list of named bodies.
func nudgesElement(values map[string]json.RawMessage, names []string) (block.Element, bool) {
	texts := make([]block.TextItem, 0, len(names))
	for _, name := range names {
		var text string
		if !keys.Take(values, name, &text) {
			continue
		}
		texts = append(texts, block.TextItem{ID: block.NewItemID(), Name: name, Text: text})
	}
	if len(texts) == 0 {
		return block.Element{}, false
	}
	return block.Element{
		ID: uuid.New(), Type: block.TypeTextSet, Role: block.RolePromptNudges,
		Content: block.TextSet{Texts: texts},
	}, true
}

// readValue reads what somebody put in one named slot, and reports whether the
// key is one this module consumed. A value of the wrong shape for the slot is
// left where it is, so a bad value costs its own key and nothing else.
func readValue(raw json.RawMessage, holds block.SettingType) (*block.Value, bool) {
	if keys.IsNull(raw) {
		return nil, true
	}
	switch holds {
	case block.SettingNumber:
		var number float64
		if json.Unmarshal(raw, &number) != nil {
			return nil, false
		}
		return &block.Value{Number: &number}, true
	case block.SettingBoolean:
		var yes bool
		if json.Unmarshal(raw, &yes) != nil {
			return nil, false
		}
		return &block.Value{Boolean: &yes}, true
	case block.SettingText:
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return nil, false
		}
		return &block.Value{Text: &text}, true
	case block.SettingStrings:
		var strings []string
		if json.Unmarshal(raw, &strings) != nil {
			return nil, false
		}
		return &block.Value{Strings: strings}, true
	}
	return nil, false
}

// writeValue writes one named slot back out. A slot nobody filled in is JSON's
// own word for nothing, which is what tells the app to use its own default
// rather than the zero somebody would read into a 0 or an empty string.
func writeValue(setting block.Setting) json.RawMessage {
	if setting.Value == nil {
		return json.RawMessage("null")
	}
	switch setting.Type {
	case block.SettingNumber:
		if setting.Value.Number != nil {
			return keys.Must(*setting.Value.Number)
		}
	case block.SettingBoolean:
		if setting.Value.Boolean != nil {
			return keys.Must(*setting.Value.Boolean)
		}
	case block.SettingText:
		if setting.Value.Text != nil {
			return keys.Must(*setting.Value.Text)
		}
	case block.SettingStrings:
		return keys.Must(orEmptyStrings(setting.Value.Strings))
	}
	return json.RawMessage("null")
}

func orEmptyStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// settings returns the named settings an asset holds under one role.
func settings(asset format.ExportAsset, role block.Role) []block.Setting {
	content, ok := asset.Content(role)
	if !ok {
		return nil
	}
	group, isGroup := content.(block.SettingGroup)
	if !isGroup {
		return nil
	}
	return group.Settings
}

// fragments returns the prompt list an asset is built on.
func fragments(asset format.ExportAsset) block.PromptList {
	content, ok := asset.Content(block.RolePromptFragments)
	if !ok {
		return block.PromptList{}
	}
	list, isList := content.(block.PromptList)
	if !isList {
		return block.PromptList{}
	}
	return list
}

// variables returns the form a preset asks a reader to fill in.
func variables(asset format.ExportAsset) []block.Variable {
	content, ok := asset.Content(block.RolePromptVariables)
	if !ok {
		return nil
	}
	schema, isSchema := content.(block.VariableSchema)
	if !isSchema {
		return nil
	}
	return schema.Variables
}

// scripts returns the find and replace an asset carries.
func scripts(asset format.ExportAsset) []block.Script {
	content, ok := asset.Content(block.RoleRegexScripts)
	if !ok {
		return nil
	}
	list, isList := content.(block.ScriptList)
	if !isList {
		return nil
	}
	return list.Scripts
}

// nudges returns the short prompts an asset carries.
func nudges(asset format.ExportAsset) []block.TextItem {
	content, ok := asset.Content(block.RolePromptNudges)
	if !ok {
		return nil
	}
	set, isSet := content.(block.TextSet)
	if !isSet {
		return nil
	}
	return set.Texts
}

// unnamedSetting matches a group holding a setting whose name this app does
// not read. A writer puts back the names it declares and no others, so a name
// from somewhere else stays behind rather than reaching a file that would
// ignore it.
func unnamedSetting(named []slot) *format.ContentCondition {
	return &format.ContentCondition{
		Description: "a setting this app has no name for",
		Matches: func(content block.Content) bool {
			group, isGroup := content.(block.SettingGroup)
			if !isGroup {
				return false
			}
			for _, setting := range group.Settings {
				if setting.Value != nil && !slices.ContainsFunc(named, func(s slot) bool {
					return s.name == setting.Name
				}) {
					return true
				}
			}
			return false
		},
	}
}

// unnamedNudge matches a list holding a nudge this app does not send.
func unnamedNudge(names []string) *format.ContentCondition {
	return &format.ContentCondition{
		Description: "a nudge this app does not send",
		Matches: func(content block.Content) bool {
			set, isSet := content.(block.TextSet)
			if !isSet {
				return false
			}
			for _, text := range set.Texts {
				if !slices.Contains(names, text.Name) {
					return true
				}
			}
			return false
		},
	}
}

// hasLooseVariable matches a form holding a variable that belongs to no prompt
// fragment. Both formats that carry variables keep them on the fragment they
// belong to, so a loose one has nowhere to go.
func hasLooseVariable(content block.Content) bool {
	schema, isSchema := content.(block.VariableSchema)
	if !isSchema {
		return false
	}
	for _, variable := range schema.Variables {
		if variable.FragmentID == nil {
			return true
		}
	}
	return false
}
