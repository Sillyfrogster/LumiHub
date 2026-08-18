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

	"github.com/google/uuid"
)

// Type is what an element's data structure is. A type exists because its
// structure differs from every other, never because a feature has a name.
type Type string

const (
	TypeProse          Type = "prose"
	TypeTextSet        Type = "text_set"
	TypeDialogueSample Type = "dialogue_sample"
	TypeImageSet       Type = "image_set"
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
	RoleGallery         Role = "gallery"
)

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

// Options are an element's presentation choices, each from a closed set.
type Options struct {
	Display Display `json:"display,omitempty"`
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
	Name string `json:"name,omitempty"`
	Text string `json:"text"`
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
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

func (d DialogueSample) Empty() bool { return len(d.Turns) == 0 }

// ImageSet is an ordered list of images, each with an optional free-text name.
// There is no separate caption field, which is what lets an image move between
// a gallery and an expression set.
type ImageSet struct {
	Images []ImageItem `json:"images"`
}

type ImageItem struct {
	MediaID uuid.UUID `json:"mediaId"`
	Name    string    `json:"name,omitempty"`
}

func (s ImageSet) Empty() bool { return len(s.Images) == 0 }

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
				Name string  `json:"name,omitempty"`
				Text *string `json:"text"`
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
			texts[i] = TextItem{Name: item.Name, Text: *item.Text}
		}
		return TextSet{Texts: texts}, nil
	case TypeDialogueSample:
		var incoming struct {
			Turns *[]struct {
				Speaker *string `json:"speaker"`
				Text    *string `json:"text"`
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
			turns[i] = DialogueTurn{Speaker: *turn.Speaker, Text: *turn.Text}
		}
		return DialogueSample{Turns: turns}, nil
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
		for i, image := range *incoming.Images {
			if image.MediaID == uuid.Nil {
				return nil, fmt.Errorf("image %d must include a media id", i+1)
			}
		}
		return ImageSet{Images: *incoming.Images}, nil
	default:
		return nil, fmt.Errorf("no element type %q", elementType)
	}
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
}

var roleTypes = map[Role][]Type{
	RoleDescription:     {TypeProse},
	RolePersonality:     {TypeProse},
	RoleScenario:        {TypeProse},
	RoleGreetings:       {TypeTextSet},
	RoleGroupGreetings:  {TypeTextSet},
	RoleExampleDialogue: {TypeDialogueSample},
	RoleGallery:         {TypeImageSet},
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
