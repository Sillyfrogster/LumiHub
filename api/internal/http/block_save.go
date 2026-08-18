package http

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) SaveAssetBlock(c *gin.Context, id types.UUID, blockID types.UUID) {
	owner, ok := h.verifiedAccount(c, "saving an asset")
	if !ok {
		return
	}
	var request SaveAssetBlockRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the block as JSON."})
		return
	}
	update, err := blockUpdate(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	saved, err := h.assets.SaveBlock(
		c.Request.Context(), owner.ID, uuid.UUID(id), uuid.UUID(blockID), update,
	)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such block."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrInvalidBlock):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the block."})
	default:
		blocks, conversionErr := toAPIBlocks(saved.Kind, []block.Block{saved.Block})
		if conversionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the saved block."})
			return
		}
		c.JSON(http.StatusOK, blocks[0])
	}
}

func blockUpdate(request SaveAssetBlockRequest) (asset.BlockUpdate, error) {
	elements := make([]block.Element, len(request.Elements))
	for i, incoming := range request.Elements {
		elementType := block.Type(incoming.Type)
		content, err := block.DecodeContent(elementType, incoming.Content)
		if err != nil {
			label := block.Role("")
			if incoming.Role != nil {
				label = block.Role(*incoming.Role)
			}
			name := label.Label()
			if name == "" {
				name = fmt.Sprintf("Element %d", i+1)
			}
			return asset.BlockUpdate{}, fmt.Errorf("%s content is malformed: %w", name, err)
		}
		role := block.Role("")
		if incoming.Role != nil {
			role = block.Role(*incoming.Role)
		}
		display := block.Display("")
		if incoming.Display != nil {
			display = block.Display(*incoming.Display)
		}
		elements[i] = block.Element{
			ID: uuid.UUID(incoming.Id), Type: elementType, Role: role,
			Slot: block.Slot(incoming.Slot), Options: block.Options{Display: display},
			Content: content,
		}
	}
	return asset.BlockUpdate{
		Title: request.Title, Layout: block.Layout(request.Layout),
		Width: block.Width(request.Width), Hidden: request.Hidden, Elements: elements,
	}, nil
}
