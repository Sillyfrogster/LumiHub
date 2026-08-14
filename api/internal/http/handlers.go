package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"
)

// Handlers turns HTTP requests into catalog calls.
type Handlers struct {
	assets         *asset.Service
	accounts       *account.Service
	maxUploadBytes int64
}

func NewHandlers(assets *asset.Service, accounts *account.Service, maxUploadBytes int64) *Handlers {
	return &Handlers{assets: assets, accounts: accounts, maxUploadBytes: maxUploadBytes}
}

const (
	sessionCookieName    = "lumihub_session"
	oauthStateCookieName = "lumihub_discord_state"
)

func (h *Handlers) SignUp(c *gin.Context) {
	var request SignUpRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send an email, password and handle as JSON."})
		return
	}

	created, token, expires, err := h.accounts.SignUp(c.Request.Context(), account.SignUpInput{
		Email:    string(request.Email),
		Password: request.Password,
		Handle:   request.Handle,
	})
	if err != nil {
		h.accountError(c, err)
		return
	}
	setSessionCookie(c, token, expires)
	c.JSON(http.StatusCreated, toAPIAccount(created))
}

func (h *Handlers) SignIn(c *gin.Context) {
	var request SignInRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password is not correct."})
		return
	}
	current, token, expires, err := h.accounts.SignIn(
		c.Request.Context(), string(request.Email), request.Password,
	)
	if errors.Is(err, account.ErrCredentials) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email or password is not correct."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not sign in."})
		return
	}
	setSessionCookie(c, token, expires)
	c.JSON(http.StatusOK, toAPIAccount(current))
}

func (h *Handlers) BeginDiscord(c *gin.Context, params BeginDiscordParams) {
	intent := account.DiscordSignIn
	if params.Intent != nil && *params.Intent == Attach {
		intent = account.DiscordAttach
	}
	token, _ := c.Cookie(sessionCookieName)
	authorization, err := h.accounts.BeginDiscord(c.Request.Context(), token, intent)
	if errors.Is(err, account.ErrDiscordUnavailable) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Discord sign-in is not available."})
		return
	}
	if errors.Is(err, account.ErrUnauthorized) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before attaching Discord."})
		return
	}
	if errors.Is(err, account.ErrEmailUnverified) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Verify your email before attaching Discord."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not begin Discord sign-in."})
		return
	}
	setOAuthStateCookie(c, authorization.State, authorization.Expires)
	c.Redirect(http.StatusSeeOther, authorization.URL)
}

func (h *Handlers) CompleteDiscord(c *gin.Context, params CompleteDiscordParams) {
	browserState, err := c.Cookie(oauthStateCookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(browserState), []byte(params.State)) != 1 {
		c.Redirect(http.StatusSeeOther, "/sign-in?discord=failed")
		return
	}
	clearOAuthStateCookie(c)
	if params.Error != nil || params.Code == nil {
		c.Redirect(http.StatusSeeOther, "/sign-in?discord=cancelled")
		return
	}
	completion, err := h.accounts.CompleteDiscord(
		c.Request.Context(), params.State, *params.Code,
	)
	attached := completion.Intent == account.DiscordAttach
	if err != nil {
		destination := "/sign-in?discord=failed"
		if attached {
			destination = "/settings?discord=failed"
		}
		switch {
		case errors.Is(err, account.ErrDiscordEmailConflict):
			if attached {
				destination = "/settings?discord=email-conflict"
			} else {
				destination = "/sign-in?discord=email-conflict"
			}
		case errors.Is(err, account.ErrDiscordClaimed):
			destination = "/settings?discord=claimed"
		}
		c.Redirect(http.StatusSeeOther, destination)
		return
	}
	if attached {
		c.Redirect(http.StatusSeeOther, "/settings?discord=attached")
		return
	}
	setSessionCookie(c, completion.SessionToken, completion.SessionExpires)
	c.Redirect(http.StatusSeeOther, "/browse")
}

func (h *Handlers) SignOut(c *gin.Context) {
	token, _ := c.Cookie(sessionCookieName)
	if err := h.accounts.SignOut(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not sign out."})
		return
	}
	clearSessionCookie(c)
	c.Status(http.StatusNoContent)
}

