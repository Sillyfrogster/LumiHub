package block

import "slices"

// DefinitionID names a block's entry in a kind catalog.
type DefinitionID string

const (
	CharacterCore     DefinitionID = "character_core"
	LorebookCore      DefinitionID = "lorebook_core"
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
	CustomSection     DefinitionID = "custom_section"
)

// Group is where a block's content ends up. The add tray groups by it,
// because a creator knows the destination before they know the name.
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

// Definition is what a kind declares about one of its blocks. Every field here
// is read at render time and none of it is copied onto a block row, so changing
// what a character requires is a code change rather than a migration that
// leaves a stale answer wherever it is missed.
type Definition struct {
	ID DefinitionID
	// Title is the wording a block carries until its creator writes their own.
	Title string
	// Required blocks exist from the moment the asset does and cannot be
	// removed. Only a required definition says whether it may be hidden,
	// because an optional block can always be removed outright and forbidding
	// the gentler action would be incoherent.
	Required bool
	Hideable bool
	// Elements are the definition's own, in slot order.
	Elements []DefinedElement
	// Layouts are in preference order, and placement takes the first with room
	// for the elements it placed.
	Layouts []Layout
	// Width is what stops an imported asset arriving as a stack of identical
	// full-width cards.
	Width Width
	// Summary is the line the add tray carries under the definition's name.
	Summary string
	// Group is where this block's content ends up.
	Group Group
	// Repeatable definitions may sit on a page more than once.
	Repeatable bool
	// Choices are the elements a creator may start the block with. A
	// definition that names none starts with the elements it declares.
	Choices []DefinedElement
}

// start is one way a section can arrive. It is the elements the section holds
// on the day it is added, and the name the add tray offers it under.
//
// A definition that names choices has one start per choice. Every other
// definition has exactly one, its own declared elements, because there is no
// route that adds an element to a section later.
type start struct {
	Type     Type
	Label    string
	Elements []DefinedElement
}

// starts returns the ways a creator may start this block, in the order the
// tray offers them.
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
	Role Role
	Type Type
	// Options are the definition's presentation choices for this element.
	Options Options
	// A pinned element exists from the moment the asset does and can be neither
	// removed nor moved, so a required block can never be hollowed out. An
	// unpinned one is placed only where a source carries it.
	Pinned bool
}

// character is the character catalog, in page order.
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
			// Group-only greetings fire in group chats alone, so they stay a
			// separate role and appear only where a source carries them.
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
		// A prompt is text and its settings are named values, so the section
		// needs no type of its own for either.
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
		// Half width has no room for duo, so a creator who wants the two
		// prompts side by side widens the section first.
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

// lorebook is the lorebook catalog. A book is its entries, so the kind has one
// required section and nothing else of its own.
var lorebook = []Definition{
	{
		ID:       LorebookCore,
		Title:    "Entries",
		Required: true,
		// A lorebook with its entries withheld is not a page.
		Hideable: false,
		Elements: []DefinedElement{
			{Role: RoleLorebookEntries, Type: TypeEntryTable, Pinned: true},
		},
		Layouts: []Layout{Single},
		Width:   Full,
	},
}

// shared is the seven definitions every kind lists. They hold the parts no
// file format carries, whatever a creator is building.
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
		// Creator notes are what a card format carries, so this is where the
		// role binds. A kind whose formats carry none never fills it.
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
		ID:         CustomSection,
		Title:      "New section",
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

// catalogs holds one catalog per kind. A kind with no catalog cannot be built
// yet, and creation refuses it rather than offering an empty page.
var catalogs = map[string][]Definition{
	"character": character,
	"lorebook":  lorebook,
}

// Catalog returns the block definitions a kind declares, in page order. Every
// kind lists its own and then the seven shared ones.
func Catalog(kind string) ([]Definition, bool) {
	own, ok := catalogs[kind]
	if !ok {
		return nil, false
	}
	return slices.Concat(own, shared), true
}

// Kinds returns every kind that has a catalog, so a creator is only offered
// what Illarin can actually build.
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

// element returns the definition's entry for a role.
func (d Definition) element(role Role) (DefinedElement, bool) {
	for _, defined := range d.Elements {
		if defined.Role == role {
			return defined, true
		}
	}
	return DefinedElement{}, false
}

// layoutFor returns the first allowed layout with room for count elements.
func (d Definition) layoutFor(count int) (Layout, bool) {
	for _, layout := range d.Layouts {
		if len(layout.Slots()) >= count {
			return layout, true
		}
	}
	return "", false
}
