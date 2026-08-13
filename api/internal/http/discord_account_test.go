package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	discordapi "github.com/Sillyfrogster/LumiHub/api/internal/discord"
)

type discordStub struct {
	profile account.DiscordProfile
}

func (s *discordStub) AuthorizationURL(state string) string {
	return "https://discord.example/authorize?state=" + url.QueryEscape(state)
}

func (s *discordStub) ExchangeProfile(context.Context, string) (account.DiscordProfile, error) {
	return s.profile, nil
}

func TestDiscordSignUpSeedsAnAvailableHandleAndStartsASession(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "discord-reader-1",
		Username:      "Storyteller",
		Email:         "reader@example.com",
		EmailVerified: true,
	}}
	r := newTestRouterWithDiscord(t, provider)
	signUp(t, r, "somebody-else@example.com", "storyteller")

	begin := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/auth/discord", nil))
	if begin.Code != http.StatusSeeOther {
		t.Fatalf("begin status = %d, want 303. body: %s", begin.Code, begin.Body.String())
	}
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorization redirect: %v", err)
	}
	state := authorize.Query().Get("state")
	if state == "" {
		t.Fatal("authorization redirect has no OAuth state")
	}

	callback := send(t, r, httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=accepted&state="+url.QueryEscape(state), nil))
	if callback.Code != http.StatusSeeOther {
		t.Fatalf("callback status = %d, want 303. body: %s", callback.Code, callback.Body.String())
	}
	if location := callback.Header().Get("Location"); location != "/browse" {
		t.Errorf("callback redirects to %q, want /browse", location)
	}
	cookies := callback.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("callback set %d cookies, want one session cookie", len(cookies))
	}
	stateAfterSignIn := sessionState(t, r, cookies[0])
	if stateAfterSignIn.Handle != "storyteller.2" || stateAfterSignIn.Email == nil ||
		*stateAfterSignIn.Email != "reader@example.com" || !stateAfterSignIn.EmailVerified ||
		!stateAfterSignIn.DiscordLinked || stateAfterSignIn.HasPassword {
		t.Errorf("Discord account = %+v", stateAfterSignIn)
	}
}

func TestDiscordResyncsItsOwnVerifiedAddress(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:  "discord-reader-2",
		Username: "QuietReader",
		Email:    "unverified@example.com",
	}}
	r := newTestRouterWithDiscord(t, provider)

	first := completeDiscordSignIn(t, r)
	firstState := sessionState(t, r, first.Result().Cookies()[0])
	if firstState.Email != nil || firstState.EmailVerified {
		t.Fatalf("unverified Discord address was accepted: %+v", firstState)
	}

	provider.profile.EmailVerified = true
	provider.profile.Email = "first.verified@example.com"
	filled := completeDiscordSignIn(t, r)
	filledState := sessionState(t, r, filled.Result().Cookies()[0])
	if filledState.Email == nil || *filledState.Email != "first.verified@example.com" ||
		!filledState.EmailVerified {
		t.Fatalf("verified Discord address did not fill the empty account: %+v", filledState)
	}

	provider.profile.Email = "updated@example.com"
	updated := completeDiscordSignIn(t, r)
	updatedState := sessionState(t, r, updated.Result().Cookies()[0])
	if updatedState.Email == nil || *updatedState.Email != "updated@example.com" ||
		!updatedState.EmailVerified {
		t.Errorf("Discord did not replace its own address: %+v", updatedState)
	}
}

func TestDiscordResyncRefusesAnAddressVerifiedOnAnotherAccount(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "discord-owner",
		Username:      "FirstOwner",
		Email:         "original@example.com",
		EmailVerified: true,
	}}
	r := newTestRouterWithDiscord(t, provider)
	original := completeDiscordSignIn(t, r).Result().Cookies()[0]

	provider.profile = account.DiscordProfile{
		Subject:       "discord-other",
		Username:      "OtherOwner",
		Email:         "claimed@example.com",
		EmailVerified: true,
	}
	completeDiscordSignIn(t, r)

	provider.profile = account.DiscordProfile{
		Subject:       "discord-owner",
		Username:      "FirstOwner",
		Email:         "claimed@example.com",
		EmailVerified: true,
	}
	refused := completeDiscordSignIn(t, r)
	if refused.Code != http.StatusSeeOther ||
		refused.Header().Get("Location") != "/sign-in?discord=email-conflict" {
		t.Fatalf("resync response = %d %q, want email-conflict redirect",
			refused.Code, refused.Header().Get("Location"))
	}
	if cookies := refused.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("refused resync set a session cookie: %+v", cookies)
	}
	current := sessionState(t, r, original)
	if current.Email == nil || *current.Email != "original@example.com" {
		t.Errorf("refused resync changed the existing address: %+v", current)
	}
}

