package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/delivery"
	"github.com/Sillyfrogster/Illarin/api/internal/linking"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

// maxLibraryBodyBytes bounds a library report before parsing, with room to spare for 2000 entries.
const maxLibraryBodyBytes = 256 << 10

func (h *Handlers) CollectDeliveries(c *gin.Context) {
	noStoreLink(c)
	instance, ok := h.instance(c, linking.ScopeReceiveAssets)
	if !ok {
		return
	}
	var request CollectDeliveries
	if !readLinkJSON(c, &request) {
		return
	}
	work, err := h.deliveries.Collect(
		c.Request.Context(), instance, uuidsFrom(request.Acknowledge),
	)
	if err != nil {
		h.deliveryError(c, err)
		return
	}
	if len(work) == 0 {
		c.Status(http.StatusNoContent)
		return
	}
	items := make([]DeliveryWork, 0, len(work))
	for _, released := range work {
		items = append(items, toAPIDeliveryWork(released))
	}
	c.JSON(http.StatusOK, DeliveryWorkList{Deliveries: items})
}

func (h *Handlers) SyncLibrary(c *gin.Context) {
	noStoreLink(c)
	instance, ok := h.instance(c, linking.ScopeSyncLibrary)
	if !ok {
		return
	}
	var request LibraryReport
	if !readBoundedJSON(c, &request, maxLibraryBodyBytes, "The library report is too large.") {
		return
	}
	result, err := h.deliveries.Sync(c.Request.Context(), instance, toLibraryReport(request))
	if err != nil {
		h.deliveryError(c, err)
		return
	}
	c.JSON(http.StatusOK, LibraryReportResult{
		Accepted: result.Accepted, Removed: result.Removed, Ignored: result.Ignored,
	})
}

func (h *Handlers) GetAssetInstances(c *gin.Context, id types.UUID) {
	noStoreLink(c)
	creator, ok := h.signedInAccount(c, "sending an asset to an application")
	if !ok {
		return
	}
	found, err := h.deliveries.AssetInstances(c.Request.Context(), creator.ID, uuid.UUID(id))
	if err != nil {
		h.deliveryError(c, err)
		return
	}
	items := make([]AssetInstance, 0, len(found.Items))
	for _, state := range found.Items {
		items = append(items, toAPIAssetInstance(state))
	}
	c.JSON(http.StatusOK, AssetInstanceList{
		ContentGeneration: found.ContentGeneration, Items: items,
	})
}

func (h *Handlers) SendAssetToInstance(
	c *gin.Context,
	id types.UUID,
	_ SendAssetToInstanceParams,
) {
	noStoreLink(c)
	creator, ok := h.signedInAccount(c, "sending an asset to an application")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	var request SendAssetRequest
	if !readLinkJSON(c, &request) {
		return
	}
	queued, err := h.deliveries.Queue(
		c.Request.Context(), creator.ID, uuid.UUID(request.InstanceId), uuid.UUID(id),
	)
	if err != nil {
		h.deliveryError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, toAPIQueuedDelivery(queued))
}

