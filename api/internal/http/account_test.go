package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignUpStartsAnUnverifiedSessionAndSendsAVerificationLink(t *testing.T) {
	outbox := &verificationOutbox{}
	r := newTestRouterWithSender(t, 1<<20, DefaultDeadlines(), outbox)

	rec := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-up", `{
		"email":"reader@example.com",
		"password":"correct horse battery staple",
		"handle":"book.worm"
	}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201. body: %s", rec.Code, rec.Body.String())
	}
	if len(outbox.messages) != 1 {
		t.Fatalf("sent %d verification messages, want 1", len(outbox.messages))
	}
	if message := outbox.messages[0]; message.address != "reader@example.com" ||
		!strings.HasPrefix(message.link, "http://localhost:3000/verify-email?token=") {
		t.Errorf("verification message = %+v", message)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("set %d cookies, want one session cookie", len(cookies))
	}
	session := cookies[0]
	if session.Name != sessionCookieName || !session.HttpOnly || session.Path != "/" ||
		session.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie = %+v", session)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.AddCookie(session)
	state := send(t, r, req)
	if state.Code != http.StatusOK {
		t.Fatalf("session status = %d, want 200. body: %s", state.Code, state.Body.String())
	}
	var got struct {
		User *struct {
			ID            uuid.UUID `json:"id"`
			Handle        string    `json:"handle"`
			Email         *string   `json:"email"`
			EmailVerified bool      `json:"emailVerified"`
		} `json:"user"`
	}
	if err := json.Unmarshal(state.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if got.User == nil || got.User.ID == uuid.Nil || got.User.Handle != "book.worm" ||
		got.User.Email == nil || *got.User.Email != "reader@example.com" || got.User.EmailVerified {
		t.Errorf("session user = %+v", got.User)
	}
}

func TestSignUpAcceptsAnyNonemptyPassword(t *testing.T) {
	r := newTestRouter(t)

	short := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-up", `{
		"email":"short.password@example.com",
		"password":"x",
		"handle":"short.password"
	}`)
	if short.Code != http.StatusCreated {
		t.Errorf("one-character password status = %d, want 201. body: %s",
			short.Code, short.Body.String())
	}

	longPassword := strings.Repeat("long password ", 16)
	long := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-up", `{
		"email":"long.password@example.com",
		"password":"`+longPassword+`",
		"handle":"long.password"
	}`)
	if long.Code != http.StatusCreated {
		t.Errorf("long password status = %d, want 201. body: %s",
			long.Code, long.Body.String())
	}
	longSignIn := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"long.password@example.com",
		"password":"`+longPassword+`"
	}`)
	if longSignIn.Code != http.StatusOK {
		t.Errorf("long password sign-in status = %d, want 200. body: %s",
			longSignIn.Code, longSignIn.Body.String())
	}

	empty := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-up", `{
		"email":"empty.password@example.com",
		"password":"",
		"handle":"empty.password"
	}`)
	if empty.Code != http.StatusBadRequest {
		t.Errorf("empty password status = %d, want 400. body: %s",
			empty.Code, empty.Body.String())
	}
}

func TestSignUpAcceptsOnlyTheHandleVocabulary(t *testing.T) {
	r := newTestRouter(t)
	invalid := []string{
		"ab",
		strings.Repeat("a", 33),
		"UPPER",
		"has-hyphen",
		"12345",
		"._._",
	}
	for _, handle := range invalid {
		t.Run(handle, func(t *testing.T) {
			rec := signUpRequest(t, r, "handles@example.com", handle)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("handle %q status = %d, want 400. body: %s",
					handle, rec.Code, rec.Body.String())
			}
		})
	}

	mixedNumbersAndPunctuation := signUpRequest(t, r, "mixed@example.com", "1._")
	if mixedNumbersAndPunctuation.Code != http.StatusCreated {
		t.Errorf("mixed number and punctuation status = %d, want 201. body: %s",
			mixedNumbersAndPunctuation.Code, mixedNumbersAndPunctuation.Body.String())
	}
}

func sendJSON(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return send(t, handler, req)
}

func TestTheFirstAccountToVerifyAnAddressClaimsIt(t *testing.T) {
	outbox := &verificationOutbox{}
	r := newTestRouterWithSender(t, 1<<20, DefaultDeadlines(), outbox)

	first := signUp(t, r, "shared@example.com", "first.reader")
	second := signUp(t, r, "shared@example.com", "second.reader")
	if len(outbox.messages) != 2 {
		t.Fatalf("sent %d verification messages, want 2", len(outbox.messages))
	}

	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	verified := sendJSON(t, r, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200. body: %s", verified.Code, verified.Body.String())
	}

	firstState := sessionState(t, r, first)
	if firstState.Email == nil || *firstState.Email != "shared@example.com" || !firstState.EmailVerified {
		t.Errorf("winner = %+v", firstState)
	}
	secondState := sessionState(t, r, second)
	if secondState.Email != nil || secondState.EmailVerified {
		t.Errorf("pending copy was not cleared: %+v", secondState)
	}

	losingURL, err := url.Parse(outbox.messages[1].link)
	if err != nil {
		t.Fatalf("parse second verification link: %v", err)
	}
	losingVerification := sendJSON(t, r, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+losingURL.Query().Get("token")+`"}`)
	if losingVerification.Code != http.StatusBadRequest {
		t.Errorf("losing verification status = %d, want 400", losingVerification.Code)
	}

	alreadyClaimed := signUpRequest(t, r, "shared@example.com", "third.reader")
	if alreadyClaimed.Code != http.StatusConflict {
		t.Errorf("signup on verified address = %d, want 409. body: %s",
			alreadyClaimed.Code, alreadyClaimed.Body.String())
	}
}

