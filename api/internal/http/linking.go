package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/linking"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

func (h *Handlers) StartLinkRequest(c *gin.Context) {
	noStoreLink(c)
	var request StartLinkRequest
	if !readLinkJSON(c, &request) {
		return
	}
	started, err := h.links.Start(
		c.Request.Context(),
		linkRequestSource(c),
		startInput(
			request.ApplicationName, request.InstanceName, request.ApplicationVersion,
			request.ProtocolVersion, request.Capabilities, request.AcceptedTargets,
			request.Scopes,
		),
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, LinkRequest{
		DeviceCode: started.DeviceCode, UserCode: started.UserCode,
		VerificationUrl: started.VerifyURL, ExpiresAt: started.ExpiresAt,
		Interval: int(started.Interval.Seconds()),
	})
}

func (h *Handlers) StartLinkAuthorization(c *gin.Context) {
	noStoreLink(c)
	var request StartLinkAuthorization
	if !readLinkJSON(c, &request) {
		return
	}
	started, err := h.links.StartAuthorization(
		c.Request.Context(),
		linkRequestSource(c),
		linking.AuthorizationInput{
			StartInput: startInput(
				request.ApplicationName, request.InstanceName, request.ApplicationVersion,
				request.ProtocolVersion, request.Capabilities, request.AcceptedTargets,
				request.Scopes,
			),
			RedirectURI: request.RedirectUri, State: request.State,
			CodeChallenge:       request.CodeChallenge,
			CodeChallengeMethod: string(request.CodeChallengeMethod),
		},
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, LinkAuthorization{
		AuthorizationUrl: started.URL, ExpiresAt: started.ExpiresAt,
	})
}

func (h *Handlers) PollLinkRequest(c *gin.Context) {
	noStoreLink(c)
	var request PollLinkRequest
	if !readLinkJSON(c, &request) {
		return
	}
	grant, linked, err := h.links.Poll(
		c.Request.Context(), linkRequestSource(c), request.DeviceCode,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	if !linked {
		c.JSON(http.StatusOK, LinkPollResult{Status: LinkPollResultStatusPending})
		return
	}
	c.JSON(http.StatusOK, toAPIPollGrant(grant))
}

func (h *Handlers) GetLinkRequest(c *gin.Context, userCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "reviewing a link")
	if !ok {
		return
	}
	pending, err := h.links.Pending(c.Request.Context(), creator.ID, userCode)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPIPendingDeviceLink(pending))
}

func (h *Handlers) ApproveLinkRequest(c *gin.Context, userCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "approving a link")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	var decision DeviceLinkDecision
	if !readLinkJSON(c, &decision) {
		return
	}
	approved, err := h.links.Approve(
		c.Request.Context(), creator.ID, userCode, decision.ApprovalToken,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPIPendingLink(approved))
}

func (h *Handlers) DenyLinkRequest(c *gin.Context, userCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "denying a link")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	var decision DeviceLinkDecision
	if !readLinkJSON(c, &decision) {
		return
	}
	if err := h.links.Deny(
		c.Request.Context(), creator.ID, userCode, decision.ApprovalToken,
	); err != nil {
		h.linkingError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) GetLinkAuthorization(c *gin.Context, requestCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "reviewing a link")
	if !ok {
		return
	}
	pending, err := h.links.PendingAuthorization(
		c.Request.Context(), creator.ID, requestCode,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPIPendingLink(pending))
}

func (h *Handlers) ApproveLinkAuthorization(c *gin.Context, requestCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "approving a link")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	redirect, err := h.links.ApproveAuthorization(
		c.Request.Context(), creator.ID, requestCode,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, LinkRedirect{RedirectUrl: redirect.URL})
}

func (h *Handlers) DenyLinkAuthorization(c *gin.Context, requestCode string) {
	noStoreLink(c)
	creator, ok := h.verifiedAccount(c, "denying a link")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	redirect, err := h.links.DenyAuthorization(
		c.Request.Context(), creator.ID, requestCode,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, LinkRedirect{RedirectUrl: redirect.URL})
}

func (h *Handlers) ExchangeLinkAuthorization(c *gin.Context) {
	noStoreLink(c)
	var request ExchangeLinkAuthorization
	if !readLinkJSON(c, &request) {
		return
	}
	grant, err := h.links.Exchange(
		c.Request.Context(), linkRequestSource(c), request.AuthorizationCode,
		request.CodeVerifier, request.RedirectUri,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPITokenGrant(grant))
}

