package block

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// PromptList is a preset's ordered prompt fragments.
type PromptList struct {
	Groups    []PromptGroup    `json:"groups"`
	Fragments []PromptFragment `json:"fragments"`
}

// PromptGroup is a heading over some of the fragments.
type PromptGroup struct {
	// ID anchors preserved data across reordering.
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// PromptFragment is one piece of a preset's prompt.
type PromptFragment struct {
	ID uuid.UUID `json:"id"`
	// Name is a label and never reaches the model.
	Name string `json:"name,omitempty"`
	// GroupID is unset for an ungrouped fragment.
	GroupID *uuid.UUID `json:"groupId,omitempty"`
	Role    PromptRole `json:"role"`
	Text    string     `json:"text"`
	// Marker names content an app splices into this position.
	Marker  string `json:"marker,omitempty"`
	Enabled bool   `json:"enabled"`
	// Placement is unset when the preset leaves the choice open.
	Placement PromptPlacement `json:"placement,omitempty"`
	// Depth counts backward from the most recent message.
	Depth *int `json:"depth,omitempty"`
}

// PromptRole identifies who speaks; append roles extend the prior message.
type PromptRole string

const (
	PromptSystem          PromptRole = "system"
	PromptUser            PromptRole = "user"
	PromptAssistant       PromptRole = "assistant"
	PromptUserAppend      PromptRole = "user_append"
	PromptAssistantAppend PromptRole = "assistant_append"
)

// Known reports whether the role belongs to the closed vocabulary.
func (r PromptRole) Known() bool {
	switch r {
	case "", PromptSystem, PromptUser, PromptAssistant, PromptUserAppend, PromptAssistantAppend:
		return true
	default:
		return false
	}
}

// PromptRoles returns the roles a fragment may speak as.
func PromptRoles() []PromptRole {
	return []PromptRole{
		PromptSystem, PromptUser, PromptAssistant, PromptUserAppend, PromptAssistantAppend,
	}
}

// PromptPlacement is where a fragment goes relative to the conversation.
type PromptPlacement string

const (
	BeforeHistory PromptPlacement = "pre_history"
	AfterHistory  PromptPlacement = "post_history"
	InHistory     PromptPlacement = "in_history"
)

// Known reports whether the placement belongs to the closed vocabulary.
func (p PromptPlacement) Known() bool {
	switch p {
	case "", BeforeHistory, AfterHistory, InHistory:
		return true
	default:
		return false
	}
}

// PromptPlacements returns the placements a fragment may take.
func PromptPlacements() []PromptPlacement {
	return []PromptPlacement{BeforeHistory, AfterHistory, InHistory}
}

// Empty counts marker-only fragments as content.
func (l PromptList) Empty() bool { return len(l.Fragments) == 0 }

// Value holds one setting or variable value.
type Value struct {
	Number  *float64 `json:"number,omitempty"`
	Boolean *bool    `json:"boolean,omitempty"`
	Text    *string  `json:"text,omitempty"`
	// Strings preserves order and duplicates.
	Strings []string `json:"strings,omitempty"`
}

// SettingType identifies what one setting holds.
type SettingType string

const (
	SettingNumber  SettingType = "number"
	SettingBoolean SettingType = "boolean"
	SettingText    SettingType = "text"
	SettingStrings SettingType = "string_list"
)

// Known reports whether the setting type belongs to the closed vocabulary.
func (t SettingType) Known() bool {
	switch t {
	case SettingNumber, SettingBoolean, SettingText, SettingStrings:
		return true
	default:
		return false
	}
}

// SettingTypes returns what a setting may hold.
func SettingTypes() []SettingType {
	return []SettingType{SettingNumber, SettingBoolean, SettingText, SettingStrings}
}

// SettingGroup is a set of app-defined settings.
type SettingGroup struct {
	Settings []Setting `json:"settings"`
}

// Setting is one named slot and whatever is in it.
type Setting struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// Label is readable wording for an opaque slot name.
	Label   string      `json:"label,omitempty"`
	Type    SettingType `json:"type"`
	Choices []string    `json:"choices,omitempty"`
	// Value is nil when nobody supplied one.
	Value *Value `json:"value,omitempty"`
}

// Empty reports whether the group has any supplied values.
func (g SettingGroup) Empty() bool {
	for _, setting := range g.Settings {
		if setting.Value != nil {
			return false
		}
	}
	return true
}

// Supplied reports how many of the group's settings somebody has filled in.
func (g SettingGroup) Supplied() int {
	count := 0
	for _, setting := range g.Settings {
		if setting.Value != nil {
			count++
		}
	}
	return count
}

