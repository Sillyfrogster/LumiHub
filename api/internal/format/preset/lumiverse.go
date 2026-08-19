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

// LumiverseModule reads and writes the preset Lumiverse exports.
//
// The file carries no format name. What it does carry is a schema version, at
// 1 or 2, and that is a marker rather than a version this module could refuse:
// there is one Lumiverse preset behind both numbers.
const LumiverseID = "preset_lumiverse"

// Where a Lumiverse preset's leftovers sit. The first is Illarin's name for
// the file's own top level, the rest are one item's own keys, and every other
// namespace is a key of `extensions`.
const (
	lumiverseNamespace         = LumiverseID
	lumiverseBlockNamespace    = LumiverseID + "_block"
	lumiverseCategoryNamespace = LumiverseID + "_category"
	lumiverseVariableNamespace = LumiverseID + "_variable"
	lumiverseScriptNamespace   = LumiverseID + "_script"
)

// The file's own top-level keys.
const (
	lvSchemaVersion = "schemaVersion"
	lvBlocks        = "blocks"
	lvSaved         = "promptVariables"
	lvSamplers      = "samplerOverrides"
	lvCompletion    = "completionSettings"
	lvAdvanced      = "advancedSettings"
	lvBehaviour     = "promptBehavior"
	lvScripts       = "regex_scripts"
	lvExtensions    = "extensions"
	lvName          = "name"
	lvDescription   = "description"
	lvVersion       = "presetVersion"
)

// One block's own keys. A block is either a heading or a prompt fragment, and
// the marker is what says which.
const (
	lvBlockID       = "id"
	lvBlockName     = "name"
	lvBlockRole     = "role"
	lvBlockText     = "content"
	lvBlockMarker   = "marker"
	lvBlockEnabled  = "enabled"
	lvBlockPosition = "position"
	lvBlockDepth    = "depth"
	lvBlockGroup    = "group"
	lvBlockVars     = "variables"
	lvHeadingMarker = "category"
)

// One variable definition's own keys, and one choice's.
const (
	lvVarID          = "id"
	lvVarName        = "name"
	lvVarWidget      = "type"
	lvVarLabel       = "label"
	lvVarDescription = "description"
	lvVarDefault     = "defaultValue"
	lvVarOptions     = "options"
	lvVarMin         = "min"
	lvVarMax         = "max"
	lvVarStep        = "step"
	lvVarSeparator   = "separator"
	lvVarRows        = "rows"
	lvOptionKey      = "id"
	lvOptionLabel    = "label"
	lvOptionValue    = "value"
)

// One script's own keys.
const (
	lvScriptName        = "name"
	lvScriptDescription = "description"
	lvScriptFind        = "find_regex"
	lvScriptFlags       = "flags"
	lvScriptReplace     = "replace_string"
	lvScriptTrim        = "trim_strings"
	lvScriptOver        = "placement"
	lvScriptChanges     = "target"
	lvScriptDisabled    = "disabled"
	lvScriptMinDepth    = "min_depth"
	lvScriptMaxDepth    = "max_depth"
	lvScriptRunOnEdit   = "run_on_edit"
)

// The text a Lumiverse script runs over, and what its replacement changes.
var (
	lumiverseScriptTargets = map[string]block.ScriptTarget{
		"user_input": block.TargetUserInput,
		"ai_output":  block.TargetModelOutput,
	}
	lumiverseScriptEffects = map[string]block.ScriptEffect{
		"display": block.EffectDisplay,
		"prompt":  block.EffectPrompt,
	}
)

// LumiverseModule reads and writes a Lumiverse preset.
type LumiverseModule struct{}

func (LumiverseModule) ID() string { return LumiverseID }

