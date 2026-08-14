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

	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type startedLink struct {
	DeviceCode              string    `json:"deviceCode"`
	UserCode                string    `json:"userCode"`
	VerificationURL         string    `json:"verificationUrl"`
	VerificationURLComplete string    `json:"verificationUrlComplete"`
	ExpiresAt               time.Time `json:"expiresAt"`
	Interval                int       `json:"interval"`
}

type polledLink struct {
	Status   string          `json:"status"`
	Token    *string         `json:"token"`
	Instance *linkedInstance `json:"instance"`
}

type linkedInstance struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LinkedAt   time.Time  `json:"linkedAt"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	RevokedAt  *time.Time `json:"revokedAt"`
}

type pendingLink struct {
	Name      string    `json:"name"`
	Scopes    []string  `json:"scopes"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func newLinkingRouter(t *testing.T) (*gin.Engine, *http.Cookie, *pgxpool.Pool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pool := testdb.Connect(t)
	outbox := &verificationOutbox{}
	handlers := newTestHandlersWithPool(t, pool, 1<<20, outbox)
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

func startLink(t *testing.T, r *gin.Engine, body string) startedLink {
	t.Helper()
	rec := sendJSON(t, r, http.MethodPost, "/v1/link/requests", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start link status = %d, want 201. body: %s", rec.Code, rec.Body.String())
	}
	var started startedLink
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode link request: %v", err)
	}
	return started
}

func poll(t *testing.T, r *gin.Engine, deviceCode string) *httptest.ResponseRecorder {
	t.Helper()
	return sendJSON(t, r, http.MethodPost, "/v1/link/poll",
		`{"deviceCode":"`+deviceCode+`"}`)
}

func approve(t *testing.T, r *gin.Engine, session *http.Cookie, userCode string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/link/requests/"+userCode+"/approve", nil)
	return send(t, r, authorized(req, session))
}

// allowNextPoll ages the last poll so the test does not have to wait out the
// interval.
func allowNextPoll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`update link_requests set last_polled_at = now() - interval '1 minute'`); err != nil {
		t.Fatalf("age poll: %v", err)
	}
}

func linkAnInstance(
	t *testing.T,
	r *gin.Engine,
	session *http.Cookie,
	pool *pgxpool.Pool,
	name string,
	scopes string,
) (linkedInstance, string) {
	t.Helper()
	started := startLink(t, r, `{"name":"`+name+`","scopes":`+scopes+`}`)
	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	allowNextPoll(t, pool)
	rec := poll(t, r, started.DeviceCode)
	if rec.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var result polledLink
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode poll: %v", err)
	}
	if result.Status != "linked" || result.Token == nil || result.Instance == nil {
		t.Fatalf("poll after approval = %+v, want a linked instance and a token", result)
	}
	return *result.Instance, *result.Token
}

