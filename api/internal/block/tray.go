package block

import (
	"fmt"

	"github.com/google/uuid"
)

// Offer is one section the add tray shows.
type Offer struct {
	Definition DefinitionID
	Title      string
	Summary    string
	Group      Group
	Repeatable bool
	// Choices are the elements a creator may start the section with. One
	// choice needs no question asking.
	Choices []Choice
}

// Choice is one element a section can start with.
type Choice struct {
	Type  Type
	Label string
}

// Offers returns every section a creator can add to a kind, in catalog order.
// A required section is never among them, because it is on the page already.
func Offers(kind string) ([]Offer, bool) {
	definitions, ok := Catalog(kind)
	if !ok {
		return nil, false
	}
	offers := make([]Offer, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Required {
			continue
		}
		choices := make([]Choice, 0, len(definition.choices()))
		for _, defined := range definition.choices() {
			choices = append(choices, Choice{
				Type:  defined.Type,
				Label: typeLabels[defined.Type],
			})
		}
		offers = append(offers, Offer{
			Definition: definition.ID,
			Title:      definition.Title,
			Summary:    definition.Summary,
			Group:      definition.Group,
			Repeatable: definition.Repeatable,
			Choices:    choices,
		})
	}
	return offers, true
}

// NewBlock builds the section a creator asked the add tray for. It goes at the
// foot of the page holding the one element they chose.
func NewBlock(kind string, id DefinitionID, elementType Type, page []Block) (Block, error) {
	definition, ok := id.Definition(kind)
	if !ok {
		return Block{}, fmt.Errorf("a %s has no %s section", kind, id)
	}
	if definition.Required {
		return Block{}, fmt.Errorf("%s is on every %s already", definition.Title, kind)
	}
	if !definition.Repeatable {
		for _, holder := range page {
			if holder.Definition == id {
				return Block{}, fmt.Errorf("%s is already on this page", definition.Title)
			}
		}
	}
	var chosen DefinedElement
	found := false
	for _, candidate := range definition.choices() {
		if candidate.Type == elementType {
			chosen, found = candidate, true
			break
		}
	}
	if !found {
		return Block{}, fmt.Errorf("%s does not hold %s content", definition.Title, elementType)
	}
	layout, ok := definition.layoutFor(1)
	if !ok {
		return Block{}, fmt.Errorf("%s has no layout with room for one element", id)
	}
	content, err := chosen.Type.Empty()
	if err != nil {
		return Block{}, err
	}
	return Block{
		ID:         uuid.New(),
		Definition: id,
		Position:   len(page),
		Layout:     layout,
		Width:      definition.Width,
		Elements: []Element{{
			ID:      uuid.New(),
			Type:    chosen.Type,
			Role:    chosen.Role,
			Slot:    layout.Slots()[0],
			Options: chosen.Options,
			Content: content,
		}},
	}, nil
}