func (LumiverseModule) Declaration() format.Declaration {
	named := slotsByApp[Lumiverse]
	return format.Declaration{
		ID: LumiverseID, Label: "Lumiverse preset", Kind: Kind,
		Direction: format.Direction{Read: true, Write: true},
		// The schema version is the whole of the evidence. It says the file is
		// the sort of thing this module reads and nothing more, so a file at
		// some other number is a file nothing here recognises rather than a
		// version of this format that is unsupported.
		Recognition: []format.Recognition{{
			Kind:       format.RecognitionDiscriminator,
			Containers: []probe.Container{probe.JSON},
			Path:       []string{lvSchemaVersion},
			Values:     []string{"1", "2"},
		}},
		Roles: map[block.Role]format.DirectionalRoleSupport{
			block.RolePromptFragments: {
				Read:  format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{Grade: format.SupportFull},
			},
			block.RolePromptVariables: {
				Read: format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{
					Grade: format.SupportPartial,
					Condition: &format.ContentCondition{
						Description: "a variable that belongs to no prompt fragment, " +
							"because the file keeps a variable on the fragment it belongs to",
						Matches: hasLooseVariable,
					},
				},
			},
			block.RoleSamplerSettings:    lumiverseSettingSupport(named.samplers),
			block.RoleCompletionSettings: lumiverseSettingSupport(named.completion),
			block.RoleAdvancedSettings:   lumiverseSettingSupport(named.advanced),
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
		// A Lumiverse preset names itself, says what it is for a person, and
		// carries its own version. The description is the same text the
		// catalog shows while browsing, so it binds both ways rather than
		// being kept twice.
		Header: []format.HeaderField{
			format.HeaderName, format.HeaderBlurb, format.HeaderAssetVersion,
		},
		Slots: declaredSlots(Lumiverse),
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys: []string{
			lvName, lvDescription, lvVersion, lvBlocks, lvSaved, lvSamplers,
			lvCompletion, lvAdvanced, lvBehaviour, lvScripts,
		},
		// No boilerplate. A preset carries an `extensions` object only where a
		// tool put something in it, and nothing in the corpus stamps a
		// namespace that records nothing onto every file.
		Boilerplate: nil,
		Preservation: format.PreservationDeclaration{
			Body: lumiverseNamespace, Container: []string{lvExtensions},
		},
		// Its own format and Illarin-authored presets. Neither preset format
		// converts to the other, so the SillyTavern preset is not here
		// (ADR-0020).
		TestedOrigins: []string{LumiverseID, format.OriginIllarin},
	}
}

// lumiverseSettingSupport is what one settings group survives. Reading takes
// whatever of the app's names the file supplied; writing puts back the names
// this app reads and no others, so a name from somewhere else stays behind.
func lumiverseSettingSupport(named []slot) format.DirectionalRoleSupport {
	return format.DirectionalRoleSupport{
		Read:  format.RoleSupport{Grade: format.SupportFull},
		Write: format.RoleSupport{Grade: format.SupportPartial, Condition: unnamedSetting(named)},
	}
}

func (m LumiverseModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

// Parse reads a Lumiverse preset into the seven roles a preset has.
//
// The block list is the required part. If it will not parse the import is
// refused and nothing is stored; past that point a failure costs only what
// failed, so one unreadable setting leaves every other one whole.
func (m LumiverseModule) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return format.Parsed{}, fmt.Errorf("%s payload: the claimed payload is missing", LumiverseID)
	}
	// The probe's payload is read by every module that looked at this file, so
	// the leftovers are computed on a copy of it.
	source := maps.Clone(payload.Root)

	var blocks []json.RawMessage
	if raw, present := source[lvBlocks]; !present || json.Unmarshal(raw, &blocks) != nil {
		return format.Parsed{}, format.MalformedInput(fmt.Errorf(
			"%s blocks: a preset's prompt fragments have to be a list", LumiverseID,
		))
	}
	delete(source, lvBlocks)

	saved := savedValues(source)
	read := readLumiverseBlocks(blocks, saved)

	elements := []block.Element{{
		ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
		Content: read.list,
	}}
	if len(read.variables) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeVariableSchema, Role: block.RolePromptVariables,
			Content: block.VariableSchema{Variables: read.variables},
		})
	}
	named := slotsByApp[Lumiverse]
	for _, group := range []struct {
		key   string
		role  block.Role
		slots []slot
	}{
		{lvSamplers, block.RoleSamplerSettings, named.samplers},
		{lvCompletion, block.RoleCompletionSettings, named.completion},
		{lvAdvanced, block.RoleAdvancedSettings, named.advanced},
	} {
		if element, filled := readNested(source, group.key, func(
			values map[string]json.RawMessage,
		) (block.Element, bool) {
			return settingsElement(group.role, values, group.slots)
		}); filled {
			elements = append(elements, element)
		}
	}
	if element, filled := readNested(source, lvBehaviour, func(
		values map[string]json.RawMessage,
	) (block.Element, bool) {
		return nudgesElement(values, named.nudges)
	}); filled {
		elements = append(elements, element)
	}

	scripts, scriptFields := readLumiverseScripts(source)
	if len(scripts) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeScriptList, Role: block.RoleRegexScripts,
			Content: block.ScriptList{Scripts: scripts},
		})
	}

	var name, blurb, version string
	keys.Take(source, lvName, &name)
	keys.Take(source, lvDescription, &blurb)
	keys.Take(source, lvVersion, &version)

	return format.Parsed{
		Kind: Kind, Format: LumiverseID,
		Header:   format.Header{Name: name, Blurb: blurb, AssetVersion: version},
		Elements: elements,
		Remainder: lumiverseRemainder(
			source, read.leftovers, scriptLeftovers(scripts, scriptFields),
		),
	}, nil
}

