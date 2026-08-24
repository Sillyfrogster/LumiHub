package preset

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
	"github.com/google/uuid"
)

// readBlocks is one pass over the block list. A block is a heading or a prompt
// fragment, and everything a preset holds under a fragment comes out with it.
type readBlocks struct {
	list      block.PromptList
	variables []block.Variable
	leftovers map[uuid.UUID]itemLeftover
	protected []format.ProtectedPrompt
}

// readLumiverseBlocks resolves explicit group IDs and implicit preceding
// headings while reading the block list.
func readLumiverseBlocks(
	blocks []json.RawMessage,
	saved map[string]map[string]json.RawMessage,
) (readBlocks, error) {
	read := readBlocks{leftovers: make(map[uuid.UUID]itemLeftover, len(blocks))}
	headings := make(map[string]uuid.UUID, len(blocks))
	sealedKeys := make(map[string]bool)
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
			if err := validateLumiverseHeadingSealing(fields); err != nil {
				return readBlocks{}, err
			}
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
		textPresent := keys.Take(fields, lvBlockText, &fragment.Text)
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
		private, sealed, err := readLumiverseSealedPrompt(
			fields, fragment.ID, fragment.Text, textPresent,
		)
		if err != nil {
			return readBlocks{}, err
		}
		if sealed {
			if sealedKeys[private.SourceKey] {
				return readBlocks{}, fmt.Errorf(
					"sealed prompt key %q appears more than once", private.SourceKey,
				)
			}
			sealedKeys[private.SourceKey] = true
			fragment.Protected = true
			fragment.Text = ""
			read.protected = append(read.protected, private)
		}

		read.variables = append(
			read.variables, readLumiverseVariables(fields, fragment.ID, saved[fileID], &read)...,
		)
		read.list.Fragments = append(read.list.Fragments, fragment)
		read.keep(fragment.ID, lumiverseBlockNamespace, fields, fileID)
	}
	return read, nil
}

func validateLumiverseHeadingSealing(fields map[string]json.RawMessage) error {
	if _, present := fields[lvBlockSealKey]; present {
		return errors.New("sealed prompt metadata belongs on a prompt fragment")
	}
	if _, present := fields[lvBlockSealKeyLegacy]; present {
		return errors.New("sealed prompt metadata belongs on a prompt fragment")
	}
	raw, present := fields[lvBlockSealed]
	if !present {
		return nil
	}
	var sealed bool
	if json.Unmarshal(raw, &sealed) != nil || sealed {
		return errors.New("sealed prompt metadata belongs on a prompt fragment")
	}
	return nil
}

func readLumiverseSealedPrompt(
	fields map[string]json.RawMessage,
	fragmentID uuid.UUID,
	text string,
	textPresent bool,
) (format.ProtectedPrompt, bool, error) {
	sealedRaw, hasSealed := fields[lvBlockSealed]
	_, hasKey := fields[lvBlockSealKey]
	_, hasLegacyKey := fields[lvBlockSealKeyLegacy]
	if !hasSealed && !hasKey && !hasLegacyKey {
		return format.ProtectedPrompt{}, false, nil
	}

	var sealed bool
	if !hasSealed || json.Unmarshal(sealedRaw, &sealed) != nil {
		return format.ProtectedPrompt{}, false, fmt.Errorf(
			"sealed prompt metadata needs sealed as true or false",
		)
	}
	if !sealed {
		if hasKey || hasLegacyKey {
			return format.ProtectedPrompt{}, false, fmt.Errorf(
				"sealed prompt key requires sealed to be true",
			)
		}
		return format.ProtectedPrompt{}, false, nil
	}
	if hasKey == hasLegacyKey {
		return format.ProtectedPrompt{}, false, fmt.Errorf(
			"sealed prompt needs exactly one sealedKey or sealed_key",
		)
	}
	if !textPresent {
		return format.ProtectedPrompt{}, false, fmt.Errorf(
			"sealed prompt content must be text",
		)
	}

	keyName := lvBlockSealKey
	if hasLegacyKey {
		keyName = lvBlockSealKeyLegacy
	}
	var sourceKey string
	if json.Unmarshal(fields[keyName], &sourceKey) != nil ||
		sourceKey == "" || sourceKey != strings.TrimSpace(sourceKey) ||
		len([]rune(sourceKey)) > 256 {
		return format.ProtectedPrompt{}, false, fmt.Errorf(
			"sealed prompt key must be 1 to 256 trimmed characters",
		)
	}

	delete(fields, lvBlockSealed)
	delete(fields, keyName)
	placeholder := "{{presetBlock::" + sourceKey + "}}"
	trimmed := strings.TrimSpace(text)
	reuse := trimmed == placeholder
	if !reuse && strings.HasPrefix(trimmed, "{{presetBlock::") && strings.HasSuffix(trimmed, "}}") {
		return format.ProtectedPrompt{}, false, fmt.Errorf(
			"sealed prompt placeholder does not match key %q", sourceKey,
		)
	}
	if reuse {
		text = ""
	}
	return format.ProtectedPrompt{
		FragmentID: fragmentID, SourceKey: sourceKey,
		Text: text, ReuseExisting: reuse,
	}, true, nil
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