// VariableSchema is the form a preset asks a reader to fill in.
type VariableSchema struct {
	Variables []Variable `json:"variables"`
}

// VariableWidget is the control a variable is filled in with.
type VariableWidget string

const (
	WidgetSwitch      VariableWidget = "switch"
	WidgetSelect      VariableWidget = "select"
	WidgetMultiSelect VariableWidget = "multiselect"
	WidgetNumber      VariableWidget = "number"
	WidgetSlider      VariableWidget = "slider"
	WidgetText        VariableWidget = "text"
	WidgetTextArea    VariableWidget = "textarea"
)

// Known reports whether the widget belongs to the closed vocabulary.
func (w VariableWidget) Known() bool {
	switch w {
	case WidgetSwitch, WidgetSelect, WidgetMultiSelect, WidgetNumber,
		WidgetSlider, WidgetText, WidgetTextArea:
		return true
	default:
		return false
	}
}

// VariableWidgets returns the controls a variable may be filled in with.
func VariableWidgets() []VariableWidget {
	return []VariableWidget{
		WidgetSwitch, WidgetSelect, WidgetMultiSelect, WidgetNumber,
		WidgetSlider, WidgetText, WidgetTextArea,
	}
}

// Variable is one thing a reader chooses before the preset runs.
type Variable struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Widget      VariableWidget `json:"widget"`
	Label       string         `json:"label,omitempty"`
	Description string         `json:"description,omitempty"`
	// FragmentID is unset when the variable belongs to no fragment.
	FragmentID *uuid.UUID       `json:"fragmentId,omitempty"`
	Default    *Value           `json:"default,omitempty"`
	Value      *Value           `json:"value,omitempty"`
	Options    []VariableOption `json:"options,omitempty"`
	Range      *VariableRange   `json:"range,omitempty"`
	// Separator joins multiselect values in a prompt.
	Separator string `json:"separator,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

// VariableOption pairs display text with a prompt value.
type VariableOption struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// Named returns what a saved choice calls this option.
func (o VariableOption) Named() string {
	if o.Key != "" {
		return o.Key
	}
	return o.Value
}

// VariableRange bounds what a number or a slider accepts.
type VariableRange struct {
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
	Step *float64 `json:"step,omitempty"`
}

// Empty reports whether the schema asks a reader for nothing.
func (s VariableSchema) Empty() bool { return len(s.Variables) == 0 }

// ScriptList is a preset's find and replace scripts, in the order they run.
type ScriptList struct {
	Scripts []Script `json:"scripts"`
}

// ScriptTarget is text a script runs over.
type ScriptTarget string

const (
	TargetUserInput    ScriptTarget = "user_input"
	TargetModelOutput  ScriptTarget = "model_output"
	TargetSlashCommand ScriptTarget = "slash_command"
	TargetLorebook     ScriptTarget = "lorebook"
)

// Known reports whether the target belongs to the closed vocabulary.
func (t ScriptTarget) Known() bool {
	switch t {
	case TargetUserInput, TargetModelOutput, TargetSlashCommand, TargetLorebook:
		return true
	default:
		return false
	}
}

// ScriptTargets returns the text a script may run over.
func ScriptTargets() []ScriptTarget {
	return []ScriptTarget{TargetUserInput, TargetModelOutput, TargetSlashCommand, TargetLorebook}
}

// ScriptEffect is what a replacement changes.
type ScriptEffect string

const (
	EffectDisplay ScriptEffect = "display"
	EffectPrompt  ScriptEffect = "prompt"
)

// Known reports whether the effect belongs to the closed vocabulary.
func (e ScriptEffect) Known() bool {
	return e == EffectDisplay || e == EffectPrompt
}

// ScriptEffects returns what a replacement may change.
func ScriptEffects() []ScriptEffect { return []ScriptEffect{EffectDisplay, EffectPrompt} }

// Script is one find and replace.
type Script struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Find        string    `json:"find"`
	// Flags are regular expression flags.
	Flags   string         `json:"flags,omitempty"`
	Replace string         `json:"replace"`
	Trim    []string       `json:"trim,omitempty"`
	Targets []ScriptTarget `json:"targets,omitempty"`
	Affects []ScriptEffect `json:"affects,omitempty"`
	Enabled bool           `json:"enabled"`
	// MinDepth and MaxDepth count backward from the latest message.
	MinDepth  *int `json:"minDepth,omitempty"`
	MaxDepth  *int `json:"maxDepth,omitempty"`
	RunOnEdit bool `json:"runOnEdit,omitempty"`
}

// Empty reports whether the list would change nothing.
func (l ScriptList) Empty() bool { return len(l.Scripts) == 0 }

func decodePromptList(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Groups *[]struct {
			ID   uuid.UUID `json:"id,omitempty"`
			Name string    `json:"name"`
		} `json:"groups"`
		Fragments *[]struct {
			ID        uuid.UUID       `json:"id,omitempty"`
			Name      string          `json:"name,omitempty"`
			GroupID   *uuid.UUID      `json:"groupId,omitempty"`
			Role      PromptRole      `json:"role"`
			Text      *string         `json:"text"`
			Marker    string          `json:"marker,omitempty"`
			Enabled   *bool           `json:"enabled"`
			Placement PromptPlacement `json:"placement,omitempty"`
			Depth     *int            `json:"depth,omitempty"`
		} `json:"fragments"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Groups == nil || incoming.Fragments == nil {
		return nil, fmt.Errorf("groups and fragments must both be present as lists")
	}

	groups := make([]PromptGroup, len(*incoming.Groups))
	known := make(map[uuid.UUID]struct{}, len(groups))
	for i, group := range *incoming.Groups {
		groups[i] = PromptGroup{ID: itemID(group.ID), Name: group.Name}
		known[groups[i].ID] = struct{}{}
	}

	fragments := make([]PromptFragment, len(*incoming.Fragments))
	for i, item := range *incoming.Fragments {
		if item.Text == nil || item.Enabled == nil {
			return nil, fmt.Errorf(
				"fragment %d must include text as a string and enabled as a yes or no", i+1,
			)
		}
		if !item.Role.Known() {
			return nil, fmt.Errorf(
				"fragment %d speaks as %q. Choose %s before saving",
				i+1, item.Role, joinPromptRoles(),
			)
		}
		if !item.Placement.Known() {
			return nil, fmt.Errorf(
				"fragment %d sits at %q. Choose %s before saving",
				i+1, item.Placement, joinPlacements(),
			)
		}
		if item.GroupID != nil {
			if _, ok := known[*item.GroupID]; !ok {
				return nil, fmt.Errorf(
					"fragment %d names a group this section does not carry. "+
						"Add the group or take the fragment out of it before saving",
					i+1,
				)
			}
		}
		fragments[i] = PromptFragment{
			ID:        itemID(item.ID),
			Name:      item.Name,
			GroupID:   item.GroupID,
			Role:      item.Role,
			Text:      *item.Text,
			Marker:    item.Marker,
			Enabled:   *item.Enabled,
			Placement: item.Placement,
			Depth:     item.Depth,
		}
	}
	return PromptList{Groups: groups, Fragments: fragments}, nil
}

