package http

import (
	"errors"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) AddAssetBlock(c *gin.Context, id types.UUID) {
	owner, ok := h.verifiedAccount(c, "adding a block")
	if !ok {
		return
	}
	var request AddAssetBlockRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name the block to add and the element it starts with."})
		return
	}
	saved, err := h.assets.AddBlock(
		c.Request.Context(), owner.ID, uuid.UUID(id),
		block.DefinitionID(request.Definition), block.Type(request.ElementType),
	)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such asset."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrInvalidBlock):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not add the block."})
	default:
		blocks, conversionErr := toAPIBlocks(saved.Kind, []block.Block{saved.Block})
		if conversionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the new block."})
			return
		}
		c.JSON(http.StatusCreated, blocks[0])
	}
}

func (h *Handlers) ArrangeAssetBlocks(c *gin.Context, id types.UUID) {
	owner, ok := h.verifiedAccount(c, "arranging an asset")
	if !ok {
		return
	}
	var request ArrangeAssetBlocksRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send every block once with its id, hidden state and width."})
		return
	}
	arrangement := make([]asset.BlockArrangement, len(request.Blocks))
	for i, choice := range request.Blocks {
		arrangement[i] = asset.BlockArrangement{
			ID: uuid.UUID(choice.Id), Hidden: choice.Hidden, Width: block.Width(choice.Width),
		}
	}
	saved, err := h.assets.ArrangeBlocks(c.Request.Context(), owner.ID, uuid.UUID(id), arrangement)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such asset."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrInvalidBlock):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not arrange the blocks."})
	default:
		blocks, conversionErr := toAPIBlocks(saved.Kind, saved.Blocks)
		if conversionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the arranged blocks."})
			return
		}
		c.JSON(http.StatusOK, blocks)
	}
}

func (h *Handlers) RemoveAssetBlock(c *gin.Context, id types.UUID, blockID types.UUID) {
	owner, ok := h.verifiedAccount(c, "removing a block")
	if !ok {
		return
	}
	err := h.assets.RemoveBlock(c.Request.Context(), owner.ID, uuid.UUID(id), uuid.UUID(blockID))
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such block."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrInvalidBlock):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not remove the block."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) MoveAssetBlockContent(c *gin.Context, id types.UUID, blockID types.UUID) {
	owner, ok := h.verifiedAccount(c, "moving block content")
	if !ok {
		return
	}
	var request MoveAssetBlockContentRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose the block that should keep this content."})
		return
	}
	saved, err := h.assets.MoveBlockContent(
		c.Request.Context(), owner.ID, uuid.UUID(id), uuid.UUID(blockID), uuid.UUID(request.DestinationBlockId),
	)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such block."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrInvalidBlock):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not move the block content."})
	default:
		blocks, conversionErr := toAPIBlocks(saved.Kind, saved.Blocks)
		if conversionErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the arranged blocks."})
			return
		}
		c.JSON(http.StatusOK, blocks)
	}
}
