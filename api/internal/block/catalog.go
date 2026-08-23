package block

import "slices"

// DefinitionID names a block's entry in a kind catalog.
type DefinitionID string

const (
	CharacterCore     DefinitionID = "character_core"
	LorebookCore      DefinitionID = "lorebook_core"
	PresetCore        DefinitionID = "preset_core"
	PresetSettings    DefinitionID = "settings"
	PresetVariables   DefinitionID = "variables"
	PresetScripts     DefinitionID = "scripts"
	PresetNudges      DefinitionID = "nudges"
	ThemeCore         DefinitionID = "theme_core"
	ThemeStylesheet   DefinitionID = "stylesheet"
	PackCore          DefinitionID = "pack_core"
	Messages          DefinitionID = "messages"
	Expressions       DefinitionID = "expressions"
	Lorebook          DefinitionID = "lorebook"
	ImagePrompts      DefinitionID = "image_prompts"
	ModelInstructions DefinitionID = "model_instructions"
	Relationships     DefinitionID = "relationships"
	Gallery           DefinitionID = "gallery"
	Usage             DefinitionID = "usage"
	Changelog         DefinitionID = "changelog"
	Attributes        DefinitionID = "attributes"
	AuthorNotes       DefinitionID = "author_notes"
	RunsBestWith      DefinitionID = "runs_best_with"
	CustomBlock       DefinitionID = "custom_block"
)

// Group identifies where a block's content belongs.
type Group string

const (
	GroupFile   Group = "file"
	GroupReader Group = "reader"
	GroupWork   Group = "work"
	GroupOther  Group = "other"
)

var groupTitles = map[Group]string{
	GroupFile:   "Content that travels with the file",
	GroupReader: "Things a reader sees",
	GroupWork:   "About the work",
	GroupOther:  "Anything else",
}

// Groups returns the add tray's groups in the order a creator reads them.
func Groups() []Group { return []Group{GroupFile, GroupReader, GroupWork, GroupOther} }

// Title returns the group's wording in the add tray.
func (g Group) Title() string { return groupTitles[g] }

// Definition describes a kind's block at render time.
type Definition struct {
	ID       DefinitionID
	Title    string
	Required bool
	Hideable bool
	Elements []DefinedElement
	// Layouts are in preference order.
	Layouts    []Layout
	Width      Width
	Summary    string
	Group      Group
	Repeatable bool
	// Choices override the starting elements in the add tray.
	Choices []DefinedElement
}

type start struct {
	Type     Type
	Label    string
	Elements []DefinedElement
}

func (d Definition) starts() []start {
	if len(d.Choices) > 0 {
		starts := make([]start, 0, len(d.Choices))
		for _, choice := range d.Choices {
			starts = append(starts, start{
				Type:     choice.Type,
				Label:    typeLabels[choice.Type],
				Elements: []DefinedElement{choice},
			})
		}
		return starts
	}
	if len(d.Elements) == 0 {
		return nil
	}
	return []start{{Type: d.Elements[0].Type, Label: d.Title, Elements: d.Elements}}
}

// DefinedElement is one element a definition places.
type DefinedElement struct {
	Role    Role
	Type    Type
	Options Options
	// Pinned elements cannot be removed or moved.
	Pinned bool
}

