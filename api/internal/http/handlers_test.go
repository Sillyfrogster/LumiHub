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

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/linking"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testRegistry(t *testing.T) *format.Registry {
	t.Helper()
	registry := format.NewRegistry()
	if err := registry.Register(opaqueTestModule{}); err != nil {
		t.Fatalf("register test format: %v", err)
	}
	return registry
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newTestRouterWithCeiling(t, 1<<20)
}

func TestFormatRegistryInvariantIsNotAnUploaderRefusal(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	new(Handlers).refuse(ctx, format.ErrConflictingClaims)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "claim") {
		t.Fatalf("response exposed registry details: %s", recorder.Body.String())
	}
}

func newTestRouterWithCeiling(t *testing.T, maxUploadBytes int64) *gin.Engine {
	t.Helper()
	return newTestRouterWith(t, maxUploadBytes, DefaultDeadlines())
}

func newTestRouterWith(t *testing.T, maxUploadBytes int64, deadlines Deadlines) *gin.Engine {
	t.Helper()
	return newTestRouterWithSender(t, maxUploadBytes, deadlines, &verificationOutbox{})
}

func newTestRouterWithSender(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
	sender account.EmailSender,
) *gin.Engine {
	t.Helper()
	return registerTestRouter(t, newTestHandlers(t, maxUploadBytes, sender), deadlines)
}

func newTestRouterWithSenderAndPool(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
	sender account.EmailSender,
) (*gin.Engine, *pgxpool.Pool) {
	t.Helper()
	router, pool, _ := newTestRouterWithSenderPoolAndHandlers(
		t, maxUploadBytes, deadlines, sender,
	)
	return router, pool
}

func newTestRouterWithSenderPoolAndHandlers(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
	sender account.EmailSender,
) (*gin.Engine, *pgxpool.Pool, *Handlers) {
	t.Helper()
	pool := testdb.Connect(t)
	handlers := newTestHandlersWithPool(t, pool, maxUploadBytes, sender)
	return registerTestRouter(t, handlers, deadlines), pool, handlers
}

func newTestHandlers(
	t *testing.T,
	maxUploadBytes int64,
	sender account.EmailSender,
) *Handlers {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool := testdb.Connect(t)
	return newTestHandlersWithPool(t, pool, maxUploadBytes, sender)
}

func newTestHandlersWithPool(
	t *testing.T,
	pool *pgxpool.Pool,
	maxUploadBytes int64,
	sender account.EmailSender,
) *Handlers {
	t.Helper()
	gin.SetMode(gin.TestMode)

	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := asset.NewService(pool, testRegistry(t), blob)
	accounts := account.NewService(pool, sender, nil, "http://localhost:3000")
	links := newTestLinkingService(pool)

	return NewHandlers(svc, accounts, links, maxUploadBytes)
}

func newTestRouterWithDiscord(
	t *testing.T,
	provider account.DiscordProvider,
) *gin.Engine {
	t.Helper()
	r, _ := newTestRouterWithDiscordAndOutbox(t, provider)
	return r
}

func newTestRouterWithDiscordAndOutbox(
	t *testing.T,
	provider account.DiscordProvider,
) (*gin.Engine, *verificationOutbox) {
	t.Helper()
	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	assets := asset.NewService(pool, testRegistry(t), blob)
	outbox := &verificationOutbox{}
	accounts := account.NewService(
		pool, outbox, provider, "http://localhost:3000",
	)
	return registerTestRouter(t, NewHandlers(assets, accounts, newTestLinkingService(pool), 1<<20), DefaultDeadlines()), outbox
}

func newTestLinkingService(pool *pgxpool.Pool) *linking.Service {
	return linking.NewService(pool, "http://localhost:3000", []byte("01234567890123456789012345678901"))
}

func registerTestRouter(t *testing.T, handlers *Handlers, deadlines Deadlines) *gin.Engine {
	t.Helper()
	r := gin.New()
	if err := Register(r, handlers, deadlines); err != nil {
		t.Fatalf("register: %v", err)
	}
	return r
}

func newVerifiedTestRouter(t *testing.T) (*gin.Engine, *http.Cookie) {
	t.Helper()
	return newVerifiedTestRouterWith(t, 1<<20, DefaultDeadlines())
}

func newVerifiedTestRouterWith(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
) (*gin.Engine, *http.Cookie) {
	t.Helper()
	router, session, _ := newVerifiedTestRouterWithService(t, maxUploadBytes, deadlines)
	return router, session
}

func newVerifiedTestRouterWithService(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
) (*gin.Engine, *http.Cookie, *asset.Service) {
	t.Helper()
	_, router, session, assets := newVerifiedTestRoutersWithService(t, maxUploadBytes, deadlines)
	return router, session, assets
}

