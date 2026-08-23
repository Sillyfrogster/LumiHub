package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/Sillyfrogster/Illarin/api/internal/asset"
	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) WithholdAsset(c *gin.Context, id types.UUID) {
	admin, ok := h.admin(c)
	if !ok {
		return
	}
	var request WithholdAssetRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Give a reason for withholding the asset."})
		return
	}
	err := h.assets.Withhold(c.Request.Context(), uuid.UUID(id), admin.ID, request.Reason)
	switch {
	case errors.Is(err, asset.ErrInvalidWithholdReason):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Give a reason for withholding the asset."})
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not withhold the asset."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) ClearAssetWithhold(c *gin.Context, id types.UUID) {
	if _, ok := h.admin(c); !ok {
		return
	}
	err := h.assets.ClearWithhold(c.Request.Context(), uuid.UUID(id))
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not clear the withhold."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) admin(c *gin.Context) (account.Account, bool) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before managing assets."})
		return account.Account{}, false
	}
	current, err := h.accounts.Current(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not check the signed-in account."})
		return account.Account{}, false
	}
	if current == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before managing assets."})
		return account.Account{}, false
	}
	if !current.EmailVerified || current.Role != account.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only an admin can manage withholds."})
		return account.Account{}, false
	}
	return *current, true
}

func (h *Handlers) viewerID(c *gin.Context) (*uuid.UUID, bool) {
	token, err := c.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return nil, true
	}
	if err != nil {
		return nil, true
	}
	current, err := h.accounts.Current(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the signed-in account."})
		return nil, false
	}
	if current == nil {
		return nil, true
	}
	return &current.ID, true
}

func (h *Handlers) ListAssets(c *gin.Context, params ListAssetsParams) {
	f := asset.ListFilter{}

	if params.Creator != nil {
		profile, err := h.accounts.Profile(c.Request.Context(), strings.ToLower(*params.Creator))
		if errors.Is(err, account.ErrProfileNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "No such profile."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the profile."})
			return
		}
		token, _ := c.Cookie(sessionCookieName)
		current, err := h.accounts.Current(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the signed-in account."})
			return
		}
		f.Profile = &asset.ProfileListingScope{
			CreatorID:        profile.ID,
			CreatorShowsNSFW: profile.ShowNSFWContributionsOnProfile,
		}
		if current != nil {
			f.Profile.ViewerID = &current.ID
		}
	}

	if params.Kind != nil {
		f.Kind = string(*params.Kind)
	}
	if params.Platform != nil {
		f.Platform, f.PlatformSet = params.Platform, true
	}
	if params.Q != nil {
		f.Query = *params.Q
	}
	if params.Facet != nil {
		f.Facets = parseFacets(*params.Facet)
	}
	if params.Limit != nil {
		f.Limit = *params.Limit
	}

	before, ok := cursorFrom(params)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "before and beforeId belong together, send both or neither",
		})
		return
	}
	f.Before = before

	var requested *string
	if params.Nsfw != nil {
		value := string(*params.Nsfw)
		requested = &value
	}
	visibility, ok := h.readerVisibility(c, requested)
	if !ok {
		return
	}
	found, err := h.assets.Browse(c.Request.Context(), f, visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list assets"})
		return
	}

	items := make([]BrowseAsset, 0, len(found.Items))
	for _, item := range found.Items {
		var cover *BrowseCover
		if item.Cover != nil {
			cover = &BrowseCover{
				Url: item.Cover.URL, Width: item.Cover.Width, Height: item.Cover.Height,
			}
		}
		var ownerState *BrowseAssetOwnerState
		if item.OwnerState != "" {
			value := BrowseAssetOwnerState(item.OwnerState)
			ownerState = &value
		}
		items = append(items, BrowseAsset{
			Id: types.UUID(item.ID), Name: item.Name, Creator: item.Creator,
			Kind: BrowseAssetKind(item.Kind), IsNsfw: item.IsNSFW, Cover: cover,
			OwnerState: ownerState,
			Withhold:   toAPIWithhold(item.Withhold),
		})
	}
	var next *BrowseCursor
	if found.Next != nil {
		next = &BrowseCursor{Before: found.Next.MadeAt, BeforeId: types.UUID(found.Next.ID)}
	}
	var empty *AssetListEmptyState
	if found.EmptyState != "" {
		value := AssetListEmptyState(found.EmptyState)
		empty = &value
	}
	platforms := make([]BrowseOption, 0, len(found.Platforms))
	for _, option := range found.Platforms {
		platforms = append(platforms, BrowseOption{
			Value: option.Value, Label: option.Label, Count: option.Count, Selected: option.Selected,
		})
	}
	facets := make([]BrowseFacet, 0, len(found.Facets))
	for _, group := range found.Facets {
		options := make([]BrowseOption, 0, len(group.Options))
		for _, option := range group.Options {
			options = append(options, BrowseOption{
				Value: option.Value, Label: option.Label, Count: option.Count, Selected: option.Selected,
			})
		}
		facets = append(facets, BrowseFacet{Key: group.Key, Label: group.Label, Options: options})
	}
	c.JSON(http.StatusOK, AssetList{
		Items: items, Total: found.Total, Suppressed: found.Suppressed,
		Visibility: AssetListVisibility(visibility),
		NextCursor: next, Platforms: platforms, Facets: facets, EmptyState: empty,
	})
}