func (h *Handlers) GetSession(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		c.JSON(http.StatusOK, SessionState{User: nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The session cookie could not be read."})
		return
	}

	current, err := h.accounts.Current(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the signed-in account."})
		return
	}
	if current == nil {
		c.JSON(http.StatusOK, SessionState{User: nil})
		return
	}
	user := toAPIAccount(*current)
	c.JSON(http.StatusOK, SessionState{User: &user})
}

func (h *Handlers) VerifyEmail(c *gin.Context) {
	var request VerifyEmailRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "The verification link is incomplete."})
		return
	}
	verified, err := h.accounts.VerifyEmail(c.Request.Context(), request.Token)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrVerification):
			c.JSON(http.StatusBadRequest, gin.H{"error": "This verification link is invalid or has expired."})
		case errors.Is(err, account.ErrEmailUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "Another account verified this email first."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not verify the email."})
		}
		return
	}
	c.JSON(http.StatusOK, toAPIAccount(verified))
}

func (h *Handlers) RequestPasswordReset(c *gin.Context) {
	var request PasswordResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the account email as JSON."})
		return
	}
	if err := h.accounts.RequestPasswordReset(c.Request.Context(), string(request.Email)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not request a password reset."})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) CompletePasswordReset(c *gin.Context) {
	var request CompletePasswordResetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the reset token and new password as JSON."})
		return
	}
	err := h.accounts.CompletePasswordReset(
		c.Request.Context(), request.Token, request.Password,
	)
	if err != nil {
		var field account.FieldError
		switch {
		case errors.As(err, &field):
			c.JSON(http.StatusBadRequest, gin.H{"error": field.Message, "field": field.Field})
		case errors.Is(err, account.ErrPasswordReset):
			c.JSON(http.StatusBadRequest, gin.H{"error": "This password reset link is invalid or has expired."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not reset the password."})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) RenameHandle(c *gin.Context) {
	var request RenameHandleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the new handle as JSON."})
		return
	}
	token, _ := c.Cookie(sessionCookieName)
	updated, err := h.accounts.RenameHandle(c.Request.Context(), token, request.Handle)
	if err != nil {
		var field account.FieldError
		switch {
		case errors.As(err, &field):
			c.JSON(http.StatusBadRequest, gin.H{"error": field.Message, "field": field.Field})
		case errors.Is(err, account.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before changing your handle."})
		case errors.Is(err, account.ErrEmailUnverified):
			c.JSON(http.StatusForbidden, gin.H{"error": "Verify your email before changing your handle."})
		case errors.Is(err, account.ErrHandleUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "That handle is not available."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not change the handle."})
		}
		return
	}
	c.JSON(http.StatusOK, toAPIAccount(updated))
}

func (h *Handlers) ChangeUnverifiedEmail(c *gin.Context) {
	var request ChangeEmailRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the corrected email as JSON."})
		return
	}
	token, _ := c.Cookie(sessionCookieName)
	updated, err := h.accounts.ChangeUnverifiedEmail(
		c.Request.Context(), token, string(request.Email),
	)
	if err != nil {
		var field account.FieldError
		switch {
		case errors.As(err, &field):
			c.JSON(http.StatusBadRequest, gin.H{"error": field.Message, "field": field.Field})
		case errors.Is(err, account.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before changing your email."})
		case errors.Is(err, account.ErrEmailUnavailable):
			c.JSON(http.StatusConflict, gin.H{"error": "An account already uses that email. Sign in instead."})
		case errors.Is(err, account.ErrEmailVerified):
			c.JSON(http.StatusConflict, gin.H{"error": "This email is already verified."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not change the email."})
		}
		return
	}
	c.JSON(http.StatusOK, toAPIAccount(updated))
}

func (h *Handlers) SetPassword(c *gin.Context) {
	var request PasswordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send the new password as JSON."})
		return
	}
	token, _ := c.Cookie(sessionCookieName)
	updated, err := h.accounts.SetPassword(c.Request.Context(), token, request.Password)
	if err != nil {
		var field account.FieldError
		switch {
		case errors.As(err, &field):
			c.JSON(http.StatusBadRequest, gin.H{"error": field.Message, "field": field.Field})
		case errors.Is(err, account.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before setting a password."})
		case errors.Is(err, account.ErrPasswordAlreadySet):
			c.JSON(http.StatusConflict, gin.H{
				"error": "This account already has a password. Use password recovery to replace it.",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not set the password."})
		}
		return
	}
	c.JSON(http.StatusOK, toAPIAccount(updated))
}

func (h *Handlers) SetNsfwVisibility(c *gin.Context) {
	var request NsfwVisibilityRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil || !request.Visibility.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose hidden, blurred or shown."})
		return
	}
	token, _ := c.Cookie(sessionCookieName)
	err := h.accounts.SetNSFWVisibility(
		c.Request.Context(), token, account.NSFWVisibility(request.Visibility),
	)
	if errors.Is(err, account.ErrUnauthorized) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before saving a content preference."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the content preference."})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DetachDiscord(c *gin.Context) {
	token, _ := c.Cookie(sessionCookieName)
	updated, err := h.accounts.DetachDiscord(c.Request.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, account.ErrUnauthorized):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before detaching Discord."})
		case errors.Is(err, account.ErrLastSignInMethod):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Verify an email and set a password before detaching Discord.",
			})
		case errors.Is(err, account.ErrDiscordNotLinked):
			c.JSON(http.StatusConflict, gin.H{"error": "Discord is not attached to this account."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not detach Discord."})
		}
		return
	}
	c.JSON(http.StatusOK, toAPIAccount(updated))
}

func (h *Handlers) GetProfile(c *gin.Context, handle string) {
	profile, err := h.accounts.Profile(c.Request.Context(), handle)
	if errors.Is(err, account.ErrProfileNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "No such profile."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read the profile."})
		return
	}
	c.JSON(http.StatusOK, Profile{Id: types.UUID(profile.ID), Handle: profile.Handle})
}

func (h *Handlers) accountError(c *gin.Context, err error) {
	var field account.FieldError
	switch {
	case errors.As(err, &field):
		c.JSON(http.StatusBadRequest, gin.H{"error": field.Message, "field": field.Field})
	case errors.Is(err, account.ErrHandleUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "That handle is not available."})
	case errors.Is(err, account.ErrEmailBelongsDiscord):
		c.JSON(http.StatusConflict, gin.H{
			"error": "That email belongs to a Discord account. Sign in with Discord and set a password.",
		})
	case errors.Is(err, account.ErrEmailUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": "An account already uses that email. Sign in instead."})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create the account."})
	}
}

func setSessionCookie(c *gin.Context, token string, expires time.Time) {
	setCredentialCookie(c, sessionCookieName, token, expires)
}

func clearSessionCookie(c *gin.Context) {
	clearCredentialCookie(c, sessionCookieName)
}

func setOAuthStateCookie(c *gin.Context, state string, expires time.Time) {
	setCredentialCookie(c, oauthStateCookieName, state, expires)
}

func clearOAuthStateCookie(c *gin.Context) {
	clearCredentialCookie(c, oauthStateCookieName)
}

func setCredentialCookie(c *gin.Context, name, value string, expires time.Time) {
	cookie := credentialCookie(c, name)
	cookie.Value = value
	cookie.Expires = expires
	cookie.MaxAge = int(time.Until(expires).Seconds())
	http.SetCookie(c.Writer, cookie)
}

func clearCredentialCookie(c *gin.Context, name string) {
	cookie := credentialCookie(c, name)
	cookie.MaxAge = -1
	http.SetCookie(c.Writer, cookie)
}

func credentialCookie(c *gin.Context, name string) *http.Cookie {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	return &http.Cookie{
		Name:     name,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func toAPIAccount(value account.Account) Account {
	result := Account{
		Id:            types.UUID(value.ID),
		Handle:        value.Handle,
		EmailVerified: value.EmailVerified,
		DiscordLinked: value.DiscordLinked,
		HasPassword:   value.HasPassword,
		Role:          AccountRole(value.Role),
	}
	if value.Email != nil {
		email := types.Email(*value.Email)
		result.Email = &email
	}
	return result
}

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

func (h *Handlers) CreateAsset(c *gin.Context) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}

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
	c.JSON(http.StatusOK, toAPIDetail(found, visibility))
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
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save discovery."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func (h *Handlers) SetFilePatch(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	var request FileFieldPatch
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send supported file fields as JSON."})
		return
	}
	patch := make(format.Patch)
	for field, value := range map[format.Field]*string{
		format.FieldDescription:             request.Description,
		format.FieldPersonality:             request.Personality,
		format.FieldScenario:                request.Scenario,
		format.FieldFirstMessage:            request.FirstMes,
		format.FieldSystemPrompt:            request.SystemPrompt,
		format.FieldPostHistoryInstructions: request.PostHistoryInstructions,
		format.FieldCreatorNotes:            request.CreatorNotes,
		format.FieldCharacterVersion:        request.CharacterVersion,
	} {
		if value != nil {
			patch[field] = *value
		}
	}
	err := h.assets.SetFilePatch(c.Request.Context(), asset.FilePatchInput{
		OwnerID: owner.ID, AssetID: uuid.UUID(id), Patch: patch,
	})
	switch {
	case errors.Is(err, asset.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
	case errors.Is(err, asset.ErrAssetFrozen):
		c.JSON(http.StatusConflict, gin.H{"error": "A withheld asset cannot be changed."})
	case errors.Is(err, asset.ErrPatchUnsupported):
		c.JSON(http.StatusConflict, gin.H{"error": "This file format cannot be patched."})
	case errors.Is(err, format.ErrInvalidPatch):
		c.JSON(http.StatusBadRequest, gin.H{"error": "The file patch is not valid."})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not save the file patch."})
	default:
		c.Status(http.StatusNoContent)
	}
}

func toAPIDetail(found asset.Detail, visibility asset.ContentVisibility) AssetDetail {
	tags := make([]AssetTag, 0, len(found.Tags))
	for _, tag := range found.Tags {
		tags = append(tags, AssetTag{Label: tag.Label, Value: tag.Value})
	}
	media := make([]AssetImage, 0, len(found.Media))
	for _, image := range found.Media {
		media = append(media, AssetImage{
			Id:        types.UUID(image.ID),
			Role:      AssetImageRole(image.Role),
			DetailUrl: image.DetailURL,
			ThumbUrl:  image.ThumbURL,
			Width:     image.Width,
			Height:    image.Height,
		})
	}
	return AssetDetail{
		Id:         types.UUID(found.ID),
		Kind:       AssetDetailKind(found.Kind),
		Name:       found.Name,
		Blurb:      found.Blurb,
		Tags:       tags,
		Creator:    found.Creator,
		IsNsfw:     found.IsNSFW,
		Discovery:  AssetDetailDiscovery(found.Discovery),
		CreatedAt:  found.CreatedAt,
		Media:      media,
		Preview:    found.Preview,
		Visibility: AssetDetailVisibility(visibility),
		Withhold:   toAPIWithhold(found.Withhold),
	}
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

func (h *Handlers) CompleteIngest(c *gin.Context, id types.UUID) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}
	var request CompleteIngestRequest
	if err := decodeOneJSON(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Send only a kind and name as JSON."})
		return
	}
	operation, err := h.assets.CompleteIngest(
		c.Request.Context(), owner.ID, uuid.UUID(id), string(request.Kind), request.Name,
	)
	if errors.Is(err, asset.ErrIngestNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such ingest operation"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a kind and give the asset a name."})
		return
	}
	location := "/v1/ingests/" + operation.ID.String()
	c.Header("Location", location)
	c.JSON(http.StatusAccepted, toAPIIngest(operation))
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
	if operation.NeedsKind != nil {
		response["needsKind"] = gin.H{
			"kind": operation.NeedsKind.Kind,
			"name": operation.NeedsKind.Name,
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

func (h *Handlers) uploadOwner(c *gin.Context) (account.Account, bool) {
	token, err := c.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before uploading."})
		return account.Account{}, false
	}
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before uploading."})
		return account.Account{}, false
	}
	current, err := h.accounts.Current(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not check the signed-in account."})
		return account.Account{}, false
	}
	if current == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before uploading."})
		return account.Account{}, false
	}
	if !current.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Verify your email before uploading."})
		return account.Account{}, false
	}
	return *current, true
}

