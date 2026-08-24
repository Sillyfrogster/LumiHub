package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/delivery"
	"github.com/Sillyfrogster/Illarin/api/internal/testdb"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testBrowserOrigin = "http://localhost:3000"

var testLinkHMACKey = []byte("01234567890123456789012345678901")

type startedLink struct {
	DeviceCode      string    `json:"deviceCode"`
	UserCode        string    `json:"userCode"`
	VerificationURL string    `json:"verificationUrl"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Interval        int       `json:"interval"`
}

type linkedInstance struct {
	ID                 string     `json:"id"`
	ApplicationName    string     `json:"applicationName"`
	InstanceName       string     `json:"instanceName"`
	ApplicationVersion *string    `json:"applicationVersion"`
	ProtocolVersion    *int       `json:"protocolVersion"`
	Capabilities       []string   `json:"capabilities"`
	AcceptedTargets    []string   `json:"acceptedTargets"`
	Prefix             string     `json:"prefix"`
	Scopes             []string   `json:"scopes"`
	LinkedAt           time.Time  `json:"linkedAt"`
	LastSeenAt         *time.Time `json:"lastSeenAt"`
	RevokedAt          *time.Time `json:"revokedAt"`
}

type tokenGrant struct {
	AccessToken          string         `json:"accessToken"`
	AccessTokenExpiresAt time.Time      `json:"accessTokenExpiresAt"`
	RefreshToken         string         `json:"refreshToken"`
	Instance             linkedInstance `json:"instance"`
}

type polledLink struct {
	Status               string          `json:"status"`
	AccessToken          *string         `json:"accessToken"`
	AccessTokenExpiresAt *time.Time      `json:"accessTokenExpiresAt"`
	RefreshToken         *string         `json:"refreshToken"`
	Instance             *linkedInstance `json:"instance"`
}

type pendingDeviceLink struct {
	ApplicationName    string    `json:"applicationName"`
	InstanceName       string    `json:"instanceName"`
	ApplicationVersion *string   `json:"applicationVersion"`
	ProtocolVersion    int       `json:"protocolVersion"`
	Capabilities       []string  `json:"capabilities"`
	AcceptedTargets    []string  `json:"acceptedTargets"`
	Scopes             []string  `json:"scopes"`
	ExpiresAt          time.Time `json:"expiresAt"`
	ApprovalToken      string    `json:"approvalToken"`
}

func newLinkingRouter(t *testing.T) (*gin.Engine, *http.Cookie, *pgxpool.Pool) {
	t.Helper()
	return newLinkingRouterWith(t, testDeliverySettings())
}

func newLinkingRouterWith(
	t *testing.T,
	settings delivery.Settings,
) (*gin.Engine, *http.Cookie, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testdb.Connect(t)
	outbox := &verificationOutbox{}
	handlers := newTestHandlersWithDelivery(t, pool, 1<<20, outbox, settings)
	router := registerTestRouter(t, handlers, DefaultDeadlines())

	session := signUp(t, router, "creator@example.com", "linking.creator")
	link, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	rec := sendJSON(t, router, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+link.Query().Get("token")+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify creator: %d %s", rec.Code, rec.Body.String())
	}
	return router, session, pool
}

func linkStartBody(application, instance string, scopes []string) map[string]any {
	return map[string]any{
		"applicationName":    application,
		"instanceName":       instance,
		"applicationVersion": "1.0.0",
		"protocolVersion":    1,
		"capabilities":       []string{"example.client:asset-install"},
		"acceptedTargets":    []string{"portable-card-v1"},
		"scopes":             scopes,
	}
}

func jsonText(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
	return string(encoded)
}

func decodeResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(rec.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response: %v. body: %s", err, rec.Body.String())
	}
	return value
}

func assertNoStore(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", cache)
	}
}

func startLink(t *testing.T, r *gin.Engine, body map[string]any) (startedLink, map[string]json.RawMessage) {
	t.Helper()
	rec := sendJSON(t, r, http.MethodPost, "/v1/link/requests", jsonText(t, body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("start link status = %d, want 201. body: %s", rec.Code, rec.Body.String())
	}
	assertNoStore(t, rec)
	started := decodeResponse[startedLink](t, rec)
	return started, decodeResponse[map[string]json.RawMessage](t, rec)
}

func reviewDeviceLink(
	t *testing.T,
	r *gin.Engine,
	session *http.Cookie,
	userCode string,
) (*httptest.ResponseRecorder, pendingDeviceLink) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/link/requests/"+userCode, nil)
	rec := send(t, r, authorized(req, session))
	assertNoStore(t, rec)
	if rec.Code != http.StatusOK {
		return rec, pendingDeviceLink{}
	}
	return rec, decodeResponse[pendingDeviceLink](t, rec)
}

func addVerifiedLinkingUser(
	t *testing.T,
	r *gin.Engine,
	pool *pgxpool.Pool,
	email string,
	handle string,
) *http.Cookie {
	t.Helper()
	session := signUp(t, r, email, handle)
	if _, err := pool.Exec(
		context.Background(),
		`update users set email_verified_at = now() where email = $1`,
		email,
	); err != nil {
		t.Fatalf("verify second linking user: %v", err)
	}
	return session
}

func browserRequest(
	t *testing.T,
	method string,
	target string,
	body any,
	session *http.Cookie,
) *http.Request {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(jsonText(t, body)))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", testBrowserOrigin)
	req.Header.Set(browserMutationHeader, "1")
	return authorized(req, session)
}

func poll(t *testing.T, r *gin.Engine, deviceCode string) *httptest.ResponseRecorder {
	t.Helper()
	rec := sendJSON(t, r, http.MethodPost, "/v1/link/poll",
		jsonText(t, map[string]string{"deviceCode": deviceCode}))
	assertNoStore(t, rec)
	return rec
}

func linkDeviceInstance(
	t *testing.T,
	r *gin.Engine,
	session *http.Cookie,
	application string,
	instance string,
	scopes []string,
) tokenGrant {
	t.Helper()
	started, _ := startLink(t, r, linkStartBody(application, instance, scopes))
	review, pending := reviewDeviceLink(t, r, session, started.UserCode)
	if review.Code != http.StatusOK || pending.ApprovalToken == "" {
		t.Fatalf("review status = %d, approval token = %q", review.Code, pending.ApprovalToken)
	}
	approve := send(t, r, browserRequest(
		t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		map[string]string{"approvalToken": pending.ApprovalToken}, session,
	))
	assertNoStore(t, approve)
	if approve.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200. body: %s", approve.Code, approve.Body.String())
	}
	rec := poll(t, r, started.DeviceCode)
	if rec.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	result := decodeResponse[polledLink](t, rec)
	if result.Status != "linked" || result.AccessToken == nil || result.RefreshToken == nil || result.Instance == nil {
		t.Fatalf("linked poll = %+v, want an instance and a token pair", result)
	}
	if result.AccessTokenExpiresAt == nil {
		t.Fatal("linked poll has no access-token expiry")
	}
	return tokenGrant{
		AccessToken:          *result.AccessToken,
		AccessTokenExpiresAt: *result.AccessTokenExpiresAt,
		RefreshToken:         *result.RefreshToken,
		Instance:             *result.Instance,
	}
}

func asInstance(t *testing.T, method, target, token string, body any) *http.Request {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(jsonText(t, body)))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func listInstances(t *testing.T, r *gin.Engine, session *http.Cookie) []linkedInstance {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/instances", nil)
	rec := send(t, r, authorized(req, session))
	assertNoStore(t, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("list instances status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []linkedInstance `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode instances: %v", err)
	}
	return list.Items
}