func TestEmailSignUpRefusesAnAddressHeldByADiscordAccount(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "discord-first",
		Username:      "DiscordFirst",
		Email:         "same-person@example.com",
		EmailVerified: true,
	}}
	r := newTestRouterWithDiscord(t, provider)
	completeDiscordSignIn(t, r)

	refused := signUpRequest(t, r, "same-person@example.com", "second.account")
	if refused.Code != http.StatusConflict {
		t.Fatalf("email sign-up status = %d, want 409. body: %s",
			refused.Code, refused.Body.String())
	}
	var answer struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(refused.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	want := "That email belongs to a Discord account. Sign in with Discord and set a password."
	if answer.Error != want {
		t.Errorf("refusal = %q, want %q", answer.Error, want)
	}
}

func TestEmailAccountCanAttachDiscordWithoutReplacingItsAddress(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "fresh-discord-link",
		Username:      "ConnectedCreator",
		Email:         "discord-address@example.com",
		EmailVerified: true,
	}}
	r, outbox := newTestRouterWithDiscordAndOutbox(t, provider)
	session := signUp(t, r, "creator-address@example.com", "connected.creator")
	verifyMessage(t, r, outbox.messages[0])

	beginRequest := httptest.NewRequest(http.MethodGet, "/v1/auth/discord?intent=attach", nil)
	beginRequest.AddCookie(session)
	begin := send(t, r, beginRequest)
	if begin.Code != http.StatusSeeOther {
		t.Fatalf("attach begin status = %d, want 303. body: %s", begin.Code, begin.Body.String())
	}
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorization redirect: %v", err)
	}
	callback := send(t, r, httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=accepted&state="+
			url.QueryEscape(authorize.Query().Get("state")), nil))
	if callback.Code != http.StatusSeeOther ||
		callback.Header().Get("Location") != "/settings?discord=attached" {
		t.Fatalf("attach callback = %d %q, want attached redirect",
			callback.Code, callback.Header().Get("Location"))
	}
	if cookies := callback.Result().Cookies(); len(cookies) != 0 {
		t.Errorf("attach callback replaced the existing session: %+v", cookies)
	}
	current := sessionState(t, r, session)
	if current.Email == nil || *current.Email != "creator-address@example.com" ||
		!current.EmailVerified || !current.DiscordLinked || !current.HasPassword {
		t.Errorf("attached account = %+v", current)
	}
}

func TestAttachingAnAlreadyClaimedDiscordIdentityRevealsNoAccount(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "already-claimed",
		Username:      "FirstLink",
		Email:         "first-link@example.com",
		EmailVerified: true,
	}}
	r, outbox := newTestRouterWithDiscordAndOutbox(t, provider)
	completeDiscordSignIn(t, r)

	session := signUp(t, r, "second-link@example.com", "second.link")
	verifyMessage(t, r, outbox.messages[0])
	beginRequest := httptest.NewRequest(http.MethodGet, "/v1/auth/discord?intent=attach", nil)
	beginRequest.AddCookie(session)
	begin := send(t, r, beginRequest)
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorization redirect: %v", err)
	}
	refused := send(t, r, httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=accepted&state="+
			url.QueryEscape(authorize.Query().Get("state")), nil))
	if refused.Code != http.StatusSeeOther ||
		refused.Header().Get("Location") != "/settings?discord=claimed" {
		t.Fatalf("claimed attach = %d %q, want generic claimed redirect",
			refused.Code, refused.Header().Get("Location"))
	}
	if current := sessionState(t, r, session); current.DiscordLinked {
		t.Errorf("claimed Discord identity reached the second account: %+v", current)
	}
}