func TestConcurrentVerificationProducesOneWinnerWithoutDeadlock(t *testing.T) {
	outbox := &verificationOutbox{}
	r, pool := newTestRouterWithSenderAndPool(t, 1<<20, DefaultDeadlines(), outbox)
	signUp(t, r, "race@example.com", "race.first")
	signUp(t, r, "race@example.com", "race.second")

	tokens := make([]string, 0, len(outbox.messages))
	for _, message := range outbox.messages {
		verificationURL, err := url.Parse(message.link)
		if err != nil {
			t.Fatalf("parse verification link: %v", err)
		}
		tokens = append(tokens, verificationURL.Query().Get("token"))
	}

	ctx := context.Background()
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin advisory lock blocker: %v", err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `select pg_advisory_xact_lock(
		hashtextextended('lumihub-email:' || $1::text, 0)
	)`, "race@example.com"); err != nil {
		t.Fatalf("hold email lock: %v", err)
	}

	start := make(chan struct{})
	ready := make(chan struct{}, len(tokens))
	statuses := make(chan int, len(tokens))
	for _, token := range tokens {
		go func() {
			ready <- struct{}{}
			<-start
			req := httptest.NewRequest(http.MethodPost, "/v1/auth/verify-email",
				strings.NewReader(`{"token":"`+token+`"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			statuses <- rec.Code
		}()
	}
	for range tokens {
		<-ready
	}
	close(start)

	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(ctx, `select count(*) from pg_locks
			where locktype = 'advisory' and not granted`).Scan(&waiting); err != nil {
			t.Fatalf("count waiting verification requests: %v", err)
		}
		if waiting >= len(tokens) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d verification requests reached the shared email lock", waiting)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatalf("release email lock: %v", err)
	}

	counts := map[int]int{}
	for range tokens {
		select {
		case status := <-statuses:
			counts[status]++
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent verification did not finish")
		}
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusBadRequest] != 1 {
		t.Errorf("verification statuses = %v, want one 200 and one 400", counts)
	}
}

type accountState struct {
	Handle        string  `json:"handle"`
	Email         *string `json:"email"`
	EmailVerified bool    `json:"emailVerified"`
	DiscordLinked bool    `json:"discordLinked"`
	HasPassword   bool    `json:"hasPassword"`
}