// readNested reads one of the file's own objects into an element and puts back
// whatever the element did not take. An object emptied of everything this
// module reads is a key the file no longer needs.
func readNested(
	source map[string]json.RawMessage,
	key string,
	read func(map[string]json.RawMessage) (block.Element, bool),
) (block.Element, bool) {
	raw, present := source[key]
	if !present {
		return block.Element{}, false
	}
	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil || values == nil {
		return block.Element{}, false
	}
	element, filled := read(values)
	if len(values) == 0 {
		delete(source, key)
	} else {
		source[key], _ = json.Marshal(values)
	}
	return element, filled
}

// savedValues takes the choices a creator saved for their variables. They are
// keyed by the block the variable belongs to, so they are read here and handed
// to whichever variable they name.
func savedValues(source map[string]json.RawMessage) map[string]map[string]json.RawMessage {
	var saved map[string]map[string]json.RawMessage
	if keys.Take(source, lvSaved, &saved) {
		return saved
	}
	return nil
}

// readBlocks is one pass over the block list. A block is a heading or a prompt
// fragment, and everything a preset holds under a fragment comes out with it.
type readBlocks struct {
	list      block.PromptList
	variables []block.Variable
	leftovers map[uuid.UUID]itemLeftover
}

// itemLeftover is what one item carried that Illarin has no place for, and the
// namespace it belongs under.
type itemLeftover struct {
	namespace string
	fields    map[string]json.RawMessage
}

// readLumiverseBlocks reads the block list into headings and fragments.
//
// A file says which heading a fragment sits under in one of two ways. Where it
// carries the heading's own identifier it is taken at face value, and where it
// carries no such key at all the fragment belongs to the heading above it.
// Both are in the corpus and they never appear on one block.
func readLumiverseBlocks(
	blocks []json.RawMessage,
	saved map[string]map[string]json.RawMessage,
) readBlocks {
	read := readBlocks{leftovers: make(map[uuid.UUID]itemLeftover, len(blocks))}
	headings := make(map[string]uuid.UUID, len(blocks))
	above := uuid.Nil

	for _, raw := range blocks {
		fields := keys.Object(raw)
		if len(fields) == 0 {
			continue
		}
		var fileID, marker string
		keys.Take(fields, lvBlockID, &fileID)
		keys.Take(fields, lvBlockMarker, &marker)

		if marker == lvHeadingMarker {
			group := block.PromptGroup{ID: block.NewItemID()}
			keys.Take(fields, lvBlockName, &group.Name)
			read.list.Groups = append(read.list.Groups, group)
			headings[fileID] = group.ID
			above = group.ID
			read.keep(group.ID, lumiverseCategoryNamespace, fields, fileID)
			continue
		}

		fragment := block.PromptFragment{ID: block.NewItemID(), Marker: marker}
		keys.Take(fields, lvBlockName, &fragment.Name)
		keys.Take(fields, lvBlockRole, &fragment.Role)
		keys.Take(fields, lvBlockText, &fragment.Text)
		keys.Take(fields, lvBlockEnabled, &fragment.Enabled)
		keys.Take(fields, lvBlockPosition, &fragment.Placement)
		var depth int
		if keys.Take(fields, lvBlockDepth, &depth) {
			fragment.Depth = &depth
		}
		if !fragment.Role.Known() {
			fields[lvBlockRole] = keys.Must(fragment.Role)
			fragment.Role = ""
		}
		if !fragment.Placement.Known() {
			fields[lvBlockPosition] = keys.Must(fragment.Placement)
			fragment.Placement = ""
		}
		fragment.GroupID = fragmentHeading(fields, headings, above)

		read.variables = append(
			read.variables, readLumiverseVariables(fields, fragment.ID, saved[fileID], &read)...,
		)
		read.list.Fragments = append(read.list.Fragments, fragment)
		read.keep(fragment.ID, lumiverseBlockNamespace, fields, fileID)
	}
	return read
}