func (h *Handlers) RefreshInstanceToken(c *gin.Context) {
	noStoreLink(c)
	var request RefreshInstanceToken
	if !readLinkJSON(c, &request) {
		return
	}
	grant, err := h.links.Refresh(
		c.Request.Context(), linkRequestSource(c), request.RefreshToken,
	)
	if err != nil {
		h.linkingError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAPITokenGrant(grant))
}

func (h *Handlers) ListInstances(c *gin.Context) {
	noStoreLink(c)
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
	noStoreLink(c)
	creator, ok := h.signedInAccount(c, "managing linked instances")
	if !ok || !h.allowLinkBrowserMutation(c) {
		return
	}
	if err := h.links.Revoke(c.Request.Context(), creator.ID, uuid.UUID(id)); err != nil {
		h.linkingError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) GetInstance(c *gin.Context) {
	noStoreLink(c)
	instance, ok := h.instance(c, "")
	if !ok {
		return
	}
	c.JSON(http.StatusOK, toAPIInstance(instance))
}

func (h *Handlers) UpdateInstance(c *gin.Context) {
	noStoreLink(c)
	instance, ok := h.instance(c, "")
	if !ok {
		return
	}
	var request UpdateInstance
	if !readLinkJSON(c, &request) {
		return
	}
	updated, err := h.links.UpdateDeclaration(c.Request.Context(), instance.ID, linking.Declaration{
		ApplicationName: instance.ApplicationName, InstanceName: instance.InstanceName,
		ApplicationVersion: applicationVersion(request.ApplicationVersion),
		ProtocolVersion:    int(request.ProtocolVersion),
		Capabilities:       stringsFromCapabilities(request.Capabilities),
		AcceptedTargets:    stringsFromTargets(request.AcceptedTargets),
	})
	if err != nil {
		h.linkingError(c, err)
		return
	}
	updated.UserID = instance.UserID
	c.JSON(http.StatusOK, toAPIInstance(updated))
}

func (h *Handlers) instance(c *gin.Context, needs linking.Scope) (linking.Instance, bool) {
	found, err := h.links.Authenticate(c.Request.Context(), bearerToken(c), needs)
	switch {
	case errors.Is(err, linking.ErrInstanceCredential):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "This access token is not live."})
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
	var delay *linking.PollDelayError
	var limited *linking.RateLimitError
	switch {
	case errors.As(err, &delay):
		c.Header("Retry-After", strconv.Itoa(int(delay.After.Seconds())))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "slow_down"})
	case errors.Is(err, linking.ErrTooManyCodes):
		c.Header("Retry-After", strconv.Itoa(int(timeHourSeconds)))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many codes were entered. Try again later."})
	case errors.As(err, &limited):
		seconds := int((limited.After + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many link requests. Try again later."})
	case errors.Is(err, linking.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name the application and this installation in 64 characters or fewer."})
	case errors.Is(err, linking.ErrInvalidDeclaration):
		c.JSON(http.StatusBadRequest, gin.H{"error": "The instance declaration is not valid."})
	case errors.Is(err, linking.ErrInvalidScopes):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ask for asset:receive, library:sync, or both, once each."})
	case errors.Is(err, linking.ErrInvalidRedirect):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use an exact 127.0.0.1 or [::1] callback with an explicit port."})
	case errors.Is(err, linking.ErrInvalidPKCE):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
	case errors.Is(err, linking.ErrAccessDenied):
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
	case errors.Is(err, linking.ErrLinkExpired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "expired_token"})
	case errors.Is(err, linking.ErrLinkRequestNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No pending link request matches that code."})
	case errors.Is(err, linking.ErrRefreshReuse):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "This instance was revoked because a replaced refresh token was reused."})
	case errors.Is(err, linking.ErrInstanceCredential):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "This token is not live."})
	case errors.Is(err, linking.ErrInstanceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "No live linked instance has that id."})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not complete the link request."})
	}
}

const timeHourSeconds = 60 * 60

func startInput(
	applicationName ApplicationName,
	instanceName InstanceName,
	version *ApplicationVersion,
	protocol LinkProtocolVersion,
	capabilities InstanceCapabilities,
	targets AcceptedTargets,
	scopes Scopes,
) linking.StartInput {
	return linking.StartInput{
		Declaration: linking.Declaration{
			ApplicationName: string(applicationName), InstanceName: string(instanceName),
			ApplicationVersion: applicationVersion(version), ProtocolVersion: int(protocol),
			Capabilities:    stringsFromCapabilities(capabilities),
			AcceptedTargets: stringsFromTargets(targets),
		},
		Scopes: toLinkingScopes(scopes),
	}
}

