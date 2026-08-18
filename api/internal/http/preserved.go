package http

import (
	"errors"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ListPreservedNamespaces names what an asset is holding on its creator's
// behalf. Only the owner asks: preserved data belongs to the file, never to
// the page, and nothing about it renders for a reader.
func (h *Handlers) ListPreservedNamespaces(c *gin.Context, id openapi_types.UUID) {
	owner, ok := h.verifiedAccount(c, "reading preserved data")
	if !ok {
		return
	}
	found, err := h.assets.PreservedNamespaces(c.Request.Context(), owner.ID, uuid.UUID(id))
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No such asset."})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read what this asset preserves."})
	default:
		served := make([]PreservedNamespace, 0, len(found))
		for _, namespace := range found {
			served = append(served, PreservedNamespace{
				Name: namespace.Name, Bytes: namespace.Bytes,
			})
		}
		c.JSON(http.StatusOK, served)
	}
}

// DeletePreservedNamespace removes one namespace for good.
func (h *Handlers) DeletePreservedNamespace(
	c *gin.Context,
	id openapi_types.UUID,
	namespace string,
) {
	owner, ok := h.verifiedAccount(c, "deleting preserved data")
	if !ok {
		return
	}
	err := h.assets.DeletePreservedNamespace(
		c.Request.Context(), owner.ID, uuid.UUID(id), namespace,
	)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "This asset preserves no such data."})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete the preserved data."})
	default:
		c.Status(http.StatusNoContent)
	}
}
