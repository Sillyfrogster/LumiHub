package http

import (
	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/oapi-codegen/runtime/types"
)

// toAPIBlocks serves an asset's blocks with what the kind catalog says about
// them today, so changing what a kind asks for reaches every asset at once.
func toAPIBlocks(kind string, blocks []block.Block) ([]AssetBlock, error) {
	out := make([]AssetBlock, 0, len(blocks))
	for _, b := range blocks {
		definition, _ := b.Definition.Definition(kind)
		title, isDefault := definition.Title, true
		if b.Title != nil {
			title, isDefault = *b.Title, false
		}
		elements, err := toAPIElements(kind, b)
		if err != nil {
			return nil, err
		}
		out = append(out, AssetBlock{
			Id:             types.UUID(b.ID),
			Definition:     string(b.Definition),
			Title:          title,
			TitleIsDefault: isDefault,
			Position:       b.Position,
			Hidden:         b.Hidden,
			Layout:         AssetBlockLayout(b.Layout),
			Width:          AssetBlockWidth(b.Width),
			Required:       definition.Required,
			// Every optional block is hideable, because it can be removed.
			Hideable: !definition.Required || definition.Hideable,
			IsEmpty:  b.Empty(),
			Elements: elements,
		})
	}
	return out, nil
}

func toAPIElements(kind string, holder block.Block) ([]AssetElement, error) {
	out := make([]AssetElement, 0, len(holder.Elements))
	for _, element := range holder.Elements {
		content, err := element.ContentJSON()
		if err != nil {
			return nil, err
		}
		served := AssetElement{
			Id:      types.UUID(element.ID),
			Type:    AssetElementType(element.Type),
			Slot:    string(element.Slot),
			Label:   element.Role.Label(),
			Pinned:  holder.Pinned(element.Role, kind),
			IsEmpty: element.Content == nil || element.Content.Empty(),
			Content: content,
		}
		if element.Role != "" {
			role := string(element.Role)
			served.Role = &role
		}
		if element.Options.Display != "" {
			display := AssetElementDisplay(element.Options.Display)
			served.Display = &display
		}
		out = append(out, served)
	}
	return out, nil
}