func (h *Handlers) DiscardDelivery(c *gin.Context, id types.UUID, _ DiscardDeliveryParams) {
	noStoreLink(c)
	creator, ok := h.signedInAccount(c, "managing deliveries")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	if err := h.deliveries.Discard(c.Request.Context(), creator.ID, uuid.UUID(id)); err != nil {
		h.deliveryError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DownloadDeliveryExport hands over one released delivery's file, authorizing the asset again here.
func (h *Handlers) DownloadDeliveryExport(
	c *gin.Context,
	id types.UUID,
	params DownloadDeliveryExportParams,
) {
	c.Header("Cache-Control", "private, no-store")
	assetID, target, err := h.deliveries.Artifact(
		c.Request.Context(), uuid.UUID(id), params.Expires, params.Signature,
	)
	if err != nil {
		h.deliveryArtifactError(c, err)
		return
	}
	if target == asset.RawDownloadTarget {
		download, err := h.assets.DownloadSourceForLinkedInstance(c.Request.Context(), assetID)
		if err != nil {
			h.deliveryArtifactError(c, err)
			return
		}
		h.handOffDownload(c, download)
		return
	}
	download, err := h.assets.DownloadExportForLinkedInstance(c.Request.Context(), assetID, target)
	if err != nil {
		h.deliveryArtifactError(c, err)
		return
	}
	h.handOffExport(c, download)
}

// deliveryArtifactError answers a signed address the way an ordinary download answers.
func (h *Handlers) deliveryArtifactError(c *gin.Context, err error) {
	if errors.Is(err, delivery.ErrArtifactNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such download"})
		return
	}
	h.downloadError(c, err)
}

func (h *Handlers) deliveryError(c *gin.Context, err error) {
	var limited *linking.RateLimitError
	switch {
	case errors.As(err, &limited):
		seconds := int((limited.After + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Try again later."})
	case errors.Is(err, delivery.ErrTooManyCollectors):
		c.Header("Retry-After", strconv.Itoa(collectorsBusySeconds))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Too many applications are waiting for work. Try again shortly.",
		})
	case errors.Is(err, delivery.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No live application of yours has that id."})
	case errors.Is(err, delivery.ErrMissingScope):
		c.JSON(http.StatusForbidden, gin.H{"error": "That application cannot receive assets."})
	case errors.Is(err, delivery.ErrAssetNotFound), errors.Is(err, delivery.ErrAssetNotSendable):
		c.JSON(http.StatusNotFound, gin.H{"error": "No asset that can be sent has that id."})
	case errors.Is(err, delivery.ErrNoTarget):
		c.JSON(http.StatusConflict, gin.H{
			"error": "That application accepts no format this asset can be written in.",
		})
	case errors.Is(err, delivery.ErrQueueFull):
		c.JSON(http.StatusConflict, gin.H{
			"error": "That application already has as many deliveries waiting as it may hold.",
		})
	case errors.Is(err, delivery.ErrDeliveryNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No delivery of yours has that id."})
	case errors.Is(err, delivery.ErrLibraryTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "Report fewer installed assets in one request.",
		})
	case errors.Is(err, delivery.ErrLibraryReport):
		c.JSON(http.StatusBadRequest, gin.H{"error": "That report is not valid."})
	case errors.Is(err, delivery.ErrAcknowledgement):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Acknowledge at most 32 deliveries in one request.",
		})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not complete the request."})
	}
}

const collectorsBusySeconds = 30

func toLibraryReport(request LibraryReport) delivery.LibraryReport {
	entries := make([]delivery.LibraryEntry, 0, len(request.Entries))
	for _, entry := range request.Entries {
		entries = append(entries, delivery.LibraryEntry{
			AssetID: uuid.UUID(entry.AssetId), ContentGeneration: entry.ContentGeneration,
		})
	}
	var removed []uuid.UUID
	if request.Removed != nil {
		removed = uuidsFrom(*request.Removed)
	}
	return delivery.LibraryReport{
		Snapshot: request.Snapshot, Entries: entries, Removed: removed,
	}
}

func toAPIDeliveryWork(released delivery.Work) DeliveryWork {
	artifacts := make([]DeliveryArtifact, 0, len(released.Artifacts))
	for _, artifact := range released.Artifacts {
		item := DeliveryArtifact{
			Kind: DeliveryArtifactKind(artifact.Kind), Url: artifact.URL,
		}
		if artifact.MediaID != nil {
			mediaID := types.UUID(*artifact.MediaID)
			role, isCover := artifact.Role, artifact.IsCover
			item.MediaId, item.Role, item.IsCover = &mediaID, &role, &isCover
		}
		artifacts = append(artifacts, item)
	}
	return DeliveryWork{
		Id: types.UUID(released.ID), AssetId: types.UUID(released.AssetID),
		ContentGeneration: released.ContentGeneration, Kind: released.Kind,
		Name: released.Name, Format: released.Format, Label: released.Label,
		QueuedAt: released.QueuedAt, LeaseExpiresAt: released.LeaseExpiresAt,
		Artifacts: artifacts,
	}
}

func toAPIQueuedDelivery(queued delivery.Delivery) QueuedDelivery {
	item := QueuedDelivery{
		Id: types.UUID(queued.ID), InstanceId: types.UUID(queued.InstanceID),
		AssetId: types.UUID(queued.AssetID), State: QueuedDeliveryState(queued.State),
		QueuedAt: queued.QueuedAt, ExpiresAt: queued.ExpiresAt,
	}
	if queued.Reason != "" {
		reason := QueuedDeliveryReason(queued.Reason)
		item.Reason = &reason
	}
	return item
}

func toAPIAssetInstance(state delivery.InstanceState) AssetInstance {
	item := AssetInstance{
		InstanceId: types.UUID(state.InstanceID), ApplicationName: state.ApplicationName,
		InstanceName: state.InstanceName, LastSeenAt: state.LastSeenAt,
		CanReceive: state.CanReceive, ReportsLibrary: state.ReportsLibrary,
		InstalledGeneration: state.InstalledGeneration,
		UpdateAvailable:     state.UpdateAvailable,
	}
	if state.Delivery != nil {
		queued := toAPIQueuedDelivery(*state.Delivery)
		item.Delivery = &queued
	}
	return item
}

func uuidsFrom(values []types.UUID) []uuid.UUID {
	converted := make([]uuid.UUID, len(values))
	for index, value := range values {
		converted[index] = uuid.UUID(value)
	}
	return converted
}