func listInstances(t *testing.T, r *gin.Engine, session *http.Cookie) []linkedInstance {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/instances", nil)
	rec := send(t, r, authorized(req, session))
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

func asInstance(t *testing.T, method, target, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func TestLinkingHandsTheClientOneCodeToShowAndOneToKeep(t *testing.T) {
	r, _, _ := newLinkingRouter(t)

	started := startLink(t, r,
		`{"name":"Lumiverse on the study desktop","scopes":["asset:receive"]}`)

	if len(started.UserCode) != 9 || started.UserCode[4] != '-' {
		t.Errorf("user code = %q, want eight characters split by a dash", started.UserCode)
	}
	if len(started.DeviceCode) < 40 {
		t.Errorf("device code = %q, want a long private code", started.DeviceCode)
	}
	if started.DeviceCode == started.UserCode {
		t.Error("the code the creator reads is also the code the client keeps")
	}
	if started.VerificationURL != "http://localhost:3000/link" {
		t.Errorf("verification URL = %q", started.VerificationURL)
	}
	if want := "http://localhost:3000/link?code=" + started.UserCode; started.VerificationURLComplete != want {
		t.Errorf("prefilled URL = %q, want %q", started.VerificationURLComplete, want)
	}
	if started.Interval < 1 {
		t.Errorf("interval = %d, want the seconds a client waits between polls", started.Interval)
	}
	if until := time.Until(started.ExpiresAt); until <= 0 || until > time.Hour {
		t.Errorf("expires in %s, want a short window", until)
	}
}

func TestAClientPollsUntilTheCreatorApproves(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive","library:sync"]}`)

	waiting := poll(t, r, started.DeviceCode)
	if waiting.Code != http.StatusOK {
		t.Fatalf("first poll status = %d, want 200. body: %s", waiting.Code, waiting.Body.String())
	}
	var pending polledLink
	if err := json.Unmarshal(waiting.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode pending poll: %v", err)
	}
	if pending.Status != "pending" || pending.Token != nil {
		t.Fatalf("poll before approval = %+v, want pending and no token", pending)
	}

	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}

	allowNextPoll(t, pool)
	rec := poll(t, r, started.DeviceCode)
	var linked polledLink
	if err := json.Unmarshal(rec.Body.Bytes(), &linked); err != nil {
		t.Fatalf("decode linked poll: %v", err)
	}
	if rec.Code != http.StatusOK || linked.Status != "linked" || linked.Token == nil {
		t.Fatalf("poll after approval = %d %+v", rec.Code, linked)
	}
	if linked.Instance == nil || linked.Instance.Name != "Lumiverse" {
		t.Fatalf("linked instance = %+v", linked.Instance)
	}
	if len(linked.Instance.Scopes) != 2 {
		t.Errorf("scopes = %v, want both granted", linked.Instance.Scopes)
	}
	if !strings.HasPrefix(*linked.Token, linked.Instance.Prefix) {
		t.Errorf("prefix %q does not start the credential", linked.Instance.Prefix)
	}
}

func TestTheApprovalScreenShowsTheNameAndScopesTheClientAskedFor(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse on the VPS","scopes":["asset:receive"]}`)

	req := httptest.NewRequest(http.MethodGet, "/v1/link/requests/"+started.UserCode, nil)
	rec := send(t, r, authorized(req, session))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var shown pendingLink
	if err := json.Unmarshal(rec.Body.Bytes(), &shown); err != nil {
		t.Fatalf("decode pending link: %v", err)
	}
	if shown.Name != "Lumiverse on the VPS" {
		t.Errorf("name = %q", shown.Name)
	}
	if len(shown.Scopes) != 1 || shown.Scopes[0] != "asset:receive" {
		t.Errorf("scopes = %v, want only the one asked for", shown.Scopes)
	}
}

func TestAClientMayAskForOneScope(t *testing.T) {
	r, session, pool := newLinkingRouter(t)

	instance, _ := linkAnInstance(t, r, session, pool, "Receiver", `["library:sync"]`)
	if len(instance.Scopes) != 1 || instance.Scopes[0] != "library:sync" {
		t.Errorf("scopes = %v, want only library:sync", instance.Scopes)
	}
}

func TestLinkingRefusesAnUnknownScopeOrAnEmptyName(t *testing.T) {
	r, _, _ := newLinkingRouter(t)

	for _, body := range []string{
		`{"name":"Lumiverse","scopes":["asset:write"]}`,
		`{"name":"Lumiverse","scopes":[]}`,
		`{"name":"Lumiverse","scopes":["asset:receive","asset:receive"]}`,
		`{"name":"   ","scopes":["asset:receive"]}`,
		`{"name":"` + strings.Repeat("x", 65) + `","scopes":["asset:receive"]}`,
	} {
		rec := sendJSON(t, r, http.MethodPost, "/v1/link/requests", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("start with %s status = %d, want 400", body, rec.Code)
		}
	}
}

func TestALinkRequestIsApprovedOnceAndRedeemedOnce(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive"]}`)

	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusOK {
		t.Fatalf("first approve status = %d. body: %s", rec.Code, rec.Body.String())
	}
	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusNotFound {
		t.Errorf("second approve status = %d, want 404", rec.Code)
	}

	allowNextPoll(t, pool)
	if rec := poll(t, r, started.DeviceCode); rec.Code != http.StatusOK {
		t.Fatalf("first poll status = %d. body: %s", rec.Code, rec.Body.String())
	}
	allowNextPoll(t, pool)
	if rec := poll(t, r, started.DeviceCode); rec.Code != http.StatusNotFound {
		t.Errorf("second poll status = %d, want 404", rec.Code)
	}
}

func TestAnExpiredLinkRequestCannotBeApprovedOrRedeemed(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive"]}`)

	if _, err := pool.Exec(context.Background(),
		`update link_requests set expires_at = now() - interval '1 second'`); err != nil {
		t.Fatalf("expire the request: %v", err)
	}

	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusNotFound {
		t.Errorf("approve an expired request = %d, want 404", rec.Code)
	}
	if rec := poll(t, r, started.DeviceCode); rec.Code != http.StatusNotFound {
		t.Errorf("poll an expired request = %d, want 404", rec.Code)
	}
}

func TestPollingFasterThanTheIntervalIsRefused(t *testing.T) {
	r, _, pool := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive"]}`)

	if rec := poll(t, r, started.DeviceCode); rec.Code != http.StatusOK {
		t.Fatalf("first poll status = %d. body: %s", rec.Code, rec.Body.String())
	}
	rec := poll(t, r, started.DeviceCode)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("immediate second poll status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("a refused poll does not say when to try again")
	}

	allowNextPoll(t, pool)
	if rec := poll(t, r, started.DeviceCode); rec.Code != http.StatusOK {
		t.Errorf("poll after the interval status = %d, want 200", rec.Code)
	}
}

func TestEnteringWrongCodesRepeatedlyIsRefused(t *testing.T) {
	r, session, _ := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive"]}`)

	for attempt := range 10 {
		rec := approve(t, r, session, "BBBB-CCCC")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("attempt %d status = %d, want 404", attempt, rec.Code)
		}
	}
	if rec := approve(t, r, session, "BBBB-CCCC"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("attempt past the limit status = %d, want 429", rec.Code)
	}
	if rec := approve(t, r, session, started.UserCode); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a real code past the limit status = %d, want 429", rec.Code)
	}
}