const (
	metadataPart = "metadata"
	filePart     = "file"
)

// readMetadata reads the catalog fields, which come first because the file is
// stored as it arrives and the fields have to be known by then.
func readMetadata(parts *multipart.Reader) (CreateAssetRequest, error) {
	part, err := nextPart(parts, metadataPart)
	if err != nil {
		return CreateAssetRequest{}, err
	}

	var metadata CreateAssetRequest
	if err := decodeOneJSON(io.LimitReader(part, 1<<20), &metadata); err != nil {
		return CreateAssetRequest{}, refusal{
			reason: "the " + metadataPart + " part is not valid JSON",
			cause:  err,
		}
	}
	return metadata, nil
}

func readMediaMetadata(parts *multipart.Reader) (AddMediaRequest, error) {
	part, err := nextPart(parts, metadataPart)
	if err != nil {
		return AddMediaRequest{}, err
	}
	var metadata AddMediaRequest
	if err := decodeOneJSON(io.LimitReader(part, 1<<20), &metadata); err != nil {
		return AddMediaRequest{}, refusal{
			reason: "the " + metadataPart + " part is not valid JSON",
			cause:  err,
		}
	}
	return metadata, nil
}

func decodeOneJSON(reader io.Reader, destination any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("more than one JSON value")
		}
		return err
	}
	return nil
}

