package block

import (
	"encoding/json"
	"fmt"
)

// schema decodes one element type and upgrades stored content to its current
// version.
type schema struct {
	// upgrade[i] rewrites content written at version i+1 into version i+2, so
	// the current version is one past the last upgrade.
	upgrade []func(json.RawMessage) (json.RawMessage, error)
	empty   func() Content
	decode  func(json.RawMessage) (Content, error)
}

func (s schema) version() int { return len(s.upgrade) + 1 }

// read turns stored content into the current shape. Content written by a build
// newer than this one is refused rather than read as if it were current.
func (s schema) read(version int, stored json.RawMessage) (Content, error) {
	if version < 1 || version > s.version() {
		return nil, fmt.Errorf(
			"content is at schema version %d and this build reads up to %d",
			version, s.version(),
		)
	}
	if len(stored) == 0 {
		return s.empty(), nil
	}
	var err error
	for _, upgrade := range s.upgrade[version-1:] {
		if stored, err = upgrade(stored); err != nil {
			return nil, err
		}
	}
	return s.decode(stored)
}

// decodeAs reads one element type's stored content into its own struct.
func decodeAs[C Content](stored json.RawMessage) (Content, error) {
	var content C
	if err := json.Unmarshal(stored, &content); err != nil {
		return nil, err
	}
	return content, nil
}

// schemas is the element vocabulary, closed to code. Adding a type is a
// deliberate change rather than a value somebody types.
var schemas = map[Type]schema{
	TypeProse: {
		empty:  func() Content { return Prose{} },
		decode: decodeAs[Prose],
	},
	TypeTextSet: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("texts")},
		empty:   func() Content { return TextSet{Texts: []TextItem{}} },
		decode:  decodeAs[TextSet],
	},
	TypeDialogueSample: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("turns")},
		empty:   func() Content { return DialogueSample{Turns: []DialogueTurn{}} },
		decode:  decodeAs[DialogueSample],
	},
	TypeImageSet: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("images")},
		empty:   func() Content { return ImageSet{Images: []ImageItem{}} },
		decode:  decodeAs[ImageSet],
	},
	TypeFieldList: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("fields")},
		empty:   func() Content { return FieldList{Fields: []FieldItem{}} },
		decode:  decodeAs[FieldList],
	},
	TypeLinkList: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("links")},
		empty:   func() Content { return LinkList{Links: []LinkItem{}} },
		decode:  decodeAs[LinkList],
	},
	TypeEntryTable: {
		upgrade: []func(json.RawMessage) (json.RawMessage, error){mintItemIDs("entries")},
		empty:   func() Content { return EntryTable{Entries: []Entry{}} },
		decode:  decodeAs[EntryTable],
	},
	TypePromptList: {
		empty: func() Content {
			return PromptList{Groups: []PromptGroup{}, Fragments: []PromptFragment{}}
		},
		decode: decodeAs[PromptList],
	},
	TypeVariableSchema: {
		empty:  func() Content { return VariableSchema{Variables: []Variable{}} },
		decode: decodeAs[VariableSchema],
	},
	TypeSettingGroup: {
		empty:  func() Content { return SettingGroup{Settings: []Setting{}} },
		decode: decodeAs[SettingGroup],
	},
	TypeScriptList: {
		empty:  func() Content { return ScriptList{Scripts: []Script{}} },
		decode: decodeAs[ScriptList],
	},
	TypeColorSet: {
		empty:  func() Content { return ColorSet{Modes: []ColorMode{}} },
		decode: decodeAs[ColorSet],
	},
	TypeStylesheetSet: {
		empty: func() Content {
			return StylesheetSet{Stylesheets: []Stylesheet{}, Assets: []StylesheetAsset{}}
		},
		decode: decodeAs[StylesheetSet],
	},
	TypeRecordList: {
		empty:  func() Content { return RecordList{Schema: LumiaRecordSchema, Records: []LumiaRecord{}} },
		decode: decodeStoredRecordList,
	},
}