func TestApprovingNeedsAVerifiedAccount(t *testing.T) {
	r, _, _ := newLinkingRouter(t)
	started := startLink(t, r, `{"name":"Lumiverse","scopes":["asset:receive"]}`)

	unsigned := httptest.NewRequest(http.MethodPost,
		"/v1/link/requests/"+started.UserCode+"/approve", nil)
	if rec := send(t, r, unsigned); rec.Code != http.StatusUnauthorized {
		t.Errorf("approve with no session status = %d, want 401", rec.Code)
	}

	unverified := signUp(t, r, "unverified@example.com", "unverified.reader")
	if rec := approve(t, r, unverified, started.UserCode); rec.Code != http.StatusForbidden {
		t.Errorf("approve unverified status = %d, want 403", rec.Code)
	}
}

func TestTheCredentialIsStoredHashedWithAPrefixKept(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	instance, token := linkAnInstance(t, r, session, pool, "Lumiverse", `["asset:receive"]`)

	var stored []byte
	var prefix string
	if err := pool.QueryRow(context.Background(),
		`select token_hash, token_prefix from linked_instances where id = $1`,
		instance.ID).Scan(&stored, &prefix); err != nil {
		t.Fatalf("read instance: %v", err)
	}
	if strings.Contains(string(stored), token) {
		t.Error("the credential is stored as it was handed out")
	}
	if len(stored) != 32 {
		t.Errorf("stored credential is %d bytes, want a 32 byte hash", len(stored))
	}
	if prefix != instance.Prefix || !strings.HasPrefix(token, prefix) {
		t.Errorf("prefix %q does not identify the credential", prefix)
	}
}

