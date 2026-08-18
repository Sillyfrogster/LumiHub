package http

import (
	"errors"
	"math"
	"net/http"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) DownloadSource(c *gin.Context, id types.UUID) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	download, err := h.assets.DownloadSource(c.Request.Context(), id, viewerID)
	if err != nil {
		h.downloadError(c, err)
		return
	}
	h.handOffDownload(c, download)
}

func (h *Handlers) DownloadExport(c *gin.Context, id types.UUID, target string) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	download, err := h.assets.DownloadExport(c.Request.Context(), id, viewerID, target)
	if err != nil {
		h.downloadError(c, err)
		return
	}
	c.Header("X-LumiHub-Export-Target", download.Target)
	h.handOffDownload(c, download.SourceDownload)
}

func (h *Handlers) downloadError(c *gin.Context, err error) {
	if errors.Is(err, asset.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the file"})
}

func (h *Handlers) handOffDownload(c *gin.Context, download asset.SourceDownload) {
	if err := h.assets.RecordDownload(c.Request.Context(), download.Event); err != nil {
		h.downloadError(c, err)
		return
	}
	disposition := "attachment"
	mediaType := "application/octet-stream"
	if download.Inline {
		disposition = "inline"
		mediaType = download.MediaType
	}
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", mediaType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Accel-Redirect", download.InternalRedirect)
	c.Status(http.StatusOK)
}

func (h *Handlers) GetMediaVariant(
	c *gin.Context,
	mediaID types.UUID,
	variant GetMediaVariantParamsVariant,
	derivativeVersion int,
) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	if derivativeVersion < 1 || uint64(derivativeVersion) > math.MaxUint32 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such media variant"})
		return
	}
	download, err := h.assets.MediaVariant(
		c.Request.Context(), uuid.UUID(mediaID), string(variant), uint32(derivativeVersion), viewerID,
	)
	if errors.Is(err, asset.ErrMediaNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such media variant"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the image"})
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Type", download.MediaType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Accel-Redirect", download.InternalRedirect)
	c.Status(http.StatusOK)
}

func toAPIMedia(found asset.Media) Media {
	return Media{
		Id:                types.UUID(found.ID),
		AssetId:           types.UUID(found.AssetID),
		Role:              MediaRole(found.Role),
		Width:             found.Width,
		Height:            found.Height,
		DerivativeVersion: int(found.DerivativeVersion),
	}
}
