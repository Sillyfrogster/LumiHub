package http

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
	"github.com/gin-gonic/gin"
	"github.com/oapi-codegen/runtime/types"
)

const (
	sessionCookieName     = "lumihub_session"
	oauthStateCookieName  = "lumihub_discord_state"
	oauthReturnCookieName = "lumihub_discord_return"
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
	returnTo := ""
	if intent == account.DiscordSignIn {
		returnTo = safeInternalPath(params.ReturnTo)
	}
	setOAuthReturnCookie(c, returnTo, authorization.Expires)
	c.Redirect(http.StatusSeeOther, authorization.URL)
}

func (h *Handlers) CompleteDiscord(c *gin.Context, params CompleteDiscordParams) {
	browserState, err := c.Cookie(oauthStateCookieName)
	if err != nil || subtle.ConstantTimeCompare([]byte(browserState), []byte(params.State)) != 1 {
		c.Redirect(http.StatusSeeOther, "/sign-in?discord=failed")
		return
	}
	returnTo := readOAuthReturnCookie(c)
	clearOAuthStateCookie(c)
	clearOAuthReturnCookie(c)
	if params.Error != nil || params.Code == nil {
		c.Redirect(http.StatusSeeOther, discordSignInDestination("cancelled", returnTo))
		return
	}
	completion, err := h.accounts.CompleteDiscord(
		c.Request.Context(), params.State, *params.Code,
	)
	attached := completion.Intent == account.DiscordAttach
	if err != nil {
		destination := discordSignInDestination("failed", returnTo)
		if attached {
			destination = "/settings?discord=failed"
		}
		switch {
		case errors.Is(err, account.ErrDiscordEmailConflict):
			if attached {
				destination = "/settings?discord=email-conflict"
			} else {
				destination = discordSignInDestination("email-conflict", returnTo)
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
	if returnTo == "" {
		returnTo = "/browse"
	}
	c.Redirect(http.StatusSeeOther, returnTo)
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

func setOAuthReturnCookie(c *gin.Context, destination string, expires time.Time) {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(destination))
	setCredentialCookie(c, oauthReturnCookieName, encoded, expires)
}

func readOAuthReturnCookie(c *gin.Context) string {
	encoded, err := c.Cookie(oauthReturnCookieName)
	if err != nil || encoded == "" {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	destination := string(decoded)
	return safeInternalPath(&destination)
}

func clearOAuthReturnCookie(c *gin.Context) {
	clearCredentialCookie(c, oauthReturnCookieName)
}

func safeInternalPath(candidate *string) string {
	if candidate == nil || !strings.HasPrefix(*candidate, "/") ||
		strings.HasPrefix(*candidate, "//") || strings.Contains(*candidate, "\\") ||
		strings.IndexFunc(*candidate, func(character rune) bool {
			return character < ' ' || character == '\u007f'
		}) >= 0 {
		return ""
	}
	return *candidate
}

func discordSignInDestination(status, returnTo string) string {
	query := url.Values{"discord": {status}}
	if returnTo != "" {
		query.Set("returnTo", returnTo)
	}
	return "/sign-in?" + query.Encode()
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

func (h *Handlers) uploadOwner(c *gin.Context) (account.Account, bool) {
	return h.verifiedAccount(c, "uploading")
}

// signedInAccount answers the account behind the session cookie, or writes the
// refusal and returns false. The action finishes the sentence "Sign in before".
func (h *Handlers) signedInAccount(c *gin.Context, action string) (account.Account, bool) {
	token, err := c.Cookie(sessionCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before " + action + "."})
		return account.Account{}, false
	}
	current, err := h.accounts.Current(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not check the signed-in account."})
		return account.Account{}, false
	}
	if current == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in before " + action + "."})
		return account.Account{}, false
	}
	return *current, true
}

// verifiedAccount answers the signed-in account only once its email is
// verified, and otherwise writes the refusal and returns false.
func (h *Handlers) verifiedAccount(c *gin.Context, action string) (account.Account, bool) {
	current, ok := h.signedInAccount(c, action)
	if !ok {
		return account.Account{}, false
	}
	if !current.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Verify your email before " + action + "."})
		return account.Account{}, false
	}
	return current, true
}