func TestDiscordCanDetachOnlyAfterAVerifiedEmailHasAPassword(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "detachable-discord",
		Username:      "Detachable",
		Email:         "detachable@example.com",
		EmailVerified: true,
	}}
	r, outbox := newTestRouterWithDiscordAndOutbox(t, provider)
	session := completeDiscordSignIn(t, r).Result().Cookies()[0]

	unsafeRequest := httptest.NewRequest(http.MethodDelete, "/v1/account/discord", nil)
	unsafeRequest.AddCookie(session)
	unsafe := send(t, r, unsafeRequest)
	if unsafe.Code != http.StatusConflict {
		t.Fatalf("Discord-only detach status = %d, want 409. body: %s",
			unsafe.Code, unsafe.Body.String())
	}
	if current := sessionState(t, r, session); !current.DiscordLinked {
		t.Errorf("refused detach removed Discord: %+v", current)
	}

	passwordRequest := httptest.NewRequest(http.MethodPut, "/v1/account/password",
		strings.NewReader(`{"password":"a new private password"}`))
	passwordRequest.Header.Set("Content-Type", "application/json")
	passwordRequest.AddCookie(session)
	password := send(t, r, passwordRequest)
	if password.Code != http.StatusOK {
		t.Fatalf("set password status = %d, want 200. body: %s",
			password.Code, password.Body.String())
	}

	detachRequest := httptest.NewRequest(http.MethodDelete, "/v1/account/discord", nil)
	detachRequest.AddCookie(session)
	detached := send(t, r, detachRequest)
	if detached.Code != http.StatusOK {
		t.Fatalf("safe detach status = %d, want 200. body: %s",
			detached.Code, detached.Body.String())
	}
	current := sessionState(t, r, session)
	if current.DiscordLinked || !current.HasPassword || !current.EmailVerified {
		t.Errorf("account after detach = %+v", current)
	}

	signedIn := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"detachable@example.com",
		"password":"a new private password"
	}`)
	if signedIn.Code != http.StatusOK {
		t.Errorf("email sign-in after detach = %d, want 200. body: %s",
			signedIn.Code, signedIn.Body.String())
	}

	newOwner := signUp(t, r, "new-owner@example.com", "new.owner")
	verifyMessage(t, r, outbox.messages[0])
	beginRequest := httptest.NewRequest(http.MethodGet, "/v1/auth/discord?intent=attach", nil)
	beginRequest.AddCookie(newOwner)
	begin := send(t, r, beginRequest)
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse released identity redirect: %v", err)
	}
	reused := send(t, r, httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=accepted&state="+
			url.QueryEscape(authorize.Query().Get("state")), nil))
	if reused.Code != http.StatusSeeOther ||
		reused.Header().Get("Location") != "/settings?discord=attached" {
		t.Fatalf("released identity attach = %d %q, want attached redirect",
			reused.Code, reused.Header().Get("Location"))
	}
	if current := sessionState(t, r, newOwner); !current.DiscordLinked {
		t.Errorf("released Discord identity was not reusable: %+v", current)
	}
}

func TestPasswordResetSetsTheFirstPasswordForADiscordAccount(t *testing.T) {
	provider := &discordStub{profile: account.DiscordProfile{
		Subject:       "recovery-discord",
		Username:      "RecoveryReader",
		Email:         "recover@example.com",
		EmailVerified: true,
	}}
	r, outbox := newTestRouterWithDiscordAndOutbox(t, provider)
	completeDiscordSignIn(t, r)

	requested := sendJSON(t, r, http.MethodPost, "/v1/auth/password-reset", `{
		"email":"recover@example.com"
	}`)
	if requested.Code != http.StatusNoContent {
		t.Fatalf("reset request status = %d, want 204. body: %s",
			requested.Code, requested.Body.String())
	}
	if len(outbox.passwordResets) != 1 ||
		outbox.passwordResets[0].address != "recover@example.com" {
		t.Fatalf("password reset outbox = %+v", outbox.passwordResets)
	}
	resetURL, err := url.Parse(outbox.passwordResets[0].link)
	if err != nil {
		t.Fatalf("parse password reset link: %v", err)
	}
	token := resetURL.Query().Get("token")
	completed := sendJSON(t, r, http.MethodPost, "/v1/auth/password-reset/complete", `{
		"token":"`+token+`",
		"password":"first account password"
	}`)
	if completed.Code != http.StatusNoContent {
		t.Fatalf("reset completion status = %d, want 204. body: %s",
			completed.Code, completed.Body.String())
	}
	reused := sendJSON(t, r, http.MethodPost, "/v1/auth/password-reset/complete", `{
		"token":"`+token+`",
		"password":"replacement"
	}`)
	if reused.Code != http.StatusBadRequest {
		t.Errorf("reused reset status = %d, want 400", reused.Code)
	}

	signedIn := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"recover@example.com",
		"password":"first account password"
	}`)
	if signedIn.Code != http.StatusOK {
		t.Errorf("first-password sign-in = %d, want 200. body: %s",
			signedIn.Code, signedIn.Body.String())
	}
}