// keep puts one item's leftover keys aside, with the identifier the file knew
// it by among them. Illarin mints its own ids, so the file's is preserved data
// like anything else and the writer puts it back where it was.
func (r *readBlocks) keep(
	id uuid.UUID,
	namespace string,
	fields map[string]json.RawMessage,
	fileID string,
) {
	if fileID != "" {
		fields[lvBlockID] = keys.Must(fileID)
	}
	if len(fields) > 0 {
		r.leftovers[id] = itemLeftover{namespace: namespace, fields: fields}
	}
}

// fragmentHeading answers which heading a fragment sits under. A file that
// names one names it by the heading's own identifier; a file that names none
// at all leaves the fragment under the last heading above it.
func fragmentHeading(
	fields map[string]json.RawMessage,
	headings map[string]uuid.UUID,
	above uuid.UUID,
) *uuid.UUID {
	raw, present := fields[lvBlockGroup]
	if !present {
		if above == uuid.Nil {
			return nil
		}
		return &above
	}
	var named string
	if keys.IsNull(raw) || json.Unmarshal(raw, &named) != nil {
		delete(fields, lvBlockGroup)
		return nil
	}
	heading, known := headings[named]
	if !known {
		return nil
	}
	delete(fields, lvBlockGroup)
	return &heading
}

// readLumiverseVariables reads the form a fragment asks a reader to fill in,
// and the choices the creator saved against it.
func readLumiverseVariables(
	fields map[string]json.RawMessage,
	fragmentID uuid.UUID,
	saved map[string]json.RawMessage,
	read *readBlocks,
) []block.Variable {
	var defined []json.RawMessage
	if !keys.Take(fields, lvBlockVars, &defined) {
		return nil
	}
	variables := make([]block.Variable, 0, len(defined))
	for _, raw := range defined {
		definition := keys.Object(raw)
		if len(definition) == 0 {
			continue
		}
		variable := block.Variable{ID: block.NewItemID(), FragmentID: &fragmentID}
		keys.Take(definition, lvVarName, &variable.Name)
		keys.Take(definition, lvVarWidget, &variable.Widget)
		keys.Take(definition, lvVarLabel, &variable.Label)
		keys.Take(definition, lvVarDescription, &variable.Description)
		keys.Take(definition, lvVarSeparator, &variable.Separator)
		keys.Take(definition, lvVarRows, &variable.Rows)
		if !variable.Widget.Known() {
			definition[lvVarWidget] = keys.Must(variable.Widget)
			variable.Widget = block.WidgetText
		}
		variable.Options = readLumiverseOptions(definition)
		variable.Range = readLumiverseRange(definition)
		if raw, present := definition[lvVarDefault]; present {
			if value, readable := readFreeValue(raw); readable {
				delete(definition, lvVarDefault)
				variable.Default = value
			}
		}
		if value, readable := readFreeValue(saved[variable.Name]); readable {
			variable.Value = value
		}
		variables = append(variables, variable)
		if len(definition) > 0 {
			read.leftovers[variable.ID] = itemLeftover{
				namespace: lumiverseVariableNamespace, fields: definition,
			}
		}
	}
	return variables
}