var character = []Definition{
	{
		ID:       CharacterCore,
		Title:    "The character",
		Required: true,
		Hideable: true,
		Elements: []DefinedElement{
			{Role: RoleDescription, Type: TypeProse, Options: Options{Display: DisplayRich}, Pinned: true},
			{Role: RolePersonality, Type: TypeProse, Options: Options{Display: DisplayRich}, Pinned: true},
			{Role: RoleScenario, Type: TypeProse, Options: Options{Display: DisplayRich}, Pinned: true},
		},
		Layouts: []Layout{Stack3, Trio},
		Width:   TwoThirds,
	},
	{
		ID:       Messages,
		Title:    "Messages",
		Required: true,
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RoleGreetings, Type: TypeTextSet, Options: Options{Display: DisplayRich}, Pinned: true},
			{Role: RoleExampleDialogue, Type: TypeDialogueSample, Pinned: true},
			{Role: RoleGroupGreetings, Type: TypeTextSet, Options: Options{Display: DisplayRich}},
		},
		Layouts: []Layout{Stack2, Stack3},
		Width:   Full,
	},
	{
		ID:      Expressions,
		Title:   "Expressions",
		Summary: "A picture per expression, each named as the source named it.",
		Group:   GroupFile,
		Elements: []DefinedElement{
			{Role: RoleExpressions, Type: TypeImageSet, Options: Options{ItemSize: ItemSmall}},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
	{
		ID:       Lorebook,
		Title:    "Lorebook",
		Summary:  "Entries a model reads once one of their key words turns up.",
		Group:    GroupFile,
		Elements: []DefinedElement{{Role: RoleLorebookEntries, Type: TypeEntryTable}},
		Layouts:  []Layout{Single},
		Width:    TwoThirds,
	},
	{
		ID:      ImagePrompts,
		Title:   "Image prompts",
		Summary: "The settings and the prompt text the artwork came from.",
		Group:   GroupWork,
		Elements: []DefinedElement{
			{Type: TypeFieldList},
			{Type: TypeProse, Options: Options{Display: DisplayVerbatim}},
			{Type: TypeProse, Options: Options{Display: DisplayVerbatim}},
		},
		Layouts: []Layout{Stack3},
		Width:   TwoThirds,
	},
	{
		ID:      ModelInstructions,
		Title:   "Model instructions",
		Summary: "The system prompt, and the note that goes after the history.",
		Group:   GroupFile,
		Elements: []DefinedElement{
			{Role: RoleSystemPrompt, Type: TypeProse, Options: Options{Display: DisplayVerbatim}},
			{Role: RolePostHistoryInstructions, Type: TypeProse, Options: Options{Display: DisplayVerbatim}},
		},
		Layouts: []Layout{Stack2, Duo},
		Width:   Half,
	},
	{
		ID:       Relationships,
		Title:    "Relationships",
		Summary:  "Links to the work this character belongs beside.",
		Group:    GroupReader,
		Elements: []DefinedElement{{Type: TypeLinkList}},
		Layouts:  []Layout{Single},
		Width:    Third,
	},
}

var lorebook = []Definition{
	{
		ID:       LorebookCore,
		Title:    "Entries",
		Required: true,
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RoleLorebookEntries, Type: TypeEntryTable, Pinned: true},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
}

var preset = []Definition{
	{
		ID:       PresetCore,
		Title:    "Prompt fragments",
		Required: true,
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RolePromptFragments, Type: TypePromptList, Pinned: true},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
	{
		ID:      PresetSettings,
		Title:   "Settings",
		Summary: "Samplers, completion behaviour, and the advanced settings.",
		Group:   GroupFile,
		Elements: []DefinedElement{
			{Role: RoleSamplerSettings, Type: TypeSettingGroup},
			{Role: RoleCompletionSettings, Type: TypeSettingGroup},
			{Role: RoleAdvancedSettings, Type: TypeSettingGroup},
		},
		Layouts: []Layout{Trio, Stack3},
		Width:   Full,
	},
	{
		ID:       PresetVariables,
		Title:    "Variables",
		Summary:  "The form a reader fills in before they use the preset.",
		Group:    GroupFile,
		Elements: []DefinedElement{{Role: RolePromptVariables, Type: TypeVariableSchema}},
		Layouts:  []Layout{Single},
		Width:    TwoThirds,
	},
	{
		ID:       PresetScripts,
		Title:    "Regex scripts",
		Summary:  "Find and replace over what is written and what comes back.",
		Group:    GroupFile,
		Elements: []DefinedElement{{Role: RoleRegexScripts, Type: TypeScriptList}},
		Layouts:  []Layout{Single},
		Width:    TwoThirds,
	},
	{
		ID:      PresetNudges,
		Title:   "Nudges",
		Summary: "The short prompts an app sends on its own, and the formats it wraps.",
		Group:   GroupFile,
		Elements: []DefinedElement{
			{Role: RolePromptNudges, Type: TypeTextSet, Options: Options{Display: DisplayVerbatim}},
		},
		Layouts: []Layout{Single},
		Width:   Third,
	},
}

var theme = []Definition{
	{
		ID:       ThemeCore,
		Title:    "Palette",
		Required: true,
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RoleThemeTokens, Type: TypeColorSet, Pinned: true},
			{Role: RoleThemeControls, Type: TypeSettingGroup, Pinned: true},
		},
		Layouts: []Layout{Duo, Stack2},
		Width:   Full,
	},
	{
		ID:       ThemeStylesheet,
		Title:    "Stylesheets",
		Required: true,
		Hideable: true,
		Elements: []DefinedElement{
			{Role: RoleStylesheets, Type: TypeStylesheetSet, Pinned: true},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
}

var pack = []Definition{
	{
		ID:       PackCore,
		Title:    "Pack items",
		Required: true,
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RolePackItems, Type: TypeRecordList, Pinned: true},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
}

var shared = []Definition{
	{
		ID:       Gallery,
		Title:    "Gallery",
		Summary:  "Images of the work, kept together and carried in a download.",
		Group:    GroupFile,
		Elements: []DefinedElement{{Role: RoleGallery, Type: TypeImageSet, Options: Options{ItemSize: ItemMedium}}},
		Layouts:  []Layout{Single},
		Width:    Half,
	},
	{
		ID:       Usage,
		Title:    "How to use this",
		Summary:  "A note on how to run it and what it expects.",
		Group:    GroupReader,
		Elements: []DefinedElement{{Type: TypeProse, Options: Options{Display: DisplayRich}}},
		Layouts:  []Layout{Single},
		Width:    Half,
	},
	{
		ID:       Changelog,
		Title:    "Changelog",
		Summary:  "What changed, one entry at a time.",
		Group:    GroupWork,
		Elements: []DefinedElement{{Type: TypeTextSet, Options: Options{Display: DisplayRich}}},
		Layouts:  []Layout{Single},
		Width:    Half,
	},
	{
		ID:       Attributes,
		Title:    "Attributes",
		Summary:  "Short facts as a grid, each one a name and a value.",
		Group:    GroupReader,
		Elements: []DefinedElement{{Type: TypeFieldList}},
		Layouts:  []Layout{Single},
		Width:    Third,
	},
	{
		ID:      AuthorNotes,
		Title:   "Author’s notes",
		Summary: "What you want to say about making it.",
		Group:   GroupWork,
		Elements: []DefinedElement{
			{Role: RoleCreatorNotes, Type: TypeProse, Options: Options{Display: DisplayRich}},
		},
		Layouts: []Layout{Single},
		Width:   Third,
	},
	{
		ID:       RunsBestWith,
		Title:    "Runs best with",
		Summary:  "Links to the work a reader should pair this with.",
		Group:    GroupReader,
		Elements: []DefinedElement{{Type: TypeLinkList}},
		Layouts:  []Layout{Single},
		Width:    Third,
	},
	{
		ID:         CustomBlock,
		Title:      "New block",
		Summary:    "A heading you write, with text, images, links or a list under it.",
		Group:      GroupOther,
		Repeatable: true,
		Layouts:    []Layout{Single, Duo, MainAside, Trio, Stack2, Stack3},
		Width:      Full,
		Choices: []DefinedElement{
			{Type: TypeProse, Options: Options{Display: DisplayRich}},
			{Type: TypeImageSet, Options: Options{ItemSize: ItemMedium}},
			{Type: TypeLinkList},
			{Type: TypeTextSet, Options: Options{Display: DisplayRich}},
		},
	},
}

var catalogs = map[string][]Definition{
	"character": character,
	"lorebook":  lorebook,
	"preset":    preset,
	"theme":     theme,
	"pack":      pack,
}

// Catalog returns a kind's block definitions in page order.
func Catalog(kind string) ([]Definition, bool) {
	own, ok := catalogs[kind]
	if !ok {
		return nil, false
	}
	return slices.Concat(own, shared), true
}

// Kinds returns every kind that has a catalog.
func Kinds() []string {
	kinds := make([]string, 0, len(catalogs))
	for kind := range catalogs {
		kinds = append(kinds, kind)
	}
	return kinds
}

// Definition returns one kind's entry for a definition id.
func (id DefinitionID) Definition(kind string) (Definition, bool) {
	definitions, ok := Catalog(kind)
	if !ok {
		return Definition{}, false
	}
	for _, definition := range definitions {
		if definition.ID == id {
			return definition, true
		}
	}
	return Definition{}, false
}

func (d Definition) element(role Role) (DefinedElement, bool) {
	for _, defined := range d.Elements {
		if defined.Role == role {
			return defined, true
		}
	}
	return DefinedElement{}, false
}

func (d Definition) layoutFor(count int) (Layout, bool) {
	for _, layout := range d.Layouts {
		if len(layout.Slots()) >= count {
			return layout, true
		}
	}
	return "", false
}
