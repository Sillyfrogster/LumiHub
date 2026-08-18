// Package block holds an asset's content model. A block is a titled container
// and the elements inside it carry the content.
//
// Semantic identity sits on the element and never on the block, so an exporter
// asks for elements by role and takes them wherever the creator put them.
package block

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/google/uuid"
)

// Type is what an element's data structure is. A type exists because its
// structure differs from every other, never because a feature has a name.
type Type string

const (
	TypeProse          Type = "prose"
	TypeTextSet        Type = "text_set"
	TypeFieldList      Type = "field_list"
	TypeDialogueSample Type = "dialogue_sample"
	TypeImageSet       Type = "image_set"
	TypeLinkList       Type = "link_list"
	TypeEntryTable     Type = "entry_table"
)

// Role is what an element's content means, and it is the whole of what import
// and export read. An element with no import or export meaning carries none.
type Role string

const (
	RoleDescription     Role = "description"
	RolePersonality     Role = "personality"
	RoleScenario        Role = "scenario"
	RoleGreetings       Role = "greetings"
	RoleGroupGreetings  Role = "group_greetings"
	RoleExampleDialogue Role = "example_dialogue"
	// RoleSystemPrompt and RolePostHistoryInstructions are prompt text a
	// creator writes for a model rather than for a reader.
	RoleSystemPrompt            Role = "system_prompt"
	RolePostHistoryInstructions Role = "post_history_instructions"
	// RoleCreatorNotes is what the creator wanted to say about making the
	// thing, which is the note every card format carries as creator_notes.
	RoleCreatorNotes Role = "creator_notes"
	RoleGallery      Role = "gallery"
	// RoleExpressions is a named set of pictures of one face. The names are
	// free text and an exporter maps them. Illarin holds no vocabulary of
	// emotions to check them against.
	RoleExpressions Role = "expressions"
	// RoleLorebookEntries is one role for a character's embedded book and for
	// a standalone lorebook alike. It was named for the field CCv2 uses while
	// character was the only kind that had one, and it is named for the
	// content now.
	RoleLorebookEntries Role = "lorebook_entries"
)

// Roles returns the semantic vocabulary in the order a report reads it, which
// is the order the roles are declared above.
func Roles() []Role {
	return []Role{
		RoleDescription, RolePersonality, RoleScenario, RoleGreetings,
		RoleGroupGreetings, RoleExampleDialogue, RoleSystemPrompt,
		RolePostHistoryInstructions, RoleCreatorNotes, RoleGallery,
		RoleExpressions, RoleLorebookEntries,
	}
}

// Known reports whether the role belongs to the shared semantic vocabulary.
func (r Role) Known() bool {
	switch r {
	case RoleDescription, RolePersonality, RoleScenario, RoleGreetings,
		RoleGroupGreetings, RoleExampleDialogue, RoleSystemPrompt,
		RolePostHistoryInstructions, RoleCreatorNotes, RoleGallery,
		RoleExpressions, RoleLorebookEntries:
		return true
	default:
		return false
	}
}

// Cardinality says how many elements one role may have on an asset.
type Cardinality int

const (
	// Singular roles hold list-like data inside one element rather than
	// repeating. Greetings is one element holding an ordered list.
	Singular Cardinality = iota
	// Repeatable roles are presentation content, and export concatenates the
	// repeats in page order.
	Repeatable
)

// Cardinality returns how many elements of this role an asset may carry. An
// unknown role is singular, which is the stricter answer.
func (r Role) Cardinality() Cardinality {
	if r == RoleGallery {
		return Repeatable
	}
	return Singular
}

// Display says whether a text body is authored prose or exact prompt text.
type Display string

const (
	DisplayRich     Display = "rich"
	DisplayVerbatim Display = "verbatim"
)

// Known reports whether the display option belongs to the closed vocabulary.
func (d Display) Known() bool {
	return d == DisplayRich || d == DisplayVerbatim
}

// ItemSize is how large the images inside an element are drawn. It names what
// it controls rather than the element's own geometry, and these three values
// are all an element may say about size.
type ItemSize string

