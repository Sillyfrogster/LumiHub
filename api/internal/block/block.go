package block

import "github.com/google/uuid"

// Block is a titled container on an asset's page. It carries how its content is
// presented and nothing about what that content means, and it stores nothing
// derived from its elements.
//
// A block never holds another block. Complex content is structured data inside
// one element, where its schema is known.
type Block struct {
	ID uuid.UUID
	// Definition names the block's entry in its kind's catalog, which is where
	// everything about how the block behaves is read from.
	Definition DefinitionID
	// Title is nil where the creator has not written one, and the definition's
	// current default stands in.
	Title    *string
	Position int
	Hidden   bool
	Layout   Layout
	Width    Width
	Elements []Element
}

// Empty reports whether every element in the block carries nothing. A required
// block may sit empty from the moment the asset exists. An optional one is
// absent instead.
func (b Block) Empty() bool {
	for _, element := range b.Elements {
		if element.Content != nil && !element.Content.Empty() {
			return false
		}
	}
	return true
}