// readLumiverseOptions reads what a select offers. The file names a choice
// separately from the text it stands for, and a saved choice names the first,
// so both come across.
func readLumiverseOptions(definition map[string]json.RawMessage) []block.VariableOption {
	var listed []json.RawMessage
	if !keys.Take(definition, lvVarOptions, &listed) {
		return nil
	}
	options := make([]block.VariableOption, 0, len(listed))
	for _, raw := range listed {
		choice := keys.Object(raw)
		option := block.VariableOption{}
		keys.Take(choice, lvOptionKey, &option.Key)
		keys.Take(choice, lvOptionLabel, &option.Label)
		keys.Take(choice, lvOptionValue, &option.Value)
		options = append(options, option)
	}
	return options
}

func readLumiverseRange(definition map[string]json.RawMessage) *block.VariableRange {
	bounds := block.VariableRange{}
	var min, max, step float64
	bounded := keys.Take(definition, lvVarMin, &min)
	if bounded {
		bounds.Min = &min
	}
	if keys.Take(definition, lvVarMax, &max) {
		bounds.Max, bounded = &max, true
	}
	if keys.Take(definition, lvVarStep, &step) {
		bounds.Step, bounded = &step, true
	}
	if !bounded {
		return nil
	}
	return &bounds
}

// readFreeValue reads a value whose shape the file decides rather than a
// declared slot. A variable's default and the choice saved for it are both
// whatever the widget wanted.
func readFreeValue(raw json.RawMessage) (*block.Value, bool) {
	if len(raw) == 0 || keys.IsNull(raw) {
		return nil, false
	}
	var held any
	if json.Unmarshal(raw, &held) != nil {
		return nil, false
	}
	switch held.(type) {
	case float64:
		var number float64
		_ = json.Unmarshal(raw, &number)
		return &block.Value{Number: &number}, true
	case bool:
		var yes bool
		_ = json.Unmarshal(raw, &yes)
		return &block.Value{Boolean: &yes}, true
	case string:
		var text string
		_ = json.Unmarshal(raw, &text)
		return &block.Value{Text: &text}, true
	case []any:
		var strings []string
		if json.Unmarshal(raw, &strings) != nil {
			return nil, false
		}
		return &block.Value{Strings: strings}, true
	}
	return nil, false
}

// readLumiverseScripts reads the find and replace list. The file keeps a copy
// of it under `extensions` as well, and the two are the same list, so both are
// read here and the writer puts both back.
func readLumiverseScripts(
	source map[string]json.RawMessage,
) ([]block.Script, map[uuid.UUID]map[string]json.RawMessage) {
	var listed []json.RawMessage
	if !keys.Take(source, lvScripts, &listed) {
		return nil, nil
	}
	// An empty list is a key the file carries and this module has no content
	// for, so it goes back where it was rather than being taken and dropped.
	if len(listed) == 0 {
		source[lvScripts] = keys.Must([]json.RawMessage{})
		return nil, nil
	}
	if extensions := keys.Object(source[lvExtensions]); len(extensions) > 0 {
		delete(extensions, lvScripts)
		if len(extensions) == 0 {
			delete(source, lvExtensions)
		} else {
			source[lvExtensions], _ = json.Marshal(extensions)
		}
	}

	scripts := make([]block.Script, 0, len(listed))
	fields := make(map[uuid.UUID]map[string]json.RawMessage, len(listed))
	for _, raw := range listed {
		script := keys.Object(raw)
		if len(script) == 0 {
			continue
		}
		read := block.Script{ID: block.NewItemID(), Enabled: true}
		keys.Take(script, lvScriptName, &read.Name)
		keys.Take(script, lvScriptDescription, &read.Description)
		keys.Take(script, lvScriptFind, &read.Find)
		keys.Take(script, lvScriptFlags, &read.Flags)
		keys.Take(script, lvScriptReplace, &read.Replace)
		keys.Take(script, lvScriptTrim, &read.Trim)
		keys.Take(script, lvScriptRunOnEdit, &read.RunOnEdit)
		var minDepth, maxDepth int
		if keys.Take(script, lvScriptMinDepth, &minDepth) {
			read.MinDepth = &minDepth
		}
		if keys.Take(script, lvScriptMaxDepth, &maxDepth) {
			read.MaxDepth = &maxDepth
		}
		var disabled bool
		if keys.Take(script, lvScriptDisabled, &disabled) {
			read.Enabled = !disabled
		}
		read.Targets = takeScriptTargets(script, lvScriptOver, lumiverseScriptTargets)
		read.Affects = takeScriptEffects(script, lvScriptChanges, lumiverseScriptEffects)
		scripts = append(scripts, read)
		if len(script) > 0 {
			fields[read.ID] = script
		}
	}
	return scripts, fields
}