const (
	ItemSmall  ItemSize = "small"
	ItemMedium ItemSize = "medium"
	ItemLarge  ItemSize = "large"
)

// Known reports whether the item size belongs to the closed vocabulary.
func (s ItemSize) Known() bool {
	return s == ItemSmall || s == ItemMedium || s == ItemLarge
}

// ItemSizes returns the sizes an element may draw its images at.
func ItemSizes() []ItemSize { return []ItemSize{ItemSmall, ItemMedium, ItemLarge} }

// Options are an element's presentation choices, each from a closed set.
type Options struct {
	Display  Display  `json:"display,omitempty"`
	ItemSize ItemSize `json:"itemSize,omitempty"`
}

// Slot is the place an element takes in its block's layout.
type Slot string

// Element is one piece of content inside a block. Elements have no rows of
// their own, so a block row carries its whole ordered element list.
type Element struct {
	ID      uuid.UUID
	Type    Type
	Role    Role
	Slot    Slot
	Options Options
	Content Content
}

// Content is the body of one element type. Every type has exactly one
// implementation.
type Content interface {
	// Empty reports whether the element carries nothing a reader would see.
	Empty() bool
}

// Prose is one text body.
type Prose struct {
	Text string `json:"text"`
}

func (p Prose) Empty() bool { return p.Text == "" }

// TextSet is an ordered list of named text bodies.
type TextSet struct {
	Texts []TextItem `json:"texts"`
}

type TextItem struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name,omitempty"`
	Text string    `json:"text"`
}

func (s TextSet) Empty() bool {
	for _, item := range s.Texts {
		if item.Text != "" {
			return false
		}
	}
	return true
}

// DialogueSample is an ordered list of speaker-tagged turns.
type DialogueSample struct {
	Turns []DialogueTurn `json:"turns"`
}

type DialogueTurn struct {
	ID      uuid.UUID `json:"id"`
	Speaker string    `json:"speaker"`
	Text    string    `json:"text"`
}

func (d DialogueSample) Empty() bool { return len(d.Turns) == 0 }

// ImageSet is an ordered list of images, each with an optional free-text name.
// There is no separate caption field, which is what lets an image move between
// a gallery and an expression set.
type ImageSet struct {
	Images []ImageItem `json:"images"`
}

type ImageItem struct {
	ID      uuid.UUID `json:"id"`
	MediaID uuid.UUID `json:"mediaId"`
	Name    string    `json:"name,omitempty"`
}

func (s ImageSet) Empty() bool { return len(s.Images) == 0 }

// FieldList is an ordered list of short named values.
type FieldList struct {
	Fields []FieldItem `json:"fields"`
}

type FieldItem struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name,omitempty"`
	Value string    `json:"value"`
}

func (l FieldList) Empty() bool {
	for _, field := range l.Fields {
		if field.Value != "" {
			return false
		}
	}
	return true
}

// LinkList is an ordered list of web links, each with the wording a reader
// sees and an optional line about why it is there.
type LinkList struct {
	Links []LinkItem `json:"links"`
}

type LinkItem struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label,omitempty"`
	URL   string    `json:"url"`
	Note  string    `json:"note,omitempty"`
}

func (l LinkList) Empty() bool {
	for _, link := range l.Links {
		if link.URL != "" {
			return false
		}
	}
	return true
}

// Empty returns the content an element of this type starts with.
func (t Type) Empty() (Content, error) {
	known, ok := schemas[t]
	if !ok {
		return nil, fmt.Errorf("no element type %q", t)
	}
	return known.empty(), nil
}

// Known reports whether the vocabulary carries this type.
func (t Type) Known() bool {
	_, ok := schemas[t]
	return ok
}