func instanceByID(t *testing.T, instances []linkedInstance, id string) linkedInstance {
	t.Helper()
	for _, instance := range instances {
		if instance.ID == id {
			return instance
		}
	}
	t.Fatalf("instance %s is absent from %+v", id, instances)
	return linkedInstance{}
}

func TestDeviceLinkingRequiresManualReviewAndReturnsATokenPair(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	started, raw := startLink(t, r,
		linkStartBody("Example client", "studio workstation", []string{"asset:receive"}))

	if _, present := raw["verificationUrlComplete"]; present {
		t.Error("device response includes verificationUrlComplete; the human code must be entered manually")
	}
	if started.VerificationURL != testBrowserOrigin+"/link" {
		t.Errorf("verification URL = %q", started.VerificationURL)
	}
	if len(started.UserCode) != 9 || started.UserCode[4] != '-' {
		t.Errorf("user code = %q, want eight characters split by a dash", started.UserCode)
	}
	if len(started.DeviceCode) < 40 || started.DeviceCode == started.UserCode {
		t.Errorf("private device code = %q, human code = %q", started.DeviceCode, started.UserCode)
	}

	review, pending := reviewDeviceLink(t, r, session, started.UserCode)
	if review.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200. body: %s", review.Code, review.Body.String())
	}
	if pending.ApprovalToken == "" {
		t.Fatal("review response has no one-use approval token")
	}
	if pending.ApplicationName != "Example client" || pending.InstanceName != "studio workstation" {
		t.Errorf("pending identity = %q / %q", pending.ApplicationName, pending.InstanceName)
	}
	if !slices.Equal(pending.Scopes, []string{"asset:receive"}) {
		t.Errorf("pending scopes = %v", pending.Scopes)
	}

	withoutCSRF := httptest.NewRequest(
		http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		strings.NewReader(jsonText(t, map[string]string{"approvalToken": pending.ApprovalToken})),
	)
	withoutCSRF.Header.Set("Content-Type", "application/json")
	withoutCSRF.AddCookie(session)
	rejected := send(t, r, withoutCSRF)
	assertNoStore(t, rejected)
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("approval without browser proof = %d, want 403", rejected.Code)
	}

	approved := send(t, r, browserRequest(
		t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		map[string]string{"approvalToken": pending.ApprovalToken}, session,
	))
	assertNoStore(t, approved)
	if approved.Code != http.StatusOK {
		t.Fatalf("approval status = %d, want 200. body: %s", approved.Code, approved.Body.String())
	}

	linked := poll(t, r, started.DeviceCode)
	if linked.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200. body: %s", linked.Code, linked.Body.String())
	}
	grant := decodeResponse[polledLink](t, linked)
	if grant.Status != "linked" || grant.AccessToken == nil || grant.RefreshToken == nil || grant.Instance == nil {
		t.Fatalf("poll after approval = %+v, want a token pair", grant)
	}
	if !strings.HasPrefix(*grant.AccessToken, "ia1.") || !strings.HasPrefix(*grant.RefreshToken, "ir1.") {
		t.Errorf("unexpected token kinds: access %q refresh %q", *grant.AccessToken, *grant.RefreshToken)
	}
	if grant.Instance.ApplicationName != "Example client" || grant.Instance.InstanceName != "studio workstation" {
		t.Errorf("linked instance = %+v", grant.Instance)
	}
}

