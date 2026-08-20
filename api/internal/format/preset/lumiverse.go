package preset

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

// LumiverseID identifies presets whose schema version is 1 or 2.
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

// lumiversePreservation is where this module keeps what a file carried that
// Illarin has no place for.
var lumiversePreservation = preservation{
	body: lumiverseNamespace, extensions: lvExtensions,
	reserved: []string{
		lumiverseNamespace, lumiverseBlockNamespace, lumiverseCategoryNamespace,
		lumiverseVariableNamespace, lumiverseScriptNamespace,
	},
}

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
		// The schema version is the format discriminator, not a compatibility gate.
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
		// Description and catalog blurb share one value.
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

// Parse reads a Lumiverse preset and preserves fields it cannot model.
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

	var name, version string
	keys.Take(source, lvName, &name)
	keys.Take(source, lvVersion, &version)

	return format.Parsed{
		Kind: Kind, Format: LumiverseID,
		Header:   format.Header{Name: name, Blurb: boundBlurb(source), AssetVersion: version},
		Elements: elements,
		Remainder: lumiversePreservation.remainder(
			source, read.leftovers,
			scriptLeftovers(scripts, lumiverseScriptNamespace, scriptFields),
		),
	}, nil
}

// boundBlurb binds descriptions that fit the catalog. Longer text stays
// preserved in the source payload.
func boundBlurb(source map[string]json.RawMessage) string {
	var description string
	if !keys.Take(source, lvDescription, &description) {
		return ""
	}
	if len([]rune(description)) > format.MaxBlurbRunes {
		source[lvDescription] = keys.Must(description)
		return ""
	}
	return description
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
