package preset

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

// SillyTavernID identifies presets with separate prompt and order lists.
const SillyTavernID = "preset_sillytavern"

// Where a SillyTavern preset's leftovers sit.
const (
	sillyTavernNamespace       = SillyTavernID
	sillyTavernPromptNamespace = SillyTavernID + "_prompt"
	sillyTavernScriptNamespace = SillyTavernID + "_script"
)

// sillyTavernPreservation is where this module keeps what a file carried that
// Illarin has no place for.
var sillyTavernPreservation = preservation{
	body: sillyTavernNamespace, extensions: stExtensions,
	reserved: []string{
		sillyTavernNamespace, sillyTavernPromptNamespace, sillyTavernScriptNamespace,
	},
}

// The file's own top-level keys, beyond the flat settings the slot table
// names.
const (
	stPrompts    = "prompts"
	stOrder      = "prompt_order"
	stExtensions = "extensions"
	stScripts    = "regex_scripts"
)

// One prompt's own keys, and one order entry's.
const (
	stIdentifier = "identifier"
	stName       = "name"
	stRole       = "role"
	stText       = "content"
	stMarker     = "marker"
	stPosition   = "injection_position"
	stDepth      = "injection_depth"
	stCharacter  = "character_id"
	stOrderList  = "order"
	stEnabled    = "enabled"
)

// One regex script's own keys.
const (
	stScriptName     = "scriptName"
	stScriptFind     = "findRegex"
	stScriptReplace  = "replaceString"
	stScriptTrim     = "trimStrings"
	stScriptOver     = "placement"
	stScriptDisabled = "disabled"
	stScriptDisplay  = "markdownOnly"
	stScriptPrompt   = "promptOnly"
	stScriptOnEdit   = "runOnEdit"
	stScriptMinDepth = "minDepth"
	stScriptMaxDepth = "maxDepth"
)

// The one order SillyTavern reads. Its prompt manager holds every order under
// a character id and asks for this one, so the others are somebody else's copy
// and travel back out untouched.
const stLiveOrder = 100001

// A prompt sits where the order puts it, unless it says it goes into the
// conversation at a depth instead.
const (
	stInOrder    = 0
	stInHistory  = 1
	stMarkerOnly = true
)

// The text a SillyTavern script runs over. It numbers them, and two of the
// numbers are ones Illarin has no wording for.
var sillyTavernScriptTargets = map[float64]block.ScriptTarget{
	1: block.TargetUserInput,
	2: block.TargetModelOutput,
	3: block.TargetSlashCommand,
	5: block.TargetLorebook,
}

// SillyTavernModule reads and writes a SillyTavern preset.
type SillyTavernModule struct{}

func (SillyTavernModule) ID() string { return SillyTavernID }

func (SillyTavernModule) Declaration() format.Declaration {
	named := slotsByApp[SillyTavern]
	return format.Declaration{
		ID: SillyTavernID, Label: "SillyTavern preset", Kind: Kind,
		Direction: format.Direction{Read: true, Write: true},
		// The prompt list and the order beside it. Both are always there and
		// no other flat SillyTavern file asks for either: its theme is told
		// apart from this by required keys the two do not share.
		Recognition: []format.Recognition{{
			Kind:       format.RecognitionSignature,
			Containers: []probe.Container{probe.JSON},
			Required: map[string]format.ValueType{
				stPrompts: format.ValueArray, stOrder: format.ValueArray,
			},
		}},
		Roles: map[block.Role]format.DirectionalRoleSupport{
			block.RolePromptFragments: {
				Read: format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{
					Grade: format.SupportPartial,
					Condition: &format.ContentCondition{
						Description: "the headings over the fragments, and a fragment " +
							"placed before or after the history, because the file has neither",
						Matches: hasHeadingOrHistoryPlacement,
					},
				},
			},
			// A SillyTavern preset has no form for a reader to fill in.
			block.RolePromptVariables: {
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			},
			block.RoleSamplerSettings:    sillyTavernSettingSupport(named.samplers),
			block.RoleCompletionSettings: sillyTavernSettingSupport(named.completion),
			block.RoleAdvancedSettings:   sillyTavernSettingSupport(named.advanced),
			block.RolePromptNudges: {
				Read: format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{
					Grade:     format.SupportPartial,
					Condition: unnamedNudge(named.nudges),
				},
			},
			block.RoleRegexScripts: {
				Read:  format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{Grade: format.SupportFull},
			},
			block.RoleGallery: {
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			},
			block.RoleCreatorNotes: {
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			},
		},
		// The file carries no name of its own and no description, so there is
		// no header field to fill and the creator names the page.
		Header: nil,
		Slots:  declaredSlots(SillyTavern),
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys: sillyTavernConsumedKeys(named),
		Boilerplate:  nil,
		Preservation: format.PreservationDeclaration{
			Body: sillyTavernNamespace, Container: []string{stExtensions},
		},
		// Its own format and Illarin-authored presets. Neither preset format
		// converts to the other, so the Lumiverse preset is not here
		// (ADR-0020).
		TestedOrigins: []string{SillyTavernID, format.OriginIllarin},
	}
}