func TestAnAuthenticatedRequestUpdatesWhenTheInstanceWasLastSeen(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	instance, token := linkAnInstance(t, r, session, pool, "Lumiverse", `["asset:receive"]`)
	if instance.LastSeenAt != nil {
		t.Errorf("a fresh instance has been seen at %v", instance.LastSeenAt)
	}

	rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", token))
	if rec.Code != http.StatusOK {
		t.Fatalf("instance status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}
	var seen linkedInstance
	if err := json.Unmarshal(rec.Body.Bytes(), &seen); err != nil {
		t.Fatalf("decode instance: %v", err)
	}
	if seen.ID != instance.ID || seen.LastSeenAt == nil {
		t.Fatalf("instance = %+v, want the same instance with a last seen time", seen)
	}

	first := *seen.LastSeenAt
	time.Sleep(10 * time.Millisecond)
	again := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", token))
	var later linkedInstance
	if err := json.Unmarshal(again.Body.Bytes(), &later); err != nil {
		t.Fatalf("decode second read: %v", err)
	}
	if later.LastSeenAt == nil || !later.LastSeenAt.After(first) {
		t.Errorf("last seen %v did not move past %v", later.LastSeenAt, first)
	}
}

func TestAnUnknownCredentialIsRefused(t *testing.T) {
	r, _, _ := newLinkingRouter(t)

	for _, token := range []string{"", "not-a-credential", "BCDFGHJK.zzzz"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/instances/me", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if rec := send(t, r, req); rec.Code != http.StatusUnauthorized {
			t.Errorf("credential %q status = %d, want 401", token, rec.Code)
		}
	}
}

func TestTheSettingsListingTellsTwoInstancesApart(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	first, _ := linkAnInstance(t, r, session, pool, "Study desktop", `["asset:receive"]`)
	second, secondToken := linkAnInstance(t, r, session, pool,
		"Headless box", `["asset:receive","library:sync"]`)
	send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", secondToken))

	items := listInstances(t, r, session)
	if len(items) != 2 {
		t.Fatalf("listed %d instances, want 2", len(items))
	}
	if items[0].ID != second.ID {
		t.Errorf("listing leads with %q, want the most recently seen", items[0].Name)
	}
	if items[0].Name != "Headless box" || len(items[0].Scopes) != 2 ||
		items[0].LastSeenAt == nil || items[0].LinkedAt.IsZero() {
		t.Errorf("first entry = %+v", items[0])
	}
	if items[1].ID != first.ID || items[1].Prefix == items[0].Prefix {
		t.Errorf("second entry = %+v, want the other instance with its own prefix", items[1])
	}
}

func TestRevokingCutsAccessAndKeepsARecord(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	instance, token := linkAnInstance(t, r, session, pool, "Lumiverse", `["asset:receive"]`)
	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", token)); rec.Code != http.StatusOK {
		t.Fatalf("instance status before revoking = %d", rec.Code)
	}

	revoke := httptest.NewRequest(http.MethodDelete, "/v1/instances/"+instance.ID, nil)
	if rec := send(t, r, authorized(revoke, session)); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want 204. body: %s", rec.Code, rec.Body.String())
	}

	if rec := send(t, r, asInstance(t, http.MethodGet, "/v1/instances/me", token)); rec.Code != http.StatusUnauthorized {
		t.Errorf("credential still works after revoking: status %d", rec.Code)
	}

	items := listInstances(t, r, session)
	if len(items) != 1 || items[0].RevokedAt == nil || items[0].Name != "Lumiverse" {
		t.Fatalf("listing after revoking = %+v, want the revoked record kept", items)
	}

	var hash []byte
	if err := pool.QueryRow(context.Background(),
		`select token_hash from linked_instances where id = $1`, instance.ID).Scan(&hash); err != nil {
		t.Fatalf("read revoked instance: %v", err)
	}
	if hash != nil {
		t.Error("a revoked instance kept its credential")
	}

	again := httptest.NewRequest(http.MethodDelete, "/v1/instances/"+instance.ID, nil)
	if rec := send(t, r, authorized(again, session)); rec.Code != http.StatusNotFound {
		t.Errorf("revoking twice status = %d, want 404", rec.Code)
	}
}

func TestOneCreatorCannotSeeOrRevokeAnotherCreatorsInstance(t *testing.T) {
	r, session, pool := newLinkingRouter(t)
	instance, _ := linkAnInstance(t, r, session, pool, "Lumiverse", `["asset:receive"]`)
	stranger := signUp(t, r, "stranger@example.com", "stranger.reader")

	if items := listInstances(t, r, stranger); len(items) != 0 {
		t.Errorf("a stranger sees %d instances, want none", len(items))
	}
	revoke := httptest.NewRequest(http.MethodDelete, "/v1/instances/"+instance.ID, nil)
	if rec := send(t, r, authorized(revoke, stranger)); rec.Code != http.StatusNotFound {
		t.Errorf("a stranger revoking status = %d, want 404", rec.Code)
	}
}

func TestTheProtocolGuideAndTheOpenAPIFileAreServed(t *testing.T) {
	r, _, _ := newLinkingRouter(t)

	guide := send(t, r, httptest.NewRequest(http.MethodGet, "/protocol", nil))
	if guide.Code != http.StatusOK {
		t.Fatalf("protocol guide status = %d, want 200", guide.Code)
	}
	if !strings.Contains(guide.Body.String(), "/v1/link/requests") {
		t.Error("the protocol guide does not describe the link endpoints")
	}

	contract := send(t, r, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))
	if contract.Code != http.StatusOK {
		t.Fatalf("openapi status = %d, want 200", contract.Code)
	}
	if !strings.Contains(contract.Body.String(), "openapi: 3.1.0") {
		t.Error("the served contract is not the OpenAPI file")
	}
}