func applicationVersion(value *ApplicationVersion) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func stringsFromCapabilities(values InstanceCapabilities) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func stringsFromTargets(values AcceptedTargets) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = string(value)
	}
	return result
}

func toLinkingScopes(requested []Scope) []linking.Scope {
	scopes := make([]linking.Scope, len(requested))
	for index, scope := range requested {
		scopes[index] = linking.Scope(scope)
	}
	return scopes
}

func toAPIScopes(granted []linking.Scope) []Scope {
	scopes := make([]Scope, len(granted))
	for index, scope := range granted {
		scopes[index] = Scope(scope)
	}
	return scopes
}

func toAPIPendingLink(pending linking.Pending) PendingLink {
	return PendingLink{
		ApplicationName:    ApplicationName(pending.ApplicationName),
		InstanceName:       InstanceName(pending.InstanceName),
		ApplicationVersion: apiApplicationVersion(pending.ApplicationVersion),
		ProtocolVersion:    LinkProtocolVersion(pending.ProtocolVersion),
		Capabilities:       apiCapabilities(pending.Capabilities),
		AcceptedTargets:    apiTargets(pending.AcceptedTargets),
		Scopes:             toAPIScopes(pending.Scopes), ExpiresAt: pending.ExpiresAt,
	}
}

func toAPIPendingDeviceLink(pending linking.Pending) PendingDeviceLink {
	base := toAPIPendingLink(pending)
	return PendingDeviceLink{
		ApplicationName: base.ApplicationName, InstanceName: base.InstanceName,
		ApplicationVersion: base.ApplicationVersion,
		ProtocolVersion:    base.ProtocolVersion, Capabilities: base.Capabilities,
		AcceptedTargets: base.AcceptedTargets, Scopes: base.Scopes,
		ExpiresAt: base.ExpiresAt, ApprovalToken: pending.ApprovalToken,
	}
}

func toAPIPollGrant(grant linking.TokenGrant) LinkPollResult {
	instance := toAPIInstance(grant.Instance)
	return LinkPollResult{
		Status:               LinkPollResultStatusLinked,
		AccessToken:          &grant.AccessToken,
		AccessTokenExpiresAt: &grant.AccessTokenExpiresAt,
		RefreshToken:         &grant.RefreshToken,
		Instance:             &instance,
	}
}

func toAPITokenGrant(grant linking.TokenGrant) InstanceTokenGrant {
	return InstanceTokenGrant{
		AccessToken:          grant.AccessToken,
		AccessTokenExpiresAt: grant.AccessTokenExpiresAt,
		RefreshToken:         grant.RefreshToken,
		Instance:             toAPIInstance(grant.Instance),
	}
}

func toAPIInstance(instance linking.Instance) LinkedInstance {
	var version *int
	if instance.ProtocolVersion > 0 {
		protocol := instance.ProtocolVersion
		version = &protocol
	}
	return LinkedInstance{
		Id:                 types.UUID(instance.ID),
		ApplicationName:    instance.ApplicationName,
		InstanceName:       instance.InstanceName,
		ApplicationVersion: apiApplicationVersion(instance.ApplicationVersion),
		ProtocolVersion:    version,
		Capabilities:       append([]string{}, instance.Capabilities...),
		AcceptedTargets:    append([]string{}, instance.AcceptedTargets...),
		Prefix:             instance.Prefix, Scopes: toAPIScopes(instance.Scopes),
		LinkedAt: instance.LinkedAt, LastSeenAt: instance.LastSeenAt,
		RevokedAt: instance.RevokedAt,
	}
}

func apiApplicationVersion(value string) *ApplicationVersion {
	if value == "" {
		return nil
	}
	version := ApplicationVersion(value)
	return &version
}

func apiCapabilities(values []string) InstanceCapabilities {
	result := make(InstanceCapabilities, len(values))
	for index, value := range values {
		result[index] = CapabilityId(value)
	}
	return result
}

func apiTargets(values []string) AcceptedTargets {
	result := make(AcceptedTargets, len(values))
	for index, value := range values {
		result[index] = ExportTargetId(value)
	}
	return result
}