// sillyTavernConsumedKeys is the file's own top level: the prompt list, the
// order beside it, and every flat setting this app names.
func sillyTavernConsumedKeys(named namedSlots) []string {
	consumed := []string{stPrompts, stOrder}
	for _, group := range [][]slot{named.samplers, named.completion, named.advanced} {
		for _, slot := range group {
			consumed = append(consumed, slot.name)
		}
	}
	return append(consumed, named.nudges...)
}

func sillyTavernSettingSupport(named []slot) format.DirectionalRoleSupport {
	return format.DirectionalRoleSupport{
		Read:  format.RoleSupport{Grade: format.SupportFull},
		Write: format.RoleSupport{Grade: format.SupportPartial, Condition: unnamedSetting(named)},
	}
}

// hasHeadingOrHistoryPlacement matches a prompt list holding something this
// file cannot say: a heading over some of the fragments, or a fragment placed
// before or after the conversation rather than at a depth inside it.
func hasHeadingOrHistoryPlacement(content block.Content) bool {
	list, isList := content.(block.PromptList)
	if !isList {
		return false
	}
	for _, fragment := range list.Fragments {
		if fragment.GroupID != nil ||
			fragment.Placement == block.BeforeHistory ||
			fragment.Placement == block.AfterHistory {
			return true
		}
	}
	return false
}

func (m SillyTavernModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

// Parse reads a SillyTavern preset and preserves fields it cannot model.
func (m SillyTavernModule) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return format.Parsed{}, fmt.Errorf("%s payload: the claimed payload is missing", SillyTavernID)
	}
	source := maps.Clone(payload.Root)

	var prompts []json.RawMessage
	if raw, present := source[stPrompts]; !present || json.Unmarshal(raw, &prompts) != nil {
		return format.Parsed{}, format.MalformedInput(fmt.Errorf(
			"%s prompts: a preset's prompt fragments have to be a list", SillyTavernID,
		))
	}
	delete(source, stPrompts)

	live, ordered := takeLiveOrder(source)
	list, leftovers := readSillyTavernPrompts(prompts, live, ordered)
	elements := []block.Element{{
		ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
		Content: list,
	}}

	named := slotsByApp[SillyTavern]
	for _, group := range []struct {
		role  block.Role
		slots []slot
	}{
		{block.RoleSamplerSettings, named.samplers},
		{block.RoleCompletionSettings, named.completion},
		{block.RoleAdvancedSettings, named.advanced},
	} {
		if element, filled := settingsElement(group.role, source, group.slots); filled {
			elements = append(elements, element)
		}
	}
	if element, filled := nudgesElement(source, named.nudges); filled {
		elements = append(elements, element)
	}

	scripts, scriptFields := readSillyTavernScripts(source)
	if len(scripts) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeScriptList, Role: block.RoleRegexScripts,
			Content: block.ScriptList{Scripts: scripts},
		})
	}

	return format.Parsed{
		Kind: Kind, Format: SillyTavernID,
		Elements: elements,
		Remainder: sillyTavernPreservation.remainder(
			source, leftovers,
			scriptLeftovers(scripts, sillyTavernScriptNamespace, scriptFields),
		),
	}, nil
}