func TestDeviceReviewsAreReadOnlyAndApprovalProofsStayWithTheirUser(t *testing.T) {
	r, firstSession, pool := newLinkingRouter(t)
	secondSession := addVerifiedLinkingUser(
		t, r, pool, "second.creator@example.com", "second.creator",
	)
	started, _ := startLink(t, r,
		linkStartBody("Example client", "review test", []string{"asset:receive"}))

	firstReview, first := reviewDeviceLink(t, r, firstSession, started.UserCode)
	reloadedReview, reloaded := reviewDeviceLink(t, r, firstSession, started.UserCode)
	if firstReview.Code != http.StatusOK || reloadedReview.Code != http.StatusOK {
		t.Fatalf("repeated review statuses = %d and %d, want 200", firstReview.Code, reloadedReview.Code)
	}
	if first.ApprovalToken == "" || first.ApprovalToken != reloaded.ApprovalToken {
		t.Fatalf("approval proof changed across tabs: %q then %q", first.ApprovalToken, reloaded.ApprovalToken)
	}

	otherReview, other := reviewDeviceLink(t, r, secondSession, started.UserCode)
	if otherReview.Code != http.StatusOK {
		t.Fatalf("second user review status = %d, want 200. body: %s", otherReview.Code, otherReview.Body.String())
	}
	if other.ApprovalToken == "" || other.ApprovalToken == first.ApprovalToken {
		t.Fatalf("approval proofs are not user-bound: first %q, second %q", first.ApprovalToken, other.ApprovalToken)
	}

	var reviewerMissing, proofMissing bool
	if err := pool.QueryRow(context.Background(), `
		select reviewed_by is null, review_token_hash is null
		  from link_requests
		 where application_name = 'Example client' and instance_name = 'review test'
	`).Scan(&reviewerMissing, &proofMissing); err != nil {
		t.Fatalf("read pending device review state: %v", err)
	}
	if !reviewerMissing || !proofMissing {
		t.Fatal("review GET claimed the device request or stored an approval proof")
	}

	crossUser := send(t, r, browserRequest(
		t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		map[string]string{"approvalToken": first.ApprovalToken}, secondSession,
	))
	if crossUser.Code != http.StatusNotFound {
		t.Fatalf("cross-user approval status = %d, want 404. body: %s", crossUser.Code, crossUser.Body.String())
	}

	approved := send(t, r, browserRequest(
		t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		map[string]string{"approvalToken": first.ApprovalToken}, firstSession,
	))
	if approved.Code != http.StatusOK {
		t.Fatalf("approval after repeated reviews = %d, want 200. body: %s", approved.Code, approved.Body.String())
	}
}

