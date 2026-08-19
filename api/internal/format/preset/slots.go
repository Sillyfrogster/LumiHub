// Package preset holds what Illarin knows about the two preset formats.
//
// Today that is the named slots each app reads. A preset created from nothing
// is seeded with them, so the settings a creator fills in have names their app
// understands. The modules that read and write the files sit beside this.
package preset

import (
	"fmt"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/google/uuid"
)

// App is one of the two apps a preset can be built for. It is asked once, when
// a preset is created from nothing, and stored nowhere. It is not identity,
// there is no switcher, and origin_format stays null. A creator who later fills
// in the other app's slots by hand makes that writer's required slots
// non-empty, and the export gates let it through on their own.
type App string

const (
	SillyTavern App = "sillytavern"
	Lumiverse   App = "lumiverse"
)

var appLabels = map[App]string{
	SillyTavern: "SillyTavern",
	Lumiverse:   "Lumiverse",
}

// Apps returns the apps a preset can be built for, in the order they are
// offered.
func Apps() []App { return []App{SillyTavern, Lumiverse} }

// Label returns the app's name as its own users write it.
func (a App) Label() string { return appLabels[a] }

// Known reports whether the app is one Illarin has slot names for.
func (a App) Known() bool {
	_, ok := appLabels[a]
	return ok
}

// slot is one named setting an app reads and the type of what goes in it.
//
// A slot carries no wording of its own and no set of allowed values. Slot names
// are taken at face value and Illarin models nothing about what any of them
// controls, so inventing a label or a list of choices here would be inventing
// knowledge Illarin does not have.
type slot struct {
	name        string
	settingType block.SettingType
}

// namedSlots is one app's settings, in the three groups the kind catalog
// carries, plus the nudges it sends on its own.
type namedSlots struct {
	samplers   []slot
	completion []slot
	advanced   []slot
	nudges     []string
}

// slotsByApp is read from real preset files of each format. Every name here
// appears in one of them, and nothing is here that does not.
var slotsByApp = map[App]namedSlots{
	SillyTavern: {
		samplers: []slot{
			{"temperature", block.SettingNumber},
			{"top_p", block.SettingNumber},
			{"top_k", block.SettingNumber},
			{"top_a", block.SettingNumber},
			{"min_p", block.SettingNumber},
			{"repetition_penalty", block.SettingNumber},
			{"frequency_penalty", block.SettingNumber},
			{"presence_penalty", block.SettingNumber},
			{"openai_max_context", block.SettingNumber},
			{"openai_max_tokens", block.SettingNumber},
		},
		completion: []slot{
			{"stream_openai", block.SettingBoolean},
			{"use_sysprompt", block.SettingBoolean},
			{"squash_system_messages", block.SettingBoolean},
			{"names_behavior", block.SettingNumber},
			{"assistant_prefill", block.SettingText},
			{"assistant_impersonation", block.SettingText},
			{"continue_prefill", block.SettingBoolean},
			{"continue_postfix", block.SettingText},
			{"function_calling", block.SettingBoolean},
			{"enable_web_search", block.SettingBoolean},
			{"media_inlining", block.SettingBoolean},
			{"inline_image_quality", block.SettingText},
			{"show_thoughts", block.SettingBoolean},
			{"reasoning_effort", block.SettingText},
			{"verbosity", block.SettingText},
			{"request_images", block.SettingBoolean},
			{"request_image_aspect_ratio", block.SettingText},
			{"request_image_resolution", block.SettingText},
		},
		advanced: []slot{
			{"seed", block.SettingNumber},
			{"n", block.SettingNumber},
			{"max_context_unlocked", block.SettingBoolean},
			{"bias_preset_selected", block.SettingText},
		},
		nudges: []string{
			"impersonation_prompt",
			"new_chat_prompt",
			"new_group_chat_prompt",
			"new_example_chat_prompt",
			"continue_nudge_prompt",
			"group_nudge_prompt",
			"send_if_empty",
			"wi_format",
			"scenario_format",
			"personality_format",
		},
	},
	Lumiverse: {
		samplers: []slot{
			{"temperature", block.SettingNumber},
			{"topP", block.SettingNumber},
			{"topK", block.SettingNumber},
			{"minP", block.SettingNumber},
			{"maxTokens", block.SettingNumber},
			{"contextSize", block.SettingNumber},
			{"frequencyPenalty", block.SettingNumber},
			{"presencePenalty", block.SettingNumber},
			{"repetitionPenalty", block.SettingNumber},
			{"streaming", block.SettingBoolean},
			{"enabled", block.SettingBoolean},
		},
		completion: []slot{
			{"useSystemPrompt", block.SettingBoolean},
			{"squashSystemMessages", block.SettingBoolean},
			{"namesBehavior", block.SettingNumber},
			{"assistantPrefill", block.SettingText},
			{"assistantImpersonation", block.SettingText},
			{"reasoningPrefill", block.SettingText},
			{"continuePrefill", block.SettingBoolean},
			{"continuePostfix", block.SettingText},
			{"enableFunctionCalling", block.SettingBoolean},
			{"enableWebSearch", block.SettingBoolean},
			{"sendInlineMedia", block.SettingBoolean},
			{"includeUsage", block.SettingBoolean},
		},
		advanced: []slot{
			{"seed", block.SettingNumber},
			{"customStopStrings", block.SettingStrings},
			{"collapseMessages", block.SettingBoolean},
			{"trimIncompleteWords", block.SettingBoolean},
		},
		nudges: []string{
			"newChatPrompt",
			"newGroupChatPrompt",
			"continueNudge",
			"groupNudge",
			"impersonationPrompt",
			"sendIfEmpty",
			"emptySendNudge",
		},
	},
}

// Seed returns the elements a preset built for this app starts with. That is
// its three settings groups with every slot named and none of them filled in,
// and its nudges named the same way.
//
// Nothing here is a value. A slot nobody has filled in stays a slot nobody has
// filled in, which is what a writer leaves out of the file rather than writing
// a zero into.
func Seed(app App) ([]block.Element, error) {
	named, ok := slotsByApp[app]
	if !ok {
		return nil, fmt.Errorf("no slot names for %q", app)
	}
	return []block.Element{
		settingElement(block.RoleSamplerSettings, named.samplers),
		settingElement(block.RoleCompletionSettings, named.completion),
		settingElement(block.RoleAdvancedSettings, named.advanced),
		nudgeElement(named.nudges),
	}, nil
}

func settingElement(role block.Role, slots []slot) block.Element {
	settings := make([]block.Setting, 0, len(slots))
	for _, named := range slots {
		settings = append(settings, block.Setting{
			ID:   block.NewItemID(),
			Name: named.name,
			Type: named.settingType,
		})
	}
	return block.Element{
		ID:      uuid.New(),
		Type:    block.TypeSettingGroup,
		Role:    role,
		Content: block.SettingGroup{Settings: settings},
	}
}

func nudgeElement(names []string) block.Element {
	texts := make([]block.TextItem, 0, len(names))
	for _, name := range names {
		texts = append(texts, block.TextItem{ID: block.NewItemID(), Name: name})
	}
	return block.Element{
		ID:      uuid.New(),
		Type:    block.TypeTextSet,
		Role:    block.RolePromptNudges,
		Content: block.TextSet{Texts: texts},
	}
}
