package http

import (
	"errors"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/gin-gonic/gin"
)

// startAssetFromNothing answers with the page a new draft's creator lands on.
func (h *Handlers) startAssetFromNothing(c *gin.Context, owner account.Account) {
	var request StartAssetRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the kind to build as JSON."})
		return
	}
	id, err := h.assets.StartFromNothing(c.Request.Context(), owner.ID, request.Kind)
	if errors.Is(err, asset.ErrKindNotBuildable) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Illarin cannot build that kind yet. Choose another.",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start the asset."})
		return
	}

	found, err := h.assets.Detail(c.Request.Context(), id, &owner.ID, asset.ContentShown)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the new asset."})
		return
	}
	page, err := toAPIDetail(found, asset.ContentShown)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the new asset."})
		return
	}
	c.Header("Location", "/v1/assets/"+id.String())
	c.JSON(http.StatusCreated, page)
}
