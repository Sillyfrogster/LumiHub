package theme

import (
	"encoding/json"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
)

func readSetting(raw json.RawMessage, kind block.SettingType) (*block.Value, bool) {
	if string(raw) == "null" {
		return nil, true
	}
	switch kind {
	case block.SettingNumber:
		var value float64
		if json.Unmarshal(raw, &value) == nil {
			return &block.Value{Number: &value}, true
		}
	case block.SettingBoolean:
		var value bool
		if json.Unmarshal(raw, &value) == nil {
			return &block.Value{Boolean: &value}, true
		}
	case block.SettingText:
		var value string
		if json.Unmarshal(raw, &value) == nil {
			return &block.Value{Text: &value}, true
		}
	case block.SettingStrings:
		var value []string
		if json.Unmarshal(raw, &value) == nil {
			return &block.Value{Strings: value}, true
		}
	}
	return nil, false
}

func writeSetting(setting block.Setting) json.RawMessage {
	if setting.Value == nil {
		return json.RawMessage("null")
	}
	switch setting.Type {
	case block.SettingNumber:
		if setting.Value.Number != nil {
			return raw(*setting.Value.Number)
		}
	case block.SettingBoolean:
		if setting.Value.Boolean != nil {
			return raw(*setting.Value.Boolean)
		}
	case block.SettingText:
		if setting.Value.Text != nil {
			return raw(*setting.Value.Text)
		}
	case block.SettingStrings:
		return raw(setting.Value.Strings)
	}
	return json.RawMessage("null")
}

func themeSettings(assetContent block.Content) []block.Setting {
	group, ok := assetContent.(block.SettingGroup)
	if !ok {
		return nil
	}
	return group.Settings
}
