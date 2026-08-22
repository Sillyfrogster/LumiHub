// Package theme holds the supported theme formats and their named slots.
package theme

import (
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/google/uuid"
)

const Kind = "theme"

type App string

const (
	SillyTavern App = "sillytavern"
	Lumiverse   App = "lumiverse"
)

var appLabels = map[App]string{
	SillyTavern: "SillyTavern",
	Lumiverse:   "Lumiverse",
}

func Apps() []App { return []App{SillyTavern, Lumiverse} }

func (a App) Label() string { return appLabels[a] }

func (a App) Known() bool {
	_, ok := appLabels[a]
	return ok
}

type namedSlot struct {
	name        string
	settingType block.SettingType
}

var lumiverseColors = []string{
	"primary", "secondary", "background", "text", "danger", "success",
	"warning", "speech", "thoughts",
}

var sillyTavernColors = []string{
	"main_text_color", "italics_text_color", "underline_text_color",
	"quote_text_color", "blur_tint_color", "chat_tint_color",
	"user_mes_blur_tint_color", "bot_mes_blur_tint_color", "shadow_color",
	"border_color",
}

var lumiverseControls = []namedSlot{
	{"accent", block.SettingText},
	{"radiusScale", block.SettingNumber},
	{"enableGlass", block.SettingBoolean},
	{"fontScale", block.SettingNumber},
	{"uiScale", block.SettingNumber},
	{"mode", block.SettingText},
	{"characterAware", block.SettingBoolean},
}

var sillyTavernControls = []namedSlot{
	{"blur_strength", block.SettingNumber},
	{"shadow_width", block.SettingNumber},
	{"font_scale", block.SettingNumber},
	{"avatar_style", block.SettingNumber},
	{"chat_display", block.SettingNumber},
	{"toastr_position", block.SettingText},
	{"chat_width", block.SettingNumber},
	{"fast_ui_mode", block.SettingBoolean},
	{"waifuMode", block.SettingBoolean},
	{"noShadows", block.SettingBoolean},
	{"timer_enabled", block.SettingBoolean},
	{"timestamps_enabled", block.SettingBoolean},
	{"timestamp_model_icon", block.SettingBoolean},
	{"mesIDDisplay_enabled", block.SettingBoolean},
	{"hideChatAvatars_enabled", block.SettingBoolean},
	{"message_token_count_enabled", block.SettingBoolean},
	{"expand_message_actions", block.SettingBoolean},
	{"enableZenSliders", block.SettingBoolean},
	{"enableLabMode", block.SettingBoolean},
	{"hotswap_enabled", block.SettingBoolean},
	{"bogus_folders", block.SettingBoolean},
	{"zoomed_avatar_magnification", block.SettingBoolean},
	{"reduced_motion", block.SettingBoolean},
	{"compact_input_area", block.SettingBoolean},
}

func Seed(app App) ([]block.Element, error) {
	switch app {
	case Lumiverse:
		return []block.Element{
			colorElement("dark", lumiverseColors),
			controlElement(lumiverseControls),
		}, nil
	case SillyTavern:
		return []block.Element{
			colorElement("", sillyTavernColors),
			controlElement(sillyTavernControls),
		}, nil
	default:
		return nil, fmt.Errorf("no theme slot names for %q", app)
	}
}

func colorElement(mode string, names []string) block.Element {
	colors := make([]block.Color, 0, len(names))
	for _, name := range names {
		colors = append(colors, block.Color{ID: block.NewItemID(), Name: name})
	}
	return block.Element{
		ID: uuid.New(), Type: block.TypeColorSet, Role: block.RoleThemeTokens,
		Content: block.ColorSet{Modes: []block.ColorMode{{Name: mode, Colors: colors}}},
	}
}

func controlElement(slots []namedSlot) block.Element {
	settings := make([]block.Setting, 0, len(slots))
	for _, slot := range slots {
		settings = append(settings, block.Setting{
			ID: block.NewItemID(), Name: slot.name, Type: slot.settingType,
		})
	}
	return block.Element{
		ID: uuid.New(), Type: block.TypeSettingGroup, Role: block.RoleThemeControls,
		Content: block.SettingGroup{Settings: settings},
	}
}
