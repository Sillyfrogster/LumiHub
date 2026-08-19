package preset

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

// SillyTavernModule reads and writes the preset SillyTavern exports.
//
// The file names itself nowhere, so it is recognised by the two keys it always
// has. Its settings are flat at the top level, its prompts are a list, and the
// order they are sent in is a second structure keyed by character.
const SillyTavernID = "preset_sillytavern"

// Where a SillyTavern preset's leftovers sit.
const (
	sillyTavernNamespace       = SillyTavernID
	sillyTavernPromptNamespace = SillyTavernID + "_prompt"
	sillyTavernScriptNamespace = SillyTavernID + "_script"
)

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

// Parse reads a SillyTavern preset into the roles a preset has.
//
// The prompt list is the required part. If it will not parse the import is
// refused and nothing is stored; past that point a failure costs only what
// failed.
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
		Remainder: sillyTavernRemainder(
			source, leftovers, sillyTavernScriptLeftovers(scripts, scriptFields),
		),
	}, nil
}

// sent is one line of the order SillyTavern reads: which prompt, and whether
// it is switched on. The switch lives here rather than on the prompt, which is
// where SillyTavern reads it from.
type sent struct {
	identifier string
	enabled    bool
}

// takeLiveOrder takes the one order SillyTavern reads out of the list of them.
// The orders belonging to other characters stay where they are and travel back
// out untouched.
func takeLiveOrder(source map[string]json.RawMessage) ([]sent, bool) {
	var orders []map[string]json.RawMessage
	if !keys.Take(source, stOrder, &orders) {
		return nil, false
	}
	var live []sent
	found := false
	remaining := make([]map[string]json.RawMessage, 0, len(orders))
	for _, entry := range orders {
		var character int
		var listed []map[string]json.RawMessage
		named := keys.Take(entry, stCharacter, &character)
		if found || !named || character != stLiveOrder || !keys.Take(entry, stOrderList, &listed) {
			if named {
				entry[stCharacter] = keys.Must(character)
			}
			remaining = append(remaining, entry)
			continue
		}
		found = true
		for _, item := range listed {
			line := sent{}
			if !keys.Take(item, stIdentifier, &line.identifier) {
				continue
			}
			keys.Take(item, stEnabled, &line.enabled)
			live = append(live, line)
		}
	}
	if len(remaining) > 0 {
		source[stOrder], _ = json.Marshal(remaining)
	}
	return live, found
}

// readSillyTavernPrompts reads the prompt list into the order the file sends
// it in, which is the order structure rather than the list's own. A prompt the
// order does not name is one the file never sends, so it goes at the end,
// switched off, where a creator can still see and edit it.
func readSillyTavernPrompts(
	prompts []json.RawMessage,
	live []sent,
	ordered bool,
) (block.PromptList, map[uuid.UUID]itemLeftover) {
	read := make([]block.PromptFragment, 0, len(prompts))
	named := make(map[string]int, len(prompts))
	leftovers := make(map[uuid.UUID]itemLeftover, len(prompts))

	for _, raw := range prompts {
		fields := keys.Object(raw)
		if len(fields) == 0 {
			continue
		}
		var identifier string
		keys.Take(fields, stIdentifier, &identifier)

		fragment := block.PromptFragment{ID: block.NewItemID(), Enabled: !ordered}
		keys.Take(fields, stName, &fragment.Name)
		keys.Take(fields, stRole, &fragment.Role)
		keys.Take(fields, stText, &fragment.Text)
		if !fragment.Role.Known() {
			fields[stRole] = keys.Must(fragment.Role)
			fragment.Role = ""
		}
		// A marker is a placeholder standing where the app splices in the
		// character, the lorebook, the persona or the chat. The file says only
		// that a prompt is one, so the identifier names what it holds a place
		// for.
		var marker bool
		if keys.Take(fields, stMarker, &marker) && marker {
			fragment.Marker = identifier
		}
		var position, depth int
		if keys.Take(fields, stPosition, &position) {
			if position == stInHistory {
				fragment.Placement = block.InHistory
				if keys.Take(fields, stDepth, &depth) {
					fragment.Depth = &depth
				}
			} else if position != stInOrder {
				fields[stPosition] = keys.Must(position)
			}
		}
		if identifier != "" {
			fields[stIdentifier] = keys.Must(identifier)
			if _, taken := named[identifier]; !taken {
				named[identifier] = len(read)
			}
		}
		read = append(read, fragment)
		if len(fields) > 0 {
			leftovers[fragment.ID] = itemLeftover{
				namespace: sillyTavernPromptNamespace, fields: fields,
			}
		}
	}

	list := block.PromptList{Fragments: make([]block.PromptFragment, 0, len(read))}
	placed := make([]bool, len(read))
	for _, line := range live {
		at, known := named[line.identifier]
		if !known || placed[at] {
			continue
		}
		placed[at] = true
		read[at].Enabled = line.enabled
		list.Fragments = append(list.Fragments, read[at])
	}
	for at, fragment := range read {
		if !placed[at] {
			list.Fragments = append(list.Fragments, fragment)
		}
	}
	return list, leftovers
}