func nextPart(parts *multipart.Reader, name string) (*multipart.Part, error) {
	part, err := parts.NextPart()
	if errors.Is(err, io.EOF) {
		return nil, refusal{reason: "the " + name + " part is missing", cause: err}
	}
	if err != nil {
		return nil, refusal{reason: "the form data could not be read", cause: err}
	}
	if part.FormName() != name {
		return nil, refusal{
			reason: fmt.Sprintf("expected the %s part here, found %q", name, part.FormName()),
			cause:  nil,
		}
	}
	return part, nil
}

func ingestInput(
	metadata CreateAssetRequest,
	filename string,
	file io.Reader,
	ownerID uuid.UUID,
) asset.IngestInput {
	in := asset.IngestInput{
		OwnerID:  ownerID,
		Filename: filename,
		File:     file,
	}
	if metadata.Name != nil {
		in.Name = metadata.Name
	}
	if metadata.Blurb != nil {
		in.Blurb = metadata.Blurb
	}
	if metadata.Tags != nil {
		in.Tags = metadata.Tags
	}
	if metadata.IsNsfw != nil {
		in.IsNSFW = metadata.IsNsfw
	}
	if metadata.Discovery != nil {
		in.Discovery = asset.Discovery(*metadata.Discovery)
	}
	return in
}

