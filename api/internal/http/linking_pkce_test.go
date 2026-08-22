package http

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

type startedAuthorization struct {
	AuthorizationURL string    `json:"authorizationUrl"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type pendingAuthorization struct {
	ApplicationName    string    `json:"applicationName"`
	InstanceName       string    `json:"instanceName"`
	ApplicationVersion *string   `json:"applicationVersion"`
	ProtocolVersion    int       `json:"protocolVersion"`
	Capabilities       []string  `json:"capabilities"`
	AcceptedTargets    []string  `json:"acceptedTargets"`
	Scopes             []string  `json:"scopes"`
	ExpiresAt          time.Time `json:"expiresAt"`
}

func TestLoopbackPKCEReviewApprovalAndOneUseExchange(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	verifier := strings.Repeat("A", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	state := strings.Repeat("s", 43)
	redirectURI := "http://127.0.0.1:49152/illarin/callback"

	body := linkStartBody("Example client", "studio workstation", []string{"asset:receive"})
	body["state"] = state
	body["redirectUri"] = redirectURI
	body["codeChallenge"] = challenge
	body["codeChallengeMethod"] = "S256"
	startedRec := sendJSON(t, r, http.MethodPost, "/v1/link/authorizations", jsonText(t, body))
	assertNoStore(t, startedRec)
	if startedRec.Code != http.StatusCreated {
		t.Fatalf("start authorization status = %d, want 201. body: %s", startedRec.Code, startedRec.Body.String())
	}
	started := decodeResponse[startedAuthorization](t, startedRec)
	if started.ExpiresAt.Before(time.Now()) {
		t.Errorf("authorization already expired at %s", started.ExpiresAt)
	}
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	requestCode := authorizationURL.Query().Get("request")
	if requestCode == "" || authorizationURL.Scheme+"://"+authorizationURL.Host+authorizationURL.Path != testBrowserOrigin+"/link" {
		t.Fatalf("authorization URL = %q", started.AuthorizationURL)
	}

	reviewReq := httptest.NewRequest(
		http.MethodGet, "/v1/link/authorizations/"+requestCode, nil,
	)
	reviewRec := send(t, r, authorized(reviewReq, session))
	assertNoStore(t, reviewRec)
	if reviewRec.Code != http.StatusOK {
		t.Fatalf("review authorization status = %d, want 200. body: %s", reviewRec.Code, reviewRec.Body.String())
	}
	pending := decodeResponse[pendingAuthorization](t, reviewRec)
	if pending.ApplicationName != "Example client" || pending.InstanceName != "studio workstation" {
		t.Errorf("pending authorization identity = %q / %q", pending.ApplicationName, pending.InstanceName)
	}
	if !slices.Equal(pending.Capabilities, []string{"example.client:asset-install"}) ||
		!slices.Equal(pending.AcceptedTargets, []string{"portable-card-v1"}) ||
		!slices.Equal(pending.Scopes, []string{"asset:receive"}) {
		t.Errorf("pending authorization declaration = %+v", pending)
	}

	approveTarget := "/v1/link/authorizations/" + requestCode + "/approve"
	csrfCases := []struct {
		name   string
		origin string
		header string
	}{
		{name: "neither"},
		{name: "origin only", origin: testBrowserOrigin},
		{name: "header only", header: "1"},
		{name: "wrong origin", origin: "https://attacker.invalid", header: "1"},
	}
	for _, test := range csrfCases {
		req := httptest.NewRequest(http.MethodPost, approveTarget, nil)
		req.Header.Set("Origin", test.origin)
		req.Header.Set(linkRequestHeader, test.header)
		rec := send(t, r, authorized(req, session))
		assertNoStore(t, rec)
		if rec.Code != http.StatusForbidden {
			t.Errorf("approve with %s = %d, want 403", test.name, rec.Code)
		}
	}

	approved := send(t, r, browserRequest(t, http.MethodPost, approveTarget, nil, session))
	assertNoStore(t, approved)
	if approved.Code != http.StatusOK {
		t.Fatalf("approve authorization status = %d, want 200. body: %s", approved.Code, approved.Body.String())
	}
	var redirect struct {
		URL string `json:"redirectUrl"`
	}
	if err := json.Unmarshal(approved.Body.Bytes(), &redirect); err != nil {
		t.Fatalf("decode redirect: %v", err)
	}
	callback, err := url.Parse(redirect.URL)
	if err != nil {
		t.Fatalf("parse callback: %v", err)
	}
	if callback.Scheme+"://"+callback.Host+callback.Path != redirectURI {
		t.Errorf("redirect base = %q, want exact callback %q", callback.Scheme+"://"+callback.Host+callback.Path, redirectURI)
	}
	if callback.Query().Get("state") != state {
		t.Errorf("redirect state = %q, want %q", callback.Query().Get("state"), state)
	}
	authorizationCode := callback.Query().Get("code")
	if authorizationCode == "" || len(callback.Query()) != 2 {
		t.Fatalf("redirect query = %v, want only code and state", callback.Query())
	}

	exchange := func(code, candidateVerifier, candidateRedirect string) *httptest.ResponseRecorder {
		t.Helper()
		return sendJSON(t, r, http.MethodPost, "/v1/link/token", jsonText(t, map[string]string{
			"authorizationCode": code,
			"codeVerifier":      candidateVerifier,
			"redirectUri":       candidateRedirect,
		}))
	}
	wrongRedirect := exchange(authorizationCode, verifier,
		"http://127.0.0.1:49153/illarin/callback")
	assertNoStore(t, wrongRedirect)
	if wrongRedirect.Code != http.StatusBadRequest {
		t.Errorf("exchange with another loopback callback = %d, want 400", wrongRedirect.Code)
	}
	wrongVerifier := exchange(authorizationCode, strings.Repeat("B", 43), redirectURI)
	assertNoStore(t, wrongVerifier)
	if wrongVerifier.Code != http.StatusBadRequest {
		t.Errorf("exchange with wrong verifier = %d, want 400", wrongVerifier.Code)
	}

	exchanged := exchange(authorizationCode, verifier, redirectURI)
	assertNoStore(t, exchanged)
	if exchanged.Code != http.StatusOK {
		t.Fatalf("exact code exchange status = %d, want 200. body: %s", exchanged.Code, exchanged.Body.String())
	}
	grant := decodeResponse[tokenGrant](t, exchanged)
	if grant.AccessToken == "" || grant.RefreshToken == "" || grant.Instance.ID == "" {
		t.Fatalf("code exchange grant = %+v", grant)
	}
	if grant.Instance.ApplicationName != "Example client" || grant.Instance.InstanceName != "studio workstation" {
		t.Errorf("linked instance = %+v", grant.Instance)
	}

	secondExchange := exchange(authorizationCode, verifier, redirectURI)
	assertNoStore(t, secondExchange)
	if secondExchange.Code != http.StatusBadRequest {
		t.Errorf("second exchange status = %d, want 400", secondExchange.Code)
	}
}

func TestBrowserAuthorizationReviewsAreReadOnlyAndTheFirstDecisionBindsTheUser(t *testing.T) {
	r, firstSession, pool := newLinkingRouter(t)
	secondSession := addVerifiedLinkingUser(
		t, r, pool, "second.browser@example.com", "second.browser",
	)
	verifier := strings.Repeat("A", 43)
	digest := sha256.Sum256([]byte(verifier))
	body := linkStartBody("Example browser client", "review test", []string{"asset:receive"})
	body["state"] = strings.Repeat("s", 43)
	body["redirectUri"] = "http://127.0.0.1:49152/illarin/callback"
	body["codeChallenge"] = base64.RawURLEncoding.EncodeToString(digest[:])
	body["codeChallengeMethod"] = "S256"
	startedRec := sendJSON(t, r, http.MethodPost, "/v1/link/authorizations", jsonText(t, body))
	if startedRec.Code != http.StatusCreated {
		t.Fatalf("start authorization status = %d, want 201. body: %s", startedRec.Code, startedRec.Body.String())
	}
	started := decodeResponse[startedAuthorization](t, startedRec)
	authorizationURL, err := url.Parse(started.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}
	requestCode := authorizationURL.Query().Get("request")
	if requestCode == "" {
		t.Fatal("authorization URL has no request code")
	}

	review := func(session *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(
			http.MethodGet, "/v1/link/authorizations/"+requestCode, nil,
		)
		return send(t, r, authorized(req, session))
	}
	for index, session := range []*http.Cookie{firstSession, firstSession, secondSession} {
		rec := review(session)
		if rec.Code != http.StatusOK {
			t.Fatalf("review %d status = %d, want 200. body: %s", index+1, rec.Code, rec.Body.String())
		}
	}

	var reviewerMissing bool
	if err := pool.QueryRow(context.Background(), `
		select reviewed_by is null
		  from link_authorizations
		 where application_name = 'Example browser client' and instance_name = 'review test'
	`).Scan(&reviewerMissing); err != nil {
		t.Fatalf("read pending browser review state: %v", err)
	}
	if !reviewerMissing {
		t.Fatal("review GET claimed the browser authorization")
	}

	denyTarget := "/v1/link/authorizations/" + requestCode + "/deny"
	denied := send(t, r, browserRequest(
		t, http.MethodPost, denyTarget, nil, firstSession,
	))
	if denied.Code != http.StatusOK {
		t.Fatalf("first decision status = %d, want 200. body: %s", denied.Code, denied.Body.String())
	}
	var redirect struct {
		URL string `json:"redirectUrl"`
	}
	if err := json.Unmarshal(denied.Body.Bytes(), &redirect); err != nil {
		t.Fatalf("decode denial redirect: %v", err)
	}
	callback, err := url.Parse(redirect.URL)
	if err != nil || callback.Query().Get("error") != "access_denied" {
		t.Fatalf("denial redirect = %q, want access_denied", redirect.URL)
	}

	approved := send(t, r, browserRequest(
		t, http.MethodPost,
		"/v1/link/authorizations/"+requestCode+"/approve", nil, secondSession,
	))
	if approved.Code != http.StatusNotFound {
		t.Fatalf("second user's decision status = %d, want 404. body: %s", approved.Code, approved.Body.String())
	}
}

func TestSameDeviceLinkingRejectsNonLoopbackRedirects(t *testing.T) {
	r, _, _ := newLinkingRouter(t)
	verifier := strings.Repeat("A", 43)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])

	for _, redirectURI := range []string{
		"http://localhost:49152/illarin/callback",
		"https://client.example/callback",
		"http://127.0.0.1/illarin/callback",
		"http://127.0.0.1:49152/illarin/callback?next=/admin",
	} {
		body := linkStartBody("Example client", "studio workstation", []string{"asset:receive"})
		body["state"] = strings.Repeat("s", 43)
		body["redirectUri"] = redirectURI
		body["codeChallenge"] = challenge
		body["codeChallengeMethod"] = "S256"
		rec := sendJSON(t, r, http.MethodPost, "/v1/link/authorizations", jsonText(t, body))
		assertNoStore(t, rec)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("redirect %q status = %d, want 400. body: %s", redirectURI, rec.Code, rec.Body.String())
		}
	}
}