// CreateAsset brings a file in, or starts an asset from nothing when the body
// is JSON naming a kind. Both paths land on the same page.
func (h *Handlers) CreateAsset(c *gin.Context) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	if c.ContentType() == "application/json" {
		h.startAssetFromNothing(c, owner)
		return
	}
	h.acceptUpload(c, owner)
}

func (h *Handlers) acceptUpload(c *gin.Context, owner account.Account) {
	parts, err := c.Request.MultipartReader()
	if err != nil {
		h.refuse(c, refusal{
			reason: "send the asset as form data, with a metadata part and a file part",
			cause:  err,
		})
		return
	}

	metadata, err := readMetadata(parts)
	if err != nil {
		h.refuse(c, err)
		return
	}
	file, err := nextPart(parts, filePart)
	if err != nil {
		h.refuse(c, err)
		return
	}
	if !metadata.Confirmed {
		h.refuse(c, refusal{reason: "confirm the catalog details before uploading"})
		return
	}
	limitedFile := http.MaxBytesReader(c.Writer, file, h.maxUploadBytes)
	defer limitedFile.Close()

	operation, err := h.assets.AcceptIngest(
		c.Request.Context(), ingestInput(metadata, file.FileName(), limitedFile, owner.ID),
	)
	if errors.Is(err, storage.ErrTombstoned) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This file cannot be accepted."})
		return
	}
	if err != nil {
		h.refuse(c, err)
		return
	}

	location := "/v1/ingests/" + operation.ID.String()
	c.Header("Location", location)
	c.JSON(http.StatusAccepted, toAPIIngest(operation))
}

func (h *Handlers) AddAssetRevision(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}

	parts, err := c.Request.MultipartReader()
	if err != nil {
		h.refuse(c, refusal{
			reason: "send the revision as form data, with a file part",
			cause:  err,
		})
		return
	}
	file, err := nextPart(parts, filePart)
	if err != nil {
		h.refuse(c, err)
		return
	}
	limitedFile := http.MaxBytesReader(c.Writer, file, h.maxUploadBytes)
	defer limitedFile.Close()

	operation, err := h.assets.AcceptRevision(c.Request.Context(), asset.RevisionInput{
		OwnerID:  owner.ID,
		AssetID:  uuid.UUID(id),
		Filename: file.FileName(),
		File:     limitedFile,
	})
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
		return
	case errors.Is(err, storage.ErrTombstoned):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This file cannot be accepted."})
		return
	case err != nil:
		h.refuse(c, err)
		return
	}

	location := "/v1/ingests/" + operation.ID.String()
	c.Header("Location", location)
	c.JSON(http.StatusAccepted, toAPIIngest(operation))
}