func decodeSettingGroup(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Settings *[]struct {
			ID      uuid.UUID   `json:"id,omitempty"`
			Name    *string     `json:"name"`
			Label   string      `json:"label,omitempty"`
			Type    SettingType `json:"type"`
			Choices []string    `json:"choices,omitempty"`
			Value   *Value      `json:"value,omitempty"`
		} `json:"settings"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Settings == nil {
		return nil, fmt.Errorf("settings must be present as a list")
	}
	settings := make([]Setting, len(*incoming.Settings))
	for i, item := range *incoming.Settings {
		if item.Name == nil || *item.Name == "" {
			return nil, fmt.Errorf("setting %d must be named before saving", i+1)
		}
		if !item.Type.Known() {
			return nil, fmt.Errorf(
				"setting %q holds %q. Choose %s before saving",
				*item.Name, item.Type, joinSettingTypes(),
			)
		}
		settings[i] = Setting{
			ID:      itemID(item.ID),
			Name:    *item.Name,
			Label:   item.Label,
			Type:    item.Type,
			Choices: item.Choices,
			Value:   item.Value,
		}
	}
	return SettingGroup{Settings: settings}, nil
}

func decodeVariableSchema(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Variables *[]struct {
			ID          uuid.UUID        `json:"id,omitempty"`
			Name        *string          `json:"name"`
			Widget      VariableWidget   `json:"widget"`
			Label       string           `json:"label,omitempty"`
			Description string           `json:"description,omitempty"`
			FragmentID  *uuid.UUID       `json:"fragmentId,omitempty"`
			Default     *Value           `json:"default,omitempty"`
			Value       *Value           `json:"value,omitempty"`
			Options     []VariableOption `json:"options,omitempty"`
			Range       *VariableRange   `json:"range,omitempty"`
			Separator   string           `json:"separator,omitempty"`
			Rows        int              `json:"rows,omitempty"`
		} `json:"variables"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Variables == nil {
		return nil, fmt.Errorf("variables must be present as a list")
	}
	variables := make([]Variable, len(*incoming.Variables))
	for i, item := range *incoming.Variables {
		if item.Name == nil || *item.Name == "" {
			return nil, fmt.Errorf("variable %d must be named before saving", i+1)
		}
		if !item.Widget.Known() {
			return nil, fmt.Errorf(
				"variable %q is filled in with %q. Choose %s before saving",
				*item.Name, item.Widget, joinWidgets(),
			)
		}
		variables[i] = Variable{
			ID:          itemID(item.ID),
			Name:        *item.Name,
			Widget:      item.Widget,
			Label:       item.Label,
			Description: item.Description,
			FragmentID:  item.FragmentID,
			Default:     item.Default,
			Value:       item.Value,
			Options:     item.Options,
			Range:       item.Range,
			Separator:   item.Separator,
			Rows:        item.Rows,
		}
	}
	return VariableSchema{Variables: variables}, nil
}