func signUp(t *testing.T, r http.Handler, email, handle string) *http.Cookie {
	t.Helper()
	rec := signUpRequest(t, r, email, handle)
	if rec.Code != http.StatusCreated {
		t.Fatalf("sign up %s status = %d, want 201. body: %s", handle, rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()[0]
}

func signUpRequest(t *testing.T, r http.Handler, email, handle string) *httptest.ResponseRecorder {
	t.Helper()
	return sendJSON(t, r, http.MethodPost, "/v1/auth/sign-up", `{
		"email":"`+email+`",
		"password":"correct horse battery staple",
		"handle":"`+handle+`"
	}`)
}

func sessionState(t *testing.T, r http.Handler, session *http.Cookie) accountState {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.AddCookie(session)
	rec := send(t, r, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var state struct {
		User accountState `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	return state.User
}

func TestAnUnverifiedAccountCanSignOutAndBackIn(t *testing.T) {
	r := newTestRouter(t)
	session := signUp(t, r, "Return.Reader@example.com", "return.reader")

	signOutRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/sign-out", nil)
	signOutRequest.AddCookie(session)
	signedOut := send(t, r, signOutRequest)
	if signedOut.Code != http.StatusNoContent {
		t.Fatalf("sign out status = %d, want 204. body: %s", signedOut.Code, signedOut.Body.String())
	}
	if cleared := signedOut.Result().Cookies(); len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Errorf("cleared cookie = %+v", cleared)
	}
	if current := sessionState(t, r, session); current.Email != nil {
		t.Errorf("signed-out session still reaches %+v", current)
	}

	wrongPassword := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"return.reader@example.com",
		"password":"this is not the password"
	}`)
	if wrongPassword.Code != http.StatusUnauthorized {
		t.Errorf("wrong password status = %d, want 401", wrongPassword.Code)
	}

	signedIn := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"RETURN.READER@example.com",
		"password":"correct horse battery staple"
	}`)
	if signedIn.Code != http.StatusOK {
		t.Fatalf("sign in status = %d, want 200. body: %s", signedIn.Code, signedIn.Body.String())
	}
	state := sessionState(t, r, signedIn.Result().Cookies()[0])
	if state.Email == nil || *state.Email != "return.reader@example.com" || state.EmailVerified {
		t.Errorf("signed-in account = %+v", state)
	}
}

func TestRenamingAHandleRetiresTheOldProfileWithoutARedirect(t *testing.T) {
	outbox := &verificationOutbox{}
	r := newTestRouterWithSender(t, 1<<20, DefaultDeadlines(), outbox)
	session := signUp(t, r, "rename@example.com", "first.handle")

	renameRequest := httptest.NewRequest(http.MethodPatch, "/v1/account/handle",
		strings.NewReader(`{"handle":"second.handle"}`))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(session)
	if unverified := send(t, r, renameRequest); unverified.Code != http.StatusForbidden {
		t.Fatalf("unverified rename status = %d, want 403. body: %s",
			unverified.Code, unverified.Body.String())
	}

	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	verified := sendJSON(t, r, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200. body: %s", verified.Code, verified.Body.String())
	}

	renamedRequest := httptest.NewRequest(http.MethodPatch, "/v1/account/handle",
		strings.NewReader(`{"handle":"second.handle"}`))
	renamedRequest.Header.Set("Content-Type", "application/json")
	renamedRequest.AddCookie(session)
	renamed := send(t, r, renamedRequest)
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename status = %d, want 200. body: %s", renamed.Code, renamed.Body.String())
	}

	oldProfile := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/profiles/first.handle", nil))
	if oldProfile.Code != http.StatusNotFound {
		t.Errorf("old profile status = %d, want 404", oldProfile.Code)
	}
	if location := oldProfile.Header().Get("Location"); location != "" {
		t.Errorf("old profile redirects to %q", location)
	}
	newProfile := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/profiles/second.handle", nil))
	if newProfile.Code != http.StatusOK {
		t.Errorf("new profile status = %d, want 200. body: %s", newProfile.Code, newProfile.Body.String())
	}

	reused := signUpRequest(t, r, "reuse@example.com", "first.handle")
	if reused.Code != http.StatusConflict {
		t.Errorf("retired handle signup = %d, want 409. body: %s", reused.Code, reused.Body.String())
	}
}

func TestOnlyAVerifiedAccountCanUpload(t *testing.T) {
	outbox := &verificationOutbox{}
	r := newTestRouterWithSender(t, 1<<20, DefaultDeadlines(), outbox)
	unverified := signUp(t, r, "uploader@example.com", "new.uploader")

	if browse := send(t, r, httptest.NewRequest(http.MethodGet, "/v1/assets", nil)); browse.Code != http.StatusOK {
		t.Errorf("unverified browse status = %d, want 200", browse.Code)
	}
	withoutSession := send(t, r, uploadRequest(t, exampleMetadata("Anonymous"), []byte("file")))
	if withoutSession.Code != http.StatusUnauthorized {
		t.Errorf("anonymous upload status = %d, want 401", withoutSession.Code)
	}
	unverifiedUpload := uploadRequest(t, exampleMetadata("Unverified"), []byte("file"))
	unverifiedUpload.AddCookie(unverified)
	if rec := send(t, r, unverifiedUpload); rec.Code != http.StatusForbidden {
		t.Errorf("unverified upload status = %d, want 403. body: %s", rec.Code, rec.Body.String())
	}

	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	verified := sendJSON(t, r, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want 200. body: %s", verified.Code, verified.Body.String())
	}

	verifiedUpload := uploadRequest(t, exampleMetadata("Verified"), []byte("file"))
	verifiedUpload.AddCookie(unverified)
	created := send(t, r, verifiedUpload)
	if created.Code != http.StatusCreated {
		t.Fatalf("verified upload status = %d, want 201. body: %s", created.Code, created.Body.String())
	}
	var asset struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}

	viewer := signUp(t, r, "viewer@example.com", "plain.viewer")
	download := httptest.NewRequest(http.MethodGet, "/v1/assets/"+asset.ID+"/original", nil)
	download.AddCookie(viewer)
	if viewed := send(t, r, download); viewed.Code != http.StatusOK {
		t.Errorf("unverified view status = %d, want 200. body: %s", viewed.Code, viewed.Body.String())
	}
}

func TestAnUnverifiedAccountCanCorrectItsEmail(t *testing.T) {
	outbox := &verificationOutbox{}
	r := newTestRouterWithSender(t, 1<<20, DefaultDeadlines(), outbox)
	session := signUp(t, r, "mistyped@example.com", "careful.creator")

	change := httptest.NewRequest(http.MethodPatch, "/v1/account/email",
		strings.NewReader(`{"email":"correct@example.com"}`))
	change.Header.Set("Content-Type", "application/json")
	change.AddCookie(session)
	changed := send(t, r, change)
	if changed.Code != http.StatusOK {
		t.Fatalf("change email status = %d, want 200. body: %s", changed.Code, changed.Body.String())
	}
	if len(outbox.messages) != 2 || outbox.messages[1].address != "correct@example.com" {
		t.Fatalf("verification outbox = %+v", outbox.messages)
	}
	state := sessionState(t, r, session)
	if state.Email == nil || *state.Email != "correct@example.com" || state.EmailVerified {
		t.Errorf("account after correction = %+v", state)
	}

	oldAddress := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"mistyped@example.com",
		"password":"correct horse battery staple"
	}`)
	if oldAddress.Code != http.StatusUnauthorized {
		t.Errorf("old address sign in = %d, want 401", oldAddress.Code)
	}
	newAddress := sendJSON(t, r, http.MethodPost, "/v1/auth/sign-in", `{
		"email":"correct@example.com",
		"password":"correct horse battery staple"
	}`)
	if newAddress.Code != http.StatusOK {
		t.Errorf("corrected address sign in = %d, want 200. body: %s",
			newAddress.Code, newAddress.Body.String())
	}
}