func TestDeviceDenialAndFastPollingReturnProtocolErrors(t *testing.T) {
	t.Run("denial", func(t *testing.T) {
		r, session, _ := newLinkingRouter(t)
		started, _ := startLink(t, r,
			linkStartBody("Example client", "remote host", []string{"asset:receive"}))
		_, pending := reviewDeviceLink(t, r, session, started.UserCode)

		denied := send(t, r, browserRequest(
			t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/deny",
			map[string]string{"approvalToken": pending.ApprovalToken}, session,
		))
		assertNoStore(t, denied)
		if denied.Code != http.StatusNoContent {
			t.Fatalf("deny status = %d, want 204. body: %s", denied.Code, denied.Body.String())
		}

		result := poll(t, r, started.DeviceCode)
		if result.Code != http.StatusBadRequest || !strings.Contains(result.Body.String(), `"access_denied"`) {
			t.Fatalf("poll after denial = %d %s, want access_denied", result.Code, result.Body.String())
		}
	})

	t.Run("slow down", func(t *testing.T) {
		r, _, _ := newLinkingRouter(t)
		started, _ := startLink(t, r,
			linkStartBody("Example client", "remote host", []string{"asset:receive"}))

		waiting := poll(t, r, started.DeviceCode)
		if waiting.Code != http.StatusOK || decodeResponse[polledLink](t, waiting).Status != "pending" {
			t.Fatalf("first poll = %d %s, want pending", waiting.Code, waiting.Body.String())
		}
		tooFast := poll(t, r, started.DeviceCode)
		if tooFast.Code != http.StatusTooManyRequests || !strings.Contains(tooFast.Body.String(), `"slow_down"`) {
			t.Fatalf("fast poll = %d %s, want slow_down", tooFast.Code, tooFast.Body.String())
		}
		retryAfter, err := strconv.Atoi(tooFast.Header().Get("Retry-After"))
		if err != nil || retryAfter <= started.Interval {
			t.Errorf("Retry-After = %q, want an interval increased past %d", tooFast.Header().Get("Retry-After"), started.Interval)
		}
	})
}

func TestRefreshingRotatesTokensAndReuseRevokesTheInstance(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	initial := linkDeviceInstance(
		t, r, session, "Example client", "refresh test", []string{"asset:receive"},
	)

	rotatedRec := sendJSON(t, r, http.MethodPost, "/v1/link/refresh",
		jsonText(t, map[string]string{"refreshToken": initial.RefreshToken}))
	assertNoStore(t, rotatedRec)
	if rotatedRec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200. body: %s", rotatedRec.Code, rotatedRec.Body.String())
	}
	rotated := decodeResponse[tokenGrant](t, rotatedRec)
	if rotated.RefreshToken == initial.RefreshToken || rotated.AccessToken == initial.AccessToken {
		t.Error("refresh returned one of the old tokens")
	}
	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", rotated.AccessToken, nil)); rec.Code != http.StatusOK {
		t.Fatalf("rotated access token status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}

	reused := sendJSON(t, r, http.MethodPost, "/v1/link/refresh",
		jsonText(t, map[string]string{"refreshToken": initial.RefreshToken}))
	assertNoStore(t, reused)
	if reused.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh token status = %d, want 401. body: %s", reused.Code, reused.Body.String())
	}
	for _, access := range []string{initial.AccessToken, rotated.AccessToken} {
		rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", access, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("access token still works after refresh reuse: status %d", rec.Code)
		}
	}
	revoked := instanceByID(t, listInstances(t, r, session), initial.Instance.ID)
	if revoked.RevokedAt == nil {
		t.Error("refresh-token reuse did not leave a revoked instance record")
	}
}

