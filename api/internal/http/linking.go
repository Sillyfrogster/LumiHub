package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/linking"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) StartLinkRequest(c *gin.Context) {
	var request StartLinkRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Send a name for this installation and the scopes it needs.",
		})
		return
	}
	started, err := h.links.Start(c.Request.Context(), linking.StartInput{
		Name:   request.Name,
		Scopes: toLinkingScopes(request.Scopes),
	})
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, LinkRequest{
		DeviceCode:              started.DeviceCode,
		UserCode:                started.UserCode,
		VerificationUrl:         started.VerifyURL,
		VerificationUrlComplete: started.CompleteURL,
		ExpiresAt:               started.ExpiresAt,
		Interval:                int(started.Interval.Seconds()),
	})
}

func (h *Handlers) PollLinkRequest(c *gin.Context) {
	var request PollLinkRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the device code as JSON."})
		return
	}
	credential, linked, err := h.links.Poll(c.Request.Context(), request.DeviceCode)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	if !linked {
		c.JSON(http.StatusOK, LinkPollResult{Status: LinkPollResultStatusPending})
		return
	}
	instance := toAPIInstance(credential.Instance)
	c.JSON(http.StatusOK, LinkPollResult{
		Status:   LinkPollResultStatusLinked,
		Token:    &credential.Token,
		Instance: &instance,
	})
}

func (h *Handlers) GetLinkRequest(c *gin.Context, userCode string) {
	creator, ok := h.verifiedAccount(c, "approving a link")
	if !ok {
		return
	}
	pending, err := h.links.Pending(c.Request.Context(), creator.ID, userCode)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPIPendingLink(pending))
}

func (h *Handlers) ApproveLinkRequest(c *gin.Context, userCode string) {
	creator, ok := h.verifiedAccount(c, "approving a link")
	if !ok {
		return
	}
	approved, err := h.links.Approve(c.Request.Context(), creator.ID, userCode)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPIPendingLink(approved))
}

func (h *Handlers) ListInstances(c *gin.Context) {
	creator, ok := h.signedInAccount(c, "managing linked instances")
	if !ok {
		return
	}
	found, err := h.links.List(c.Request.Context(), creator.ID)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	items := make([]LinkedInstance, 0, len(found))
	for _, instance := range found {
		items = append(items, toAPIInstance(instance))
	}
	c.JSON(http.StatusOK, LinkedInstanceList{Items: items})
}

func (h *Handlers) RevokeInstance(c *gin.Context, id types.UUID) {
	creator, ok := h.signedInAccount(c, "managing linked instances")
	if !ok {
		return
	}
	err := h.links.Revoke(c.Request.Context(), creator.ID, uuid.UUID(id))
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) GetInstance(c *gin.Context) {
	instance, ok := h.instance(c, "")
	if !ok {
		return
	}
	c.JSON(http.StatusOK, toAPIInstance(instance))
}

// instance answers the linked instance whose credential the request carries, or
// writes the refusal and returns false. Pass the scope the endpoint needs, or
// an empty scope where any live credential is enough.
func (h *Handlers) instance(c *gin.Context, needs linking.Scope) (linking.Instance, bool) {
	found, err := h.links.Authenticate(c.Request.Context(), bearerToken(c), needs)
	switch {
	case errors.Is(err, linking.ErrInstanceCredential):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "This credential is not linked to an account."})
		return linking.Instance{}, false
	case errors.Is(err, linking.ErrInstanceMissingScope):
		c.JSON(http.StatusForbidden, gin.H{"error": "This instance was not granted that scope."})
		return linking.Instance{}, false
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the linked instance."})
		return linking.Instance{}, false
	}
	return found, true
}

func bearerToken(c *gin.Context) string {
	scheme, token, found := strings.Cut(c.GetHeader("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func (h *Handlers) linkingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, linking.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name this installation in 64 characters or fewer.",
		})
	case errors.Is(err, linking.ErrInvalidScopes):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Ask for asset:receive, library:sync, or both, once each.",
		})
	case errors.Is(err, linking.ErrLinkRequestNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No link request is waiting on that code."})
	case errors.Is(err, linking.ErrPollTooSoon):
		c.Header("Retry-After", strconv.Itoa(int(linking.PollInterval().Seconds())))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Wait the full interval between polls."})
	case errors.Is(err, linking.ErrTooManyCodes):
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Too many codes were entered. Try again later.",
		})
	case errors.Is(err, linking.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No linked instance has that id."})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not complete the link request."})
	}
}

func toLinkingScopes(requested []Scope) []linking.Scope {
	scopes := make([]linking.Scope, len(requested))
	for i, scope := range requested {
		scopes[i] = linking.Scope(scope)
	}
	return scopes
}

func toAPIScopes(granted []linking.Scope) []Scope {
	scopes := make([]Scope, len(granted))
	for i, scope := range granted {
		scopes[i] = Scope(scope)
	}
	return scopes
}

func toAPIPendingLink(pending linking.Pending) PendingLink {
	return PendingLink{
		Name:      pending.Name,
		Scopes:    toAPIScopes(pending.Scopes),
		ExpiresAt: pending.ExpiresAt,
	}
}

func toAPIInstance(instance linking.Instance) LinkedInstance {
	return LinkedInstance{
		Id:         types.UUID(instance.ID),
		Name:       instance.Name,
		Prefix:     instance.Prefix,
		Scopes:     toAPIScopes(instance.Scopes),
		LinkedAt:   instance.LinkedAt,
		LastSeenAt: instance.LastSeenAt,
		RevokedAt:  instance.RevokedAt,
	}
}
