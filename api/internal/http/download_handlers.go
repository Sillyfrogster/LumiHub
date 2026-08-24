package http

import (
	"errors"
	"math"
	"net/http"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
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
	h.handOffExport(c, download)
}

func (h *Handlers) downloadError(c *gin.Context, err error) {
	if errors.Is(err, asset.ErrNotFound) || errors.Is(err, asset.ErrTargetNotOffered) || errors.Is(err, asset.ErrLinkedInstallOnly) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such download"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the file"})
}

// handOffExport writes a generated file straight out. There is nothing on disk
// to hand nginx, because an export is produced on request and never cached.
func (h *Handlers) handOffExport(c *gin.Context, download asset.Export) {
	if download.Event != nil {
		if err := h.assets.RecordDownload(c.Request.Context(), *download.Event); err != nil {
			h.downloadError(c, err)
			return
		}
	}
	c.Header("Content-Disposition", `attachment; filename="`+download.Filename+`"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Illarin-Export-Target", download.Target)
	c.Data(http.StatusOK, download.MediaType, download.Body)
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
	params GetMediaVariantParams,
) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	if derivativeVersion < 1 || uint64(derivativeVersion) > math.MaxUint32 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such media variant"})
		return
	}
	download, err := h.assets.MediaVariant(c.Request.Context(), asset.MediaRequest{
		MediaID:   uuid.UUID(mediaID),
		Variant:   string(variant),
		Version:   uint32(derivativeVersion),
		ViewerID:  viewerID,
		Expires:   valueOrEmpty(params.Expires),
		Signature: valueOrEmpty(params.Signature),
	})
	if errors.Is(err, asset.ErrMediaNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such media variant"})
		return
	}
	if errors.Is(err, storage.ErrInsufficientSpace) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "The image is temporarily unavailable."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the image"})
		return
	}
	// A public image can never resolve to different bytes, so it is cached
	// hard. A draft's image is served against a signature that runs out.
	cache := "public, max-age=31536000, immutable"
	if download.Private {
		cache = "private, no-store"
	}
	c.Header("Cache-Control", cache)
	c.Header("Content-Disposition", "inline")
	c.Header("Content-Type", download.MediaType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Accel-Redirect", download.InternalRedirect)
	c.Status(http.StatusOK)
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