// readSillyTavernScripts reads the find and replace the file keeps under
// `extensions`.
func readSillyTavernScripts(
	source map[string]json.RawMessage,
) ([]block.Script, map[uuid.UUID]map[string]json.RawMessage) {
	extensions := keys.Object(source[stExtensions])
	var listed []json.RawMessage
	if !keys.Take(extensions, stScripts, &listed) {
		return nil, nil
	}
	// An empty list is a key the file carries and this module has no content
	// for, so it goes back where it was rather than being taken and dropped.
	if len(listed) == 0 {
		return nil, nil
	}
	if len(extensions) == 0 {
		delete(source, stExtensions)
	} else {
		source[stExtensions], _ = json.Marshal(extensions)
	}

	scripts := make([]block.Script, 0, len(listed))
	fields := make(map[uuid.UUID]map[string]json.RawMessage, len(listed))
	for _, raw := range listed {
		script := keys.Object(raw)
		if len(script) == 0 {
			continue
		}
		read := block.Script{ID: block.NewItemID(), Enabled: true}
		keys.Take(script, stScriptName, &read.Name)
		keys.Take(script, stScriptFind, &read.Find)
		keys.Take(script, stScriptReplace, &read.Replace)
		keys.Take(script, stScriptTrim, &read.Trim)
		keys.Take(script, stScriptOnEdit, &read.RunOnEdit)
		var minDepth, maxDepth int
		if keys.Take(script, stScriptMinDepth, &minDepth) {
			read.MinDepth = &minDepth
		}
		if keys.Take(script, stScriptMaxDepth, &maxDepth) {
			read.MaxDepth = &maxDepth
		}
		var disabled bool
		if keys.Take(script, stScriptDisabled, &disabled) {
			read.Enabled = !disabled
		}
		read.Targets = takeNumberedTargets(script)
		read.Affects = takeSillyTavernEffects(script)
		scripts = append(scripts, read)
		if len(script) > 0 {
			fields[read.ID] = script
		}
	}
	return scripts, fields
}

// takeNumberedTargets reads the text a script runs over. SillyTavern numbers
// them, and a number Illarin has no wording for leaves the whole list where it
// is rather than being read to the nearest one it has.
func takeNumberedTargets(script map[string]json.RawMessage) []block.ScriptTarget {
	var numbered []float64
	if !keys.Take(script, stScriptOver, &numbered) {
		return nil
	}
	targets := make([]block.ScriptTarget, 0, len(numbered))
	for _, number := range numbered {
		target, wording := sillyTavernScriptTargets[number]
		if !wording {
			script[stScriptOver] = keys.Must(numbered)
			return nil
		}
		targets = append(targets, target)
	}
	return targets
}

// takeSillyTavernEffects reads what a replacement changes. The file says it
// with two switches, and neither one set means the replacement changes both
// what a person is shown and what the model is sent.
func takeSillyTavernEffects(script map[string]json.RawMessage) []block.ScriptEffect {
	var display, prompt bool
	shown := keys.Take(script, stScriptDisplay, &display)
	sent := keys.Take(script, stScriptPrompt, &prompt)
	if !shown && !sent {
		return nil
	}
	switch {
	case display && !prompt:
		return []block.ScriptEffect{block.EffectDisplay}
	case prompt && !display:
		return []block.ScriptEffect{block.EffectPrompt}
	default:
		return []block.ScriptEffect{block.EffectDisplay, block.EffectPrompt}
	}
}

func sillyTavernScriptLeftovers(
	scripts []block.Script,
	fields map[uuid.UUID]map[string]json.RawMessage,
) map[uuid.UUID]itemLeftover {
	leftovers := make(map[uuid.UUID]itemLeftover, len(fields))
	for _, script := range scripts {
		if held, kept := fields[script.ID]; kept {
			leftovers[script.ID] = itemLeftover{
				namespace: sillyTavernScriptNamespace, fields: held,
			}
		}
	}
	return leftovers
}

// sillyTavernRemainder is everything the file carried that did not become
// content: its own leftover keys, one namespace per key of `extensions`, and
// one item's leftover keys on the item they came from.
func sillyTavernRemainder(
	source map[string]json.RawMessage,
	items ...map[uuid.UUID]itemLeftover,
) []format.Remainder {
	extensions := make(map[string]json.RawMessage)
	if raw, held := source[stExtensions]; held {
		if json.Unmarshal(raw, &extensions) == nil {
			delete(source, stExtensions)
		} else {
			extensions = make(map[string]json.RawMessage)
		}
	}
	reserved := []string{
		sillyTavernNamespace, sillyTavernPromptNamespace, sillyTavernScriptNamespace,
	}
	clashes := make(map[string]json.RawMessage)
	for _, name := range reserved {
		if collision, clash := extensions[name]; clash {
			clashes[name] = collision
			delete(extensions, name)
		}
	}
	if len(clashes) > 0 {
		source[stExtensions], _ = json.Marshal(clashes)
	}

	rows := make([]format.Remainder, 0, len(extensions)+1)
	if len(source) > 0 {
		payload, _ := json.Marshal(source)
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: sillyTavernNamespace, Payload: payload,
		})
	}
	for _, namespace := range slices.Sorted(maps.Keys(extensions)) {
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: extensions[namespace],
		})
	}
	for _, group := range items {
		for _, id := range slices.SortedFunc(maps.Keys(group), compareIDs) {
			payload, _ := json.Marshal(group[id].fields)
			rows = append(rows, format.Remainder{
				Owner: format.OwnerItem, OwnerID: id,
				Namespace: group[id].namespace, Payload: payload,
			})
		}
	}
	return rows
}