func (h *Handlers) DeleteAsset(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	err := h.assets.Delete(c.Request.Context(), owner.ID, uuid.UUID(id))
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be deleted."})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not delete the asset."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) RestoreAsset(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	err := h.assets.Restore(c.Request.Context(), owner.ID, uuid.UUID(id))
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such recoverable asset"})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not restore the asset."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) ListDeletedAssets(c *gin.Context, handle string) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	if !strings.EqualFold(owner.Handle, handle) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such deleted listing"})
		return
	}
	found, err := h.assets.Deleted(c.Request.Context(), owner.ID, strings.ToLower(handle))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not list deleted assets."})
		return
	}
	items := make([]DeletedAsset, len(found))
	for i, item := range found {
		items[i] = DeletedAsset{
			Id: types.UUID(item.ID), Name: item.Name, Kind: DeletedAssetKind(item.Kind),
			DeletedAt: item.DeletedAt, RecoverableUntil: item.RecoverableUntil,
		}
	}
	c.JSON(http.StatusOK, DeletedAssetList{Items: items})
}

func (h *Handlers) AddMedia(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	parts, err := c.Request.MultipartReader()
	if err != nil {
		h.refuse(c, refusal{
			reason: "send the image as form data, with a metadata part and a file part",
			cause:  err,
		})
		return
	}
	metadata, err := readMediaMetadata(parts)
	if err != nil {
		h.refuse(c, err)
		return
	}
	file, err := nextPart(parts, filePart)
	if err != nil {
		h.refuse(c, err)
		return
	}
	limitedFile := http.MaxBytesReader(c.Writer, file, h.maxUploadBytes)
	defer limitedFile.Close()
	added, err := h.assets.AddMedia(c.Request.Context(), asset.AddMediaInput{
		OwnerID: owner.ID,
		AssetID: uuid.UUID(id),
		Role:    asset.MediaRole(metadata.Role),
		File:    limitedFile,
	})
	if errors.Is(err, asset.ErrMediaNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	}
	if errors.Is(err, asset.ErrAssetFrozen) {
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
		return
	}
	if err != nil {
		h.refuse(c, err)
		return
	}
	c.JSON(http.StatusCreated, toAPIMedia(added))
}

// readerVisibility is the reader's own preference for adult content: whatever
// the request states, and otherwise whatever their account holds.
func (h *Handlers) readerVisibility(
	c *gin.Context,
	requested *string,
) (asset.ContentVisibility, bool) {
	if requested != nil {
		return asset.ContentVisibility(*requested), true
	}
	token, _ := c.Cookie(sessionCookieName)
	preference, err := h.accounts.NSFWVisibility(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not read the content preference",
		})
		return "", false
	}
	return asset.ContentVisibility(preference), true
}