func TestAnIdleRefreshFamilyExpiresAndRevokesItsInstance(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	grant := linkDeviceInstance(
		t, r, session, "Example client", "idle refresh test", []string{"asset:receive"},
	)
	if _, err := pool.Exec(context.Background(),
		`update linked_instances
		    set linked_at = now() - interval '91 days', last_seen_at = null
		  where id = $1`, grant.Instance.ID); err != nil {
		t.Fatalf("age linked instance: %v", err)
	}

	refresh := sendJSON(t, r, http.MethodPost, "/v1/link/refresh",
		jsonText(t, map[string]string{"refreshToken": grant.RefreshToken}))
	assertNoStore(t, refresh)
	if refresh.Code != http.StatusUnauthorized {
		t.Fatalf("idle refresh status = %d, want 401. body: %s", refresh.Code, refresh.Body.String())
	}
	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", grant.AccessToken, nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("access token survived idle-family revocation: status %d", rec.Code)
	}
	if revoked := instanceByID(t, listInstances(t, r, session), grant.Instance.ID); revoked.RevokedAt == nil {
		t.Error("idle refresh family did not leave a revoked instance record")
	}
}

func TestSameApplicationInstancesStayIndependentThroughUpdateAndRevocation(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	first := linkDeviceInstance(
		t, r, session, "Example client", "studio workstation", []string{"asset:receive"},
	)
	second := linkDeviceInstance(
		t, r, session, "Example client", "remote host", []string{"asset:receive", "library:sync"},
	)
	if first.Instance.ID == second.Instance.ID || first.RefreshToken == second.RefreshToken {
		t.Fatal("two installations of one application share identity or credentials")
	}

	updatedCapabilities := []string{
		"example.client:library-report",
		"example.client:asset-install",
	}
	updatedTargets := []string{"portable-lore-v1", "portable-card-v1"}
	update := send(t, r, asInstance(t, http.MethodPut, "/v1/instances/me", first.AccessToken, map[string]any{
		"applicationVersion": "1.1.0",
		"protocolVersion":    1,
		"capabilities":       updatedCapabilities,
		"acceptedTargets":    updatedTargets,
	}))
	assertNoStore(t, update)
	if update.Code != http.StatusOK {
		t.Fatalf("update declaration status = %d, want 200. body: %s", update.Code, update.Body.String())
	}
	updated := decodeResponse[linkedInstance](t, update)
	if !slices.Equal(updated.Capabilities, updatedCapabilities) || !slices.Equal(updated.AcceptedTargets, updatedTargets) {
		t.Errorf("updated declaration lost order: capabilities %v, targets %v", updated.Capabilities, updated.AcceptedTargets)
	}
	if !slices.Equal(updated.Scopes, []string{"asset:receive"}) {
		t.Errorf("declaration update changed granted scopes to %v", updated.Scopes)
	}

	otherRec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", second.AccessToken, nil))
	assertNoStore(t, otherRec)
	if otherRec.Code != http.StatusOK {
		t.Fatalf("second instance status = %d, want 200. body: %s", otherRec.Code, otherRec.Body.String())
	}
	other := decodeResponse[linkedInstance](t, otherRec)
	if !slices.Equal(other.Capabilities, []string{"example.client:asset-install"}) ||
		!slices.Equal(other.AcceptedTargets, []string{"portable-card-v1"}) ||
		!slices.Equal(other.Scopes, []string{"asset:receive", "library:sync"}) {
		t.Errorf("first instance update changed the second: %+v", other)
	}

	revoke := send(t, r, browserRequest(
		t, http.MethodDelete, "/v1/instances/"+first.Instance.ID, nil, session,
	))
	assertNoStore(t, revoke)
	if revoke.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want 204. body: %s", revoke.Code, revoke.Body.String())
	}
	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", first.AccessToken, nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked first instance status = %d, want 401", rec.Code)
	}
	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", second.AccessToken, nil)); rec.Code != http.StatusOK {
		t.Errorf("second instance was affected by first revocation: status %d", rec.Code)
	}

	items := listInstances(t, r, session)
	cut := instanceByID(t, items, first.Instance.ID)
	live := instanceByID(t, items, second.Instance.ID)
	if cut.RevokedAt == nil || cut.ApplicationName != "Example client" || cut.InstanceName != "studio workstation" {
		t.Errorf("revoked audit record = %+v", cut)
	}
	if cut.ApplicationVersion != nil || cut.ProtocolVersion != nil || len(cut.Capabilities) != 0 || len(cut.AcceptedTargets) != 0 {
		t.Errorf("revoked instance kept its live declaration: %+v", cut)
	}
	if live.RevokedAt != nil || live.InstanceName != "remote host" {
		t.Errorf("other instance record = %+v", live)
	}
}