func TestDiscordOAuthRequestsIdentifyAndEmailFromTheProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v10/oauth2/token":
			if err := request.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
			}
			if request.Form.Get("client_id") != "client-id" ||
				request.Form.Get("client_secret") != "client-secret" ||
				request.Form.Get("grant_type") != "authorization_code" ||
				request.Form.Get("code") != "provider-code" ||
				request.Form.Get("redirect_uri") != "http://localhost:3000/api/v1/auth/discord/callback" {
				t.Errorf("token form = %v", request.Form)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"access_token":"user-token","token_type":"Bearer"}`)
		case "/api/v10/users/@me":
			if authorization := request.Header.Get("Authorization"); authorization != "Bearer user-token" {
				t.Errorf("user authorization = %q", authorization)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"id":"provider-user",
				"username":"ProviderReader",
				"email":"provider@example.com",
				"verified":true
			}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer upstream.Close()

	provider, err := discordapi.NewClient(discordapi.Config{
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		RedirectURI:           "http://localhost:3000/api/v1/auth/discord/callback",
		AuthorizationEndpoint: upstream.URL + "/oauth2/authorize",
		TokenEndpoint:         upstream.URL + "/api/v10/oauth2/token",
		UserEndpoint:          upstream.URL + "/api/v10/users/@me",
	}, upstream.Client())
	if err != nil {
		t.Fatalf("create Discord client: %v", err)
	}
	r := newTestRouterWithDiscord(t, provider)

	begin := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/auth/discord", nil))
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	if authorize.Query().Get("scope") != "identify email" ||
		authorize.Query().Get("response_type") != "code" ||
		authorize.Query().Get("client_id") != "client-id" ||
		authorize.Query().Get("redirect_uri") != "http://localhost:3000/api/v1/auth/discord/callback" {
		t.Errorf("authorization query = %v", authorize.Query())
	}
	callback := send(t, r, httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=provider-code&state="+
			url.QueryEscape(authorize.Query().Get("state")), nil))
	if callback.Code != http.StatusSeeOther || len(callback.Result().Cookies()) != 1 {
		t.Fatalf("provider callback = %d, cookies %v", callback.Code, callback.Result().Cookies())
	}
	current := sessionState(t, r, callback.Result().Cookies()[0])
	if current.Email == nil || *current.Email != "provider@example.com" ||
		!current.EmailVerified || !current.DiscordLinked {
		t.Errorf("provider account = %+v", current)
	}
}

func verifyMessage(t *testing.T, r http.Handler, message verificationMessage) {
	t.Helper()
	verificationURL, err := url.Parse(message.link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	verified := sendJSON(t, r, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200. body: %s", verified.Code, verified.Body.String())
	}
}

func completeDiscordSignIn(t *testing.T, r http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	begin := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/auth/discord", nil))
	if begin.Code != http.StatusSeeOther {
		t.Fatalf("begin status = %d, want 303. body: %s", begin.Code, begin.Body.String())
	}
	authorize, err := url.Parse(begin.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse authorization redirect: %v", err)
	}
	state := authorize.Query().Get("state")
	callback := httptest.NewRequest(http.MethodGet,
		"/v1/auth/discord/callback?code=accepted&state="+url.QueryEscape(state), nil)
	return send(t, r, callback)
}