// DecodeContent reads a save request through the schema for its element type.
func DecodeContent(elementType Type, raw json.RawMessage) (Content, error) {
	if !elementType.Known() {
		return nil, fmt.Errorf("no element type %q", elementType)
	}
	switch elementType {
	case TypeProse:
		var incoming struct {
			Text *string `json:"text"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Text == nil {
			return nil, fmt.Errorf("text must be present as a string")
		}
		return Prose{Text: *incoming.Text}, nil
	case TypeTextSet:
		var incoming struct {
			Texts *[]struct {
				ID   uuid.UUID `json:"id,omitempty"`
				Name string    `json:"name,omitempty"`
				Text *string   `json:"text"`
			} `json:"texts"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Texts == nil {
			return nil, fmt.Errorf("texts must be present as a list")
		}
		texts := make([]TextItem, len(*incoming.Texts))
		for i, item := range *incoming.Texts {
			if item.Text == nil {
				return nil, fmt.Errorf("text %d must include text as a string", i+1)
			}
			texts[i] = TextItem{ID: itemID(item.ID), Name: item.Name, Text: *item.Text}
		}
		return TextSet{Texts: texts}, nil
	case TypeDialogueSample:
		var incoming struct {
			Turns *[]struct {
				ID      uuid.UUID `json:"id,omitempty"`
				Speaker *string   `json:"speaker"`
				Text    *string   `json:"text"`
			} `json:"turns"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Turns == nil {
			return nil, fmt.Errorf("turns must be present as a list")
		}
		turns := make([]DialogueTurn, len(*incoming.Turns))
		for i, turn := range *incoming.Turns {
			if turn.Speaker == nil || turn.Text == nil {
				return nil, fmt.Errorf("turn %d must include speaker and text as strings", i+1)
			}
			turns[i] = DialogueTurn{ID: itemID(turn.ID), Speaker: *turn.Speaker, Text: *turn.Text}
		}
		return DialogueSample{Turns: turns}, nil
	case TypeFieldList:
		var incoming struct {
			Fields *[]struct {
				ID    uuid.UUID `json:"id,omitempty"`
				Name  string    `json:"name,omitempty"`
				Value *string   `json:"value"`
			} `json:"fields"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Fields == nil {
			return nil, fmt.Errorf("fields must be present as a list")
		}
		fields := make([]FieldItem, len(*incoming.Fields))
		for i, field := range *incoming.Fields {
			if field.Value == nil {
				return nil, fmt.Errorf("field %d must include value as a string", i+1)
			}
			fields[i] = FieldItem{ID: itemID(field.ID), Name: field.Name, Value: *field.Value}
		}
		return FieldList{Fields: fields}, nil
	case TypeLinkList:
		var incoming struct {
			Links *[]LinkItem `json:"links"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Links == nil {
			return nil, fmt.Errorf("links must be present as a list")
		}
		links := *incoming.Links
		for i := range links {
			if err := checkWebAddress(links[i].URL); err != nil {
				return nil, fmt.Errorf("link %d: %w", i+1, err)
			}
			links[i].ID = itemID(links[i].ID)
		}
		return LinkList{Links: links}, nil
	case TypeEntryTable:
		return decodeEntryTable(raw)
	case TypeImageSet:
		var incoming struct {
			Images *[]ImageItem `json:"images"`
		}
		if err := decodeContentJSON(raw, &incoming); err != nil {
			return nil, err
		}
		if incoming.Images == nil {
			return nil, fmt.Errorf("images must be present as a list")
		}
		images := *incoming.Images
		for i := range images {
			if images[i].MediaID == uuid.Nil {
				return nil, fmt.Errorf("image %d must include a media id", i+1)
			}
			images[i].ID = itemID(images[i].ID)
		}
		return ImageSet{Images: images}, nil
	default:
		return nil, fmt.Errorf("no element type %q", elementType)
	}
}

// checkWebAddress takes http and https and refuses everything else, because
// any other scheme is a way to run code from a page a reader trusts.
func checkWebAddress(address string) error {
	parsed, err := url.Parse(address)
	if err != nil {
		return fmt.Errorf("%q is not an address", address)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("a link must start with http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("a link must name a site")
	}
	return nil
}

func decodeContentJSON(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("content must be one JSON value")
	}
	return nil
}

// wireElement is how an element is stored and served. The version belongs to
// the content, so a writer cannot forget to stamp it.
type wireElement struct {
	ID      uuid.UUID       `json:"id"`
	Type    Type            `json:"type"`
	Role    Role            `json:"role,omitempty"`
	Slot    Slot            `json:"slot"`
	Version int             `json:"version"`
	Options Options         `json:"options"`
	Content json.RawMessage `json:"content"`
}

func (e Element) MarshalJSON() ([]byte, error) {
	known, ok := schemas[e.Type]
	if !ok {
		return nil, fmt.Errorf("no element type %q", e.Type)
	}
	content, err := json.Marshal(e.Content)
	if err != nil {
		return nil, fmt.Errorf("write %s content: %w", e.Type, err)
	}
	return json.Marshal(wireElement{
		ID: e.ID, Type: e.Type, Role: e.Role, Slot: e.Slot,
		Version: known.version(), Options: e.Options, Content: content,
	})
}

// ContentJSON returns the element's body on its own.
func (e Element) ContentJSON() (json.RawMessage, error) {
	body, err := json.Marshal(e.Content)
	if err != nil {
		return nil, fmt.Errorf("write %s content: %w", e.Type, err)
	}
	return body, nil
}

func (e *Element) UnmarshalJSON(data []byte) error {
	var stored wireElement
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	known, ok := schemas[stored.Type]
	if !ok {
		return fmt.Errorf("no element type %q", stored.Type)
	}
	content, err := known.read(stored.Version, stored.Content)
	if err != nil {
		return fmt.Errorf("read %s content: %w", stored.Type, err)
	}
	*e = Element{
		ID: stored.ID, Type: stored.Type, Role: stored.Role, Slot: stored.Slot,
		Options: stored.Options, Content: content,
	}
	return nil
}

// labels is the wording a role carries on the page. It sits on the role
// because it stays the same wherever a creator moves the element.
var labels = map[Role]string{
	RoleDescription:     "Description",
	RolePersonality:     "Personality",
	RoleScenario:        "Scenario",
	RoleGreetings:       "Greetings",
	RoleGroupGreetings:  "Group-only greetings",
	RoleExampleDialogue: "Example dialogue",
	RoleGallery:         "Images",

	RoleSystemPrompt:            "System prompt",
	RolePostHistoryInstructions: "Post-history instructions",
	RoleCreatorNotes:            "Author’s notes",
	RoleExpressions:             "Expressions",
	RoleLorebookEntries:         "Entries",
}

// typeLabels name an element that carries no role, so a removal confirmation
// can still say what a creator is about to lose.
var typeLabels = map[Type]string{
	TypeProse:          "Text",
	TypeTextSet:        "List",
	TypeFieldList:      "Details",
	TypeDialogueSample: "Dialogue",
	TypeImageSet:       "Images",
	TypeLinkList:       "Links",
	TypeEntryTable:     "Entries",
}

// Label returns the element's wording, from its role where it has one and from
// its type where it does not.
func (e Element) Label() string {
	if label := e.Role.Label(); label != "" {
		return label
	}
	return typeLabels[e.Type]
}

var roleTypes = map[Role][]Type{
	RoleDescription:     {TypeProse},
	RolePersonality:     {TypeProse},
	RoleScenario:        {TypeProse},
	RoleGreetings:       {TypeTextSet},
	RoleGroupGreetings:  {TypeTextSet},
	RoleExampleDialogue: {TypeDialogueSample},
	RoleGallery:         {TypeImageSet},

	RoleSystemPrompt:            {TypeProse},
	RolePostHistoryInstructions: {TypeProse},
	RoleCreatorNotes:            {TypeProse},
	RoleExpressions:             {TypeImageSet},
	RoleLorebookEntries:         {TypeEntryTable},
}

// Label returns the role's wording on the page.
func (r Role) Label() string { return labels[r] }

// Allows reports whether this role may attach to an element type.
func (r Role) Allows(elementType Type) bool {
	allowed, ok := roleTypes[r]
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if candidate == elementType {
			return true
		}
	}
	return false
}

// AllowedTypes returns the element types this role may attach to.
func (r Role) AllowedTypes() []Type { return roleTypes[r] }