// takeScriptTargets reads the text a script runs over. A list holding a name
// Illarin has no wording for is left where it is rather than being read to the
// nearest one it has, so the whole list travels back out untouched.
func takeScriptTargets(
	script map[string]json.RawMessage,
	key string,
	known map[string]block.ScriptTarget,
) []block.ScriptTarget {
	var named []string
	if !keys.Take(script, key, &named) {
		return nil
	}
	targets := make([]block.ScriptTarget, 0, len(named))
	for _, name := range named {
		target, wording := known[name]
		if !wording {
			script[key] = keys.Must(named)
			return nil
		}
		targets = append(targets, target)
	}
	return targets
}

func takeScriptEffects(
	script map[string]json.RawMessage,
	key string,
	known map[string]block.ScriptEffect,
) []block.ScriptEffect {
	var named []string
	if !keys.Take(script, key, &named) {
		return nil
	}
	effects := make([]block.ScriptEffect, 0, len(named))
	for _, name := range named {
		effect, wording := known[name]
		if !wording {
			script[key] = keys.Must(named)
			return nil
		}
		effects = append(effects, effect)
	}
	return effects
}

func scriptLeftovers(
	scripts []block.Script,
	fields map[uuid.UUID]map[string]json.RawMessage,
) map[uuid.UUID]itemLeftover {
	leftovers := make(map[uuid.UUID]itemLeftover, len(fields))
	for _, script := range scripts {
		if held, kept := fields[script.ID]; kept {
			leftovers[script.ID] = itemLeftover{
				namespace: lumiverseScriptNamespace, fields: held,
			}
		}
	}
	return leftovers
}

// lumiverseRemainder is everything the file carried that did not become
// content: the preset's own leftover keys, one namespace per key of
// `extensions`, and one item's leftover keys on the item they came from.
func lumiverseRemainder(
	source map[string]json.RawMessage,
	items ...map[uuid.UUID]itemLeftover,
) []format.Remainder {
	extensions := make(map[string]json.RawMessage)
	if raw, held := source[lvExtensions]; held {
		if json.Unmarshal(raw, &extensions) == nil {
			delete(source, lvExtensions)
		} else {
			extensions = make(map[string]json.RawMessage)
		}
	}
	// An extensions key named for one of this module's own namespaces would
	// ask for two namespaces of one name. It travels back out whole either
	// way, so it stays where it is rather than being split out beside its
	// namesake.
	reserved := []string{
		lumiverseNamespace, lumiverseBlockNamespace, lumiverseCategoryNamespace,
		lumiverseVariableNamespace, lumiverseScriptNamespace,
	}
	clashes := make(map[string]json.RawMessage)
	for _, name := range reserved {
		if collision, clash := extensions[name]; clash {
			clashes[name] = collision
			delete(extensions, name)
		}
	}
	if len(clashes) > 0 {
		source[lvExtensions], _ = json.Marshal(clashes)
	}

	rows := make([]format.Remainder, 0, len(extensions)+1)
	if len(source) > 0 {
		payload, _ := json.Marshal(source)
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: lumiverseNamespace, Payload: payload,
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

func compareIDs(first, second uuid.UUID) int {
	return slices.Compare(first[:], second[:])
}
