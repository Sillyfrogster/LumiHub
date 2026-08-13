package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
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
	}
	if value.Email != nil {
		email := types.Email(*value.Email)
		result.Email = &email
	}
	return result
}

func (h *Handlers) ListAssets(c *gin.Context, params ListAssetsParams) {
	f := asset.ListFilter{}

	if params.Kind != nil {
		f.Kind = *params.Kind
	}
	if params.Platform != nil {
		f.Platform, f.PlatformSet = params.Platform, true
	}
	if params.Tag != nil {
		f.Tags = *params.Tag
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

	found, err := h.assets.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list assets"})
		return
	}

	items := make([]Asset, 0, len(found))
	for _, a := range found {
		items = append(items, toAPI(a))
	}
	c.JSON(http.StatusOK, AssetList{Items: items})
}

func (h *Handlers) CreateAsset(c *gin.Context) {
	owner, ok := h.uploadOwner(c)
	if !ok {
		return
	}

	// Refused before the body is read, so a hopeless upload is never received.
	if c.Request.ContentLength > h.maxUploadBytes {
		h.refuse(c, errOverCeiling)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxUploadBytes)

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

	created, err := h.assets.Create(c.Request.Context(), createInput(metadata, file, owner.ID))
	if err != nil {
		h.refuse(c, err)
		return
	}

	c.JSON(http.StatusCreated, toAPI(created))
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
	if err := json.NewDecoder(part).Decode(&metadata); err != nil {
		return CreateAssetRequest{}, refusal{
			reason: "the " + metadataPart + " part is not valid JSON",
			cause:  err,
		}
	}
	return metadata, nil
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

func createInput(metadata CreateAssetRequest, file io.Reader, ownerID uuid.UUID) asset.CreateInput {
	in := asset.CreateInput{
		OwnerID:  ownerID,
		Filename: metadata.Filename,
		File:     file,
		Name:     metadata.Name,
	}
	if metadata.Kind != nil {
		in.Kind = *metadata.Kind
	}
	if metadata.Description != nil {
		in.Description = *metadata.Description
	}
	if metadata.Tags != nil {
		in.Tags = *metadata.Tags
	}
	if metadata.IsNsfw != nil {
		in.IsNSFW = *metadata.IsNsfw
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

// errOverCeiling is a request that declares itself too large to be worth
// reading.
var errOverCeiling = errors.New("upload over the ceiling")

func (h *Handlers) refuse(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) || errors.Is(err, errOverCeiling) {
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

func (h *Handlers) DownloadOriginal(c *gin.Context, id types.UUID) {
	rc, err := h.assets.OpenOriginal(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, asset.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no such asset"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read the file"})
		return
	}
	defer rc.Close()

	// Originals always download. Serving a stored media type would let an
	// uploaded HTML file run as a page on this site.
	c.Header("Content-Disposition", `attachment; filename="original"`)
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, -1, "application/octet-stream", rc, nil)
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
		Description:         a.Description,
		Tags:                a.Tags,
		IsNsfw:              a.IsNSFW,
		Discovery:           AssetDiscovery(a.Discovery),
		CreatedAt:           a.CreatedAt,
	}
}