func newVerifiedTestRoutersWithService(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
) (*gin.Engine, *gin.Engine, *http.Cookie, *asset.Service) {
	setupRouter, router, session, assets, _ := newVerifiedTestRoutersWithPool(
		t, maxUploadBytes, deadlines,
	)
	return setupRouter, router, session, assets
}

func newVerifiedTestRoutersWithPool(
	t *testing.T,
	maxUploadBytes int64,
	deadlines Deadlines,
) (*gin.Engine, *gin.Engine, *http.Cookie, *asset.Service, *pgxpool.Pool) {
	t.Helper()
	outbox := &verificationOutbox{}
	setupRouter, pool, handlers := newTestRouterWithSenderPoolAndHandlers(
		t, maxUploadBytes, DefaultDeadlines(), outbox,
	)
	session := signUp(t, setupRouter, "verified@example.com", "verified.creator")
	verificationURL, err := url.Parse(outbox.messages[0].link)
	if err != nil {
		t.Fatalf("parse verification link: %v", err)
	}
	rec := sendJSON(t, setupRouter, http.MethodPost, "/v1/auth/verify-email",
		`{"token":"`+verificationURL.Query().Get("token")+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify test account: %d %s", rec.Code, rec.Body.String())
	}
	return setupRouter, registerTestRouter(t, handlers, deadlines), session, handlers.assets, pool
}

func authorized(req *http.Request, session *http.Cookie) *http.Request {
	req.AddCookie(session)
	return req
}

type verificationMessage struct {
	address string
	link    string
}

type verificationOutbox struct {
	messages       []verificationMessage
	passwordResets []verificationMessage
}

func (o *verificationOutbox) SendVerification(_ context.Context, address, link string) error {
	o.messages = append(o.messages, verificationMessage{address: address, link: link})
	return nil
}

func (o *verificationOutbox) SendPasswordReset(_ context.Context, address, link string) error {
	o.passwordResets = append(o.passwordResets, verificationMessage{address: address, link: link})
	return nil
}

func post(
	t *testing.T,
	r *gin.Engine,
	session *http.Cookie,
	assets *asset.Service,
	name string,
) *httptest.ResponseRecorder {
	t.Helper()
	metadata := exampleMetadata(name)
	metadata["filename"] = name + ".lumitheme"
	return uploadAndFinish(t, r, session, assets, metadata, []byte(name))
}

type listedAsset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type listedPage struct {
	Items      []listedAsset `json:"items"`
	NextCursor *struct {
		Before   time.Time `json:"before"`
		BeforeID string    `json:"beforeId"`
	} `json:"nextCursor"`
}

func getPage(t *testing.T, r *gin.Engine, url string) listedPage {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200. body: %s", url, rec.Code, rec.Body.String())
	}

	var list listedPage
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return list
}

func get(t *testing.T, r *gin.Engine, url string) []listedAsset {
	t.Helper()
	return getPage(t, r, url).Items
}

func TestCreateThenListRoundTrip(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())

	rec := post(t, r, session, assets, "Mystery")
	if rec.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200. body: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Asset *struct {
			CreatedAt time.Time `json:"createdAt"`
		} `json:"asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created asset: %v", err)
	}
	if created.Asset == nil || created.Asset.CreatedAt.IsZero() {
		t.Error("created asset came back with no made date")
	}

	items := get(t, r, "/v1/assets")
	if len(items) != 1 || items[0].Name != "Mystery" {
		t.Fatalf("list returned %+v, want one asset named Mystery", items)
	}
}

func TestListPagesFromWhereTheLastPageEnded(t *testing.T) {
	r, session, assets := newVerifiedIngestRouter(t, format.NewRegistry())
	for _, name := range []string{"first", "second", "third"} {
		if rec := post(t, r, session, assets, name); rec.Code != http.StatusOK {
			t.Fatalf("POST %s status = %d. body: %s", name, rec.Code, rec.Body.String())
		}
	}

	page := getPage(t, r, "/v1/assets?limit=2")
	if len(page.Items) != 2 || page.Items[0].Name != "third" || page.Items[1].Name != "second" {
		t.Fatalf("first page = %+v, want third then second", page)
	}
	if page.NextCursor == nil {
		t.Fatal("first page has no next cursor")
	}

	next := get(t, r, "/v1/assets?limit=2&before="+
		url.QueryEscape(page.NextCursor.Before.Format(time.RFC3339Nano))+"&beforeId="+page.NextCursor.BeforeID)
	if len(next) != 1 || next[0].Name != "first" {
		t.Fatalf("second page = %+v, want only first", next)
	}
}

func TestListRefusesHalfACursor(t *testing.T) {
	r := newTestRouter(t)

	for _, query := range []string{
		"/v1/assets?before=2024-05-01T12:00:00Z",
		"/v1/assets?beforeId=6f1e1a2c-0000-4000-8000-000000000000",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, query, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want 400: half a cursor has no fixed point", query, rec.Code)
		}
	}
}