func decodeScriptList(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Scripts *[]struct {
			ID          uuid.UUID      `json:"id,omitempty"`
			Name        string         `json:"name,omitempty"`
			Description string         `json:"description,omitempty"`
			Find        *string        `json:"find"`
			Flags       string         `json:"flags,omitempty"`
			Replace     *string        `json:"replace"`
			Trim        []string       `json:"trim,omitempty"`
			Targets     []ScriptTarget `json:"targets,omitempty"`
			Affects     []ScriptEffect `json:"affects,omitempty"`
			Enabled     *bool          `json:"enabled"`
			MinDepth    *int           `json:"minDepth,omitempty"`
			MaxDepth    *int           `json:"maxDepth,omitempty"`
			RunOnEdit   bool           `json:"runOnEdit,omitempty"`
		} `json:"scripts"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Scripts == nil {
		return nil, fmt.Errorf("scripts must be present as a list")
	}
	scripts := make([]Script, len(*incoming.Scripts))
	for i, item := range *incoming.Scripts {
		if item.Find == nil || item.Replace == nil || item.Enabled == nil {
			return nil, fmt.Errorf(
				"script %d must include what to find and what to replace it with, "+
					"and enabled as a yes or no",
				i+1,
			)
		}
		for _, target := range item.Targets {
			if !target.Known() {
				return nil, fmt.Errorf(
					"script %d runs over %q. Choose from %s before saving",
					i+1, target, joinScriptTargets(),
				)
			}
		}
		for _, effect := range item.Affects {
			if !effect.Known() {
				return nil, fmt.Errorf(
					"script %d changes %q. Choose what a person is shown, "+
						"what the model is sent, or both, before saving",
					i+1, effect,
				)
			}
		}
		if item.MinDepth != nil && item.MaxDepth != nil && *item.MinDepth > *item.MaxDepth {
			return nil, fmt.Errorf(
				"script %d reaches back from %d to %d, which is no messages at all",
				i+1, *item.MinDepth, *item.MaxDepth,
			)
		}
		scripts[i] = Script{
			ID:          itemID(item.ID),
			Name:        item.Name,
			Description: item.Description,
			Find:        *item.Find,
			Flags:       item.Flags,
			Replace:     *item.Replace,
			Trim:        item.Trim,
			Targets:     item.Targets,
			Affects:     item.Affects,
			Enabled:     *item.Enabled,
			MinDepth:    item.MinDepth,
			MaxDepth:    item.MaxDepth,
			RunOnEdit:   item.RunOnEdit,
		}
	}
	return ScriptList{Scripts: scripts}, nil
}

func joinPromptRoles() string {
	names := make([]string, 0, len(PromptRoles()))
	for _, role := range PromptRoles() {
		names = append(names, string(role))
	}
	return joinWithOr(names)
}

func joinPlacements() string {
	names := make([]string, 0, len(PromptPlacements()))
	for _, placement := range PromptPlacements() {
		names = append(names, string(placement))
	}
	return joinWithOr(names)
}

func joinSettingTypes() string {
	names := make([]string, 0, len(SettingTypes()))
	for _, settingType := range SettingTypes() {
		names = append(names, string(settingType))
	}
	return joinWithOr(names)
}

func joinWidgets() string {
	names := make([]string, 0, len(VariableWidgets()))
	for _, widget := range VariableWidgets() {
		names = append(names, string(widget))
	}
	return joinWithOr(names)
}

func joinScriptTargets() string {
	names := make([]string, 0, len(ScriptTargets()))
	for _, target := range ScriptTargets() {
		names = append(names, string(target))
	}
	return joinWithOr(names)
}
