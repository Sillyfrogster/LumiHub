package block

import (
	"fmt"

	"github.com/google/uuid"
)

// Place arranges role-tagged elements into the blocks a kind's catalog
// declares. Import and migration both call it, so a file and a database row
// land on the same page.
//
// A required block is returned whether or not anything filled it, because a
// creator has to see what the kind is asking of them. An optional block is
// returned only where something fills it, so it is either absent or populated
// and never present and empty.
func Place(kind string, tagged []Element) ([]Block, error) {
	definitions, ok := Catalog(kind)
	if !ok {
		return nil, fmt.Errorf("no block catalog for kind %q", kind)
	}

	placed := make([]bool, len(tagged))
	blocks := make([]Block, 0, len(definitions))
	for _, definition := range definitions {
		elements, err := definition.fill(tagged, placed)
		if err != nil {
			return nil, err
		}
		if len(elements) == 0 && !definition.Required {
			continue
		}
		layout, ok := definition.layoutFor(len(elements))
		if !ok {
			return nil, fmt.Errorf(
				"%s has %d elements and no layout with room for them",
				definition.ID, len(elements),
			)
		}
		for i := range elements {
			elements[i].Slot = layout.Slots()[i]
		}
		blocks = append(blocks, Block{
			ID:         uuid.New(),
			Definition: definition.ID,
			Position:   len(blocks),
			Layout:     layout,
			Width:      definition.Width,
			Elements:   elements,
		})
	}

	for i, element := range tagged {
		if !placed[i] {
			return nil, fmt.Errorf(
				"kind %q has nowhere to put a %s element", kind, element.Role,
			)
		}
	}
	return blocks, nil
}

// fill takes the definition's elements from what a source supplied, marking
// what it took. A pinned element the source did not supply is created empty.
func (d Definition) fill(tagged []Element, placed []bool) ([]Element, error) {
	elements := make([]Element, 0, len(d.Elements))
	for _, defined := range d.Elements {
		found, err := d.take(defined.Role, tagged, placed)
		if err != nil {
			return nil, err
		}
		if found < 0 {
			if !defined.Pinned {
				continue
			}
			content, err := defined.Type.Empty()
			if err != nil {
				return nil, err
			}
			elements = append(elements, Element{
				ID: uuid.New(), Type: defined.Type, Role: defined.Role,
				Options: defined.Options, Content: content,
			})
			continue
		}
		element := tagged[found]
		if element.Type != defined.Type {
			return nil, fmt.Errorf(
				"%s carries a %s element and %s takes a %s",
				defined.Role, element.Type, d.ID, defined.Type,
			)
		}
		placed[found] = true
		if element.ID == uuid.Nil {
			element.ID = uuid.New()
		}
		// Presentation is the definition's to declare, not the source file's.
		element.Options = defined.Options
		elements = append(elements, element)
	}
	return elements, nil
}

// take finds the one unplaced element carrying a role, or -1 for none. The
// definition has one place for it, so a second is refused rather than dropped.
func (d Definition) take(role Role, tagged []Element, placed []bool) (int, error) {
	found := -1
	for i, candidate := range tagged {
		if placed[i] || candidate.Role != role {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf(
				"%s carries more than one %s element and the block has one place for it",
				d.ID, role,
			)
		}
		found = i
	}
	return found, nil
}

// Pinned reports whether an element in this block can be neither removed nor
// moved. It is read from the catalog rather than from the row.
func (b Block) Pinned(role Role, kind string) bool {
	definition, ok := b.Definition.Definition(kind)
	if !ok {
		return false
	}
	defined, ok := definition.element(role)
	return ok && defined.Pinned
}