func TestLinkSecretsAreHashedAndHumanCodesUseKeyedDigestsAtRest(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	started, _ := startLink(t, r,
		linkStartBody("Example client", "storage test", []string{"asset:receive"}))
	_, pending := reviewDeviceLink(t, r, session, started.UserCode)
	approved := send(t, r, browserRequest(
		t, http.MethodPost, "/v1/link/requests/"+started.UserCode+"/approve",
		map[string]string{"approvalToken": pending.ApprovalToken}, session,
	))
	if approved.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200. body: %s", approved.Code, approved.Body.String())
	}
	polled := poll(t, r, started.DeviceCode)
	result := decodeResponse[polledLink](t, polled)
	if result.AccessToken == nil || result.RefreshToken == nil || result.Instance == nil {
		t.Fatalf("poll did not return a token pair: %+v", result)
	}

	var userCodeHash []byte
	if err := pool.QueryRow(context.Background(), `select user_code_hash from link_requests`).Scan(&userCodeHash); err != nil {
		t.Fatalf("read human-code digest: %v", err)
	}
	normalizedCode := strings.ReplaceAll(started.UserCode, "-", "")
	mac := hmac.New(sha256.New, testLinkHMACKey)
	mac.Write([]byte("user-code"))
	mac.Write([]byte{0})
	mac.Write([]byte(normalizedCode))
	if !hmac.Equal(userCodeHash, mac.Sum(nil)) {
		t.Errorf("stored human-code digest is not the expected keyed digest")
	}
	plainHash := sha256.Sum256([]byte(normalizedCode))
	if bytes.Equal(userCodeHash, plainHash[:]) || bytes.Contains(userCodeHash, []byte(normalizedCode)) {
		t.Error("human code is recoverable from its stored value")
	}

	var refreshHash []byte
	if err := pool.QueryRow(context.Background(),
		`select refresh_token_hash from linked_instances where id = $1`, result.Instance.ID,
	).Scan(&refreshHash); err != nil {
		t.Fatalf("read refresh-token hash: %v", err)
	}
	wantRefreshHash := sha256.Sum256([]byte(*result.RefreshToken))
	if !bytes.Equal(refreshHash, wantRefreshHash[:]) || bytes.Contains(refreshHash, []byte(*result.RefreshToken)) {
		t.Error("refresh token is not stored solely as its hash")
	}

	var accessHash []byte
	if err := pool.QueryRow(context.Background(),
		`select token_hash from instance_access_tokens where instance_id = $1`, result.Instance.ID,
	).Scan(&accessHash); err != nil {
		t.Fatalf("read access-token hash: %v", err)
	}
	wantAccessHash := sha256.Sum256([]byte(*result.AccessToken))
	if !bytes.Equal(accessHash, wantAccessHash[:]) || bytes.Contains(accessHash, []byte(*result.AccessToken)) {
		t.Error("access token is not stored solely as its hash")
	}
}

func TestLinkBodiesStopAtFourKiBAndResponsesAreNotStored(t *testing.T) {
	r, _, _ := newLinkingRouter(t)
	body := `{"applicationName":"` + strings.Repeat("x", maxLinkBodyBytes) + `"}`
	rec := sendJSON(t, r, http.MethodPost, "/v1/link/requests", body)
	assertNoStore(t, rec)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized link body status = %d, want 413. body: %s", rec.Code, rec.Body.String())
	}

	invalid := sendJSON(t, r, http.MethodPost, "/v1/link/token", `{}`)
	assertNoStore(t, invalid)
	if invalid.Code != http.StatusBadRequest {
		t.Errorf("invalid exchange status = %d, want 400", invalid.Code)
	}
}

func TestFiveCodeReviewsCannotBeResetByAValidCode(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	started, _ := startLink(t, r,
		linkStartBody("Example client", "attempt test", []string{"asset:receive"}))

	for attempt := 1; attempt <= 4; attempt++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/link/requests/BBBB-CCCC", nil)
		rec := send(t, r, authorized(req, session))
		assertNoStore(t, rec)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("wrong-code review %d status = %d, want 404", attempt, rec.Code)
		}
	}
	valid, pending := reviewDeviceLink(t, r, session, started.UserCode)
	if valid.Code != http.StatusOK || pending.ApprovalToken == "" {
		t.Fatalf("fifth, valid review = %d, approval token %q", valid.Code, pending.ApprovalToken)
	}

	for _, code := range []string{"BBBB-CCCC", started.UserCode} {
		req := httptest.NewRequest(http.MethodGet, "/v1/link/requests/"+code, nil)
		rec := send(t, r, authorized(req, session))
		assertNoStore(t, rec)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("review after five total attempts for %q = %d, want 429", code, rec.Code)
		}
	}
}