// refusal is a reason a request cannot be accepted, worded for whoever sent
// it. The cause stays attached so the ceiling can still be recognised through
// the layers that wrapped it.
type refusal struct {
	reason string
	cause  error
}

func (r refusal) Error() string { return r.reason }
func (r refusal) Unwrap() error { return r.cause }

func (h *Handlers) refuse(c *gin.Context, err error) {
	if errors.Is(err, format.ErrInvariant) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create the asset"})
		return
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("the upload is over the limit of %d bytes", h.maxUploadBytes),
		})
		return
	}

	var refused refusal
	if errors.As(err, &refused) {
		c.JSON(http.StatusBadRequest, gin.H{"error": refused.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": "could not create the asset"})
}

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
	var assetID, revisionID *types.UUID
	if found.AssetID != nil {
		converted := types.UUID(*found.AssetID)
		assetID = &converted
	}
	if found.RevisionID != nil {
		converted := types.UUID(*found.RevisionID)
		revisionID = &converted
	}
	return Media{
		Id:                types.UUID(found.ID),
		AssetId:           assetID,
		RevisionId:        revisionID,
		Role:              MediaRole(found.Role),
		Width:             found.Width,
		Height:            found.Height,
		DerivativeVersion: int(found.DerivativeVersion),
	}
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

/** Split "key=value" query values into facets. Anything without an = is ignored. */
func parseFacets(raw []string) []format.Facet {
	out := make([]format.Facet, 0, len(raw))
	for _, pair := range raw {
		for i := 0; i < len(pair); i++ {
			if pair[i] == '=' {
				out = append(out, format.Facet{Key: pair[:i], Value: pair[i+1:]})
				break
			}
		}
	}
	return out
}

func toAPI(a asset.Asset) Asset {
	return Asset{
		Id:                  types.UUID(a.ID),
		Kind:                a.Kind,
		PassthroughPlatform: a.PassthroughPlatform,
		Format:              a.Format,
		Name:                a.Name,
		Blurb:               a.Blurb,
		Tags:                a.Tags,
		IsNsfw:              a.IsNSFW,
		Discovery:           AssetDiscovery(a.Discovery),
		CreatedAt:           a.CreatedAt,
	}
}
