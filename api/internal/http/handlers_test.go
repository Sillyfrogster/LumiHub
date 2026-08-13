package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/asset"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/passthrough"
	"github.com/Sillyfrogster/LumiHub/api/internal/storage"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/gin-gonic/gin"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	return newTestRouterWithCeiling(t, 1<<20)
}

func newTestRouterWithCeiling(t *testing.T, maxUploadBytes int64) *gin.Engine {
	t.Helper()
	return newTestRouterWith(t, maxUploadBytes, DefaultDeadlines())
}

func newTestRouterWith(t *testing.T, maxUploadBytes int64, deadlines Deadlines) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool := testdb.Connect(t)
	blob, err := storage.NewStore(pool, t.TempDir())
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	svc := asset.NewService(pool, format.NewRegistry(passthrough.New()), blob)

	r := gin.New()
	if err := Register(r, NewHandlers(svc, maxUploadBytes), deadlines); err != nil {
		t.Fatalf("register: %v", err)
	}
	return r
}

func post(t *testing.T, r *gin.Engine, name string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, r, uploadRequest(t, exampleMetadata(name), []byte(name)))
}

type listedAsset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func get(t *testing.T, r *gin.Engine, url string) []listedAsset {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200. body: %s", url, rec.Code, rec.Body.String())
	}

	var list struct {
		Items []listedAsset `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return list.Items
}

func TestCreateThenListRoundTrip(t *testing.T) {
	r := newTestRouter(t)

	rec := post(t, r, "Mystery")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want 201. body: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created asset: %v", err)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created asset came back with no made date")
	}

	items := get(t, r, "/v1/assets")
	if len(items) != 1 || items[0].Name != "Mystery" {
		t.Fatalf("list returned %+v, want one asset named Mystery", items)
	}
	if !items[0].CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("listed made date %v, created said %v", items[0].CreatedAt, created.CreatedAt)
	}
}

func TestListPagesFromWhereTheLastPageEnded(t *testing.T) {
	r := newTestRouter(t)
	for _, name := range []string{"first", "second", "third"} {
		if rec := post(t, r, name); rec.Code != http.StatusCreated {
			t.Fatalf("POST %s status = %d. body: %s", name, rec.Code, rec.Body.String())
		}
	}

	page := get(t, r, "/v1/assets?limit=2")
	if len(page) != 2 || page[0].Name != "third" || page[1].Name != "second" {
		t.Fatalf("first page = %+v, want third then second", page)
	}

	last := page[len(page)-1]
	next := get(t, r, "/v1/assets?limit=2&before="+
		url.QueryEscape(last.CreatedAt.Format(time.RFC3339Nano))+"&beforeId="+last.ID)
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