// GetAsset answers an asset's own page. For anyone but the owner, withheld,
// deleted and never-existed all leave through the same 404, so no response says
// which.
func (h *Handlers) GetAsset(c *gin.Context, id types.UUID, params GetAssetParams) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	var requested *string
	if params.Nsfw != nil {
		value := string(*params.Nsfw)
		requested = &value
	}
	visibility, ok := h.readerVisibility(c, requested)
	if !ok {
		return
	}
	found, err := h.assets.Detail(c.Request.Context(), uuid.UUID(id), viewerID, visibility)
	if errors.Is(err, asset.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the asset"})
		return
	}
	page, err := toAPIDetail(found, visibility)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the asset"})
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *Handlers) SetAssetDiscovery(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	var request AssetDiscoveryRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil || !request.Discovery.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose listed or unlisted."})
		return
	}
	err := h.assets.SetDiscovery(
		c.Request.Context(), owner.ID, uuid.UUID(id), asset.Discovery(request.Discovery),
	)
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrAssetIsDraft):
		c.JSON(http.StatusConflict, gin.H{
			"error": "Discovery applies once the asset is published.",
		})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save discovery."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func toAPIDetail(found asset.Detail, visibility asset.ContentVisibility) (AssetDetail, error) {
	tags := make([]AssetTag, 0, len(found.Tags))
	for _, tag := range found.Tags {
		tags = append(tags, AssetTag{Label: tag.Label, Value: tag.Value})
	}
	media := make([]AssetImage, 0, len(found.Media))
	for _, image := range found.Media {
		media = append(media, AssetImage{
			Id:        types.UUID(image.ID),
			Role:      AssetImageRole(image.Role),
			IsCover:   image.IsCover,
			DetailUrl: image.DetailURL,
			ThumbUrl:  image.ThumbURL,
			Width:     image.Width,
			Height:    image.Height,
		})
	}
	blocks, err := toAPIBlocks(found.Kind, found.Blocks)
	if err != nil {
		return AssetDetail{}, err
	}
	addable := toAPIAddableBlocks(found.Kind, found.IsOwner)
	return AssetDetail{
		Id:            types.UUID(found.ID),
		Kind:          AssetDetailKind(found.Kind),
		Name:          found.Name,
		Blurb:         found.Blurb,
		Tags:          tags,
		Creator:       found.Creator,
		IsNsfw:        found.IsNSFW,
		Discovery:     AssetDetailDiscovery(found.Discovery),
		Lifecycle:     AssetDetailLifecycle(found.Lifecycle),
		IsOwner:       found.IsOwner,
		Downloads:     toAPIDownloads(found.Downloads),
		Original:      toAPIOriginalUpload(found.Original),
		CreatedAt:     found.CreatedAt,
		Blocks:        blocks,
		Media:         media,
		Preview:       found.Preview,
		Readiness:     toAPIReadiness(found.Readiness),
		SealedBlocks:  countOrAbsent(found.SealedBlocks),
		AddableBlocks: addable,
		Visibility:    AssetDetailVisibility(visibility),
		Withhold:      toAPIWithhold(found.Withhold),
	}, nil
}

// toAPIDownloads serves the download menu: one line per format, each already
// carrying what it does and does not take with it.
func toAPIDownloads(targets []format.Target) []DownloadTarget {
	downloads := make([]DownloadTarget, 0, len(targets))
	for _, target := range targets {
		roles := make([]DownloadRoleVerdict, 0, len(target.Roles))
		for _, role := range target.Roles {
			roles = append(roles, DownloadRoleVerdict{
				Role: string(role.Role), Label: role.Label,
				Verdict:     DownloadRoleVerdictVerdict(role.Verdict),
				Reason:      textOrNil(role.Reason),
				Destination: textOrNil(role.Destination),
				Sample:      toAPIDownloadSample(role.Sample),
			})
		}
		downloads = append(downloads, DownloadTarget{
			Format: target.Format, Label: target.Label,
			Recommended: target.Recommended, Roles: roles,
		})
	}
	return downloads
}

func toAPIDownloadSample(sample block.Sample) DownloadSample {
	converted := DownloadSample{Count: sample.Count}
	if len(sample.Texts) > 0 {
		texts := sample.Texts
		converted.Texts = &texts
	}
	if len(sample.Images) > 0 {
		images := make([]types.UUID, 0, len(sample.Images))
		for _, image := range sample.Images {
			images = append(images, types.UUID(image))
		}
		converted.Images = &images
	}
	return converted
}

func toAPIOriginalUpload(found *asset.OriginalUpload) *OriginalUpload {
	if found == nil {
		return nil
	}
	return &OriginalUpload{
		Label: found.Label, MediaType: found.MediaType, ArrivedAt: found.ArrivedAt,
	}
}

func textOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// toAPIAddableBlocks serves the add tray's catalog. Only the owner can add a
// block, so nobody else is handed the list.
func toAPIAddableBlocks(kind string, isOwner bool) *[]AddableBlock {
	if !isOwner {
		return nil
	}
	offers, ok := block.Offers(kind)
	if !ok {
		return nil
	}
	addable := make([]AddableBlock, 0, len(offers))
	for _, offer := range offers {
		choices := make([]AddableBlockChoice, 0, len(offer.Choices))
		for _, choice := range offer.Choices {
			choices = append(choices, AddableBlockChoice{
				Label: choice.Label, Type: ElementType(choice.Type),
			})
		}
		addable = append(addable, AddableBlock{
			Definition: string(offer.Definition),
			Title:      offer.Title,
			Summary:    offer.Summary,
			Group:      AddableBlockGroup(offer.Group),
			GroupTitle: offer.Group.Title(),
			Repeatable: offer.Repeatable,
			Choices:    choices,
		})
	}
	return &addable
}

func toAPIWithhold(found *asset.Withhold) *AssetWithhold {
	if found == nil {
		return nil
	}
	return &AssetWithhold{Reason: found.Reason, Actor: found.Actor, At: found.At}
}

func (h *Handlers) ListMedia(c *gin.Context, id types.UUID) {
	viewerID, ok := h.viewerID(c)
	if !ok {
		return
	}
	found, err := h.assets.ListMedia(c.Request.Context(), uuid.UUID(id), viewerID)
	if errors.Is(err, asset.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list the images"})
		return
	}
	items := make([]Media, 0, len(found))
	for _, item := range found {
		items = append(items, toAPIMedia(item))
	}
	c.JSON(http.StatusOK, MediaList{Items: items})
}

func (h *Handlers) GetIngest(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	operation, err := h.assets.GetIngest(c.Request.Context(), owner.ID, uuid.UUID(id))
	if errors.Is(err, asset.ErrIngestNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such ingest operation"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the ingest operation"})
		return
	}
	c.JSON(http.StatusOK, toAPIIngest(operation))
}

func toAPIIngest(operation asset.IngestOperation) gin.H {
	response := gin.H{
		"id":     operation.ID,
		"status": operation.Status,
		"url":    "/v1/ingests/" + operation.ID.String(),
		"asset":  ingestAsset(operation.Asset),
	}
	if operation.Failure != nil {
		response["failure"] = gin.H{
			"reason":  operation.Failure.Reason,
			"message": operation.Failure.Message,
		}
	}
	return response
}

func ingestAsset(a *asset.Asset) *Asset {
	if a == nil {
		return nil
	}
	converted := toAPI(*a)
	return &converted
}

// ResolveLegacyAsset answers for a v1 public address. The lookup happens before
// any redirect, so the answer never confirms an asset a visitor may not see.
func (h *Handlers) ResolveLegacyAsset(c *gin.Context, author string, name string) {
	found, err := h.assets.ResolveLegacyAddress(c.Request.Context(), author+"/"+name)
	if errors.Is(err, asset.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the asset"})
		return
	}
	c.JSON(http.StatusOK, LegacyAsset{Id: types.UUID(found.ID), Name: found.Name})
}

func cursorFrom(params ListAssetsParams) (*asset.Cursor, bool) {
	switch {
	case params.Before == nil && params.BeforeId == nil:
		return nil, true
	case params.Before == nil || params.BeforeId == nil:
		return nil, false
	}
	return &asset.Cursor{MadeAt: *params.Before, ID: uuid.UUID(*params.BeforeId)}, true
}

func parseFacets(raw []string) []asset.FacetSelection {
	out := make([]asset.FacetSelection, 0, len(raw))
	for _, pair := range raw {
		key, value, split := strings.Cut(pair, "=")
		if !split {
			continue
		}
		out = append(out, asset.FacetSelection{Key: key, Value: value})
	}
	return out
}

func toAPI(a asset.Asset) Asset {
	return Asset{
		Id: types.UUID(a.ID), Kind: a.Kind, Format: a.Format,
		Name: a.Name, Blurb: a.Blurb, Tags: a.Tags, IsNsfw: a.IsNSFW,
		Discovery: AssetDiscovery(a.Discovery), CreatedAt: a.CreatedAt,
	}
}

// countOrAbsent leaves the field out where there is nothing to count, because a zero would read as an answer this reader is not entitled to.
func countOrAbsent(count int) *int {
	if count == 0 {
		return nil
	}
	return &count
}
