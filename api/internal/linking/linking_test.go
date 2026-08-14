package linking

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonicalScopesAcceptsEachKnownScopeOnce(t *testing.T) {
	both, err := canonicalScopes([]Scope{ScopeSyncLibrary, ScopeReceiveAssets})
	if err != nil {
		t.Fatalf("both scopes: %v", err)
	}
	if len(both) != 2 || both[0] != ScopeReceiveAssets || both[1] != ScopeSyncLibrary {
		t.Errorf("canonical order = %v", both)
	}

	one, err := canonicalScopes([]Scope{ScopeSyncLibrary})
	if err != nil || len(one) != 1 || one[0] != ScopeSyncLibrary {
		t.Errorf("one scope = %v, %v", one, err)
	}

	for _, requested := range [][]Scope{
		{},
		{"asset:write"},
		{ScopeReceiveAssets, ScopeReceiveAssets},
		{ScopeReceiveAssets, "asset:write"},
	} {
		if _, err := canonicalScopes(requested); !errors.Is(err, ErrInvalidScopes) {
			t.Errorf("canonicalScopes(%v) error = %v, want a refusal", requested, err)
		}
	}
}

func TestAUserCodeIsReadTheWayACreatorTypesIt(t *testing.T) {
	code, err := newCode(codeLength)
	if err != nil {
		t.Fatalf("new code: %v", err)
	}
	shown := FormatUserCode(code)

	for _, typed := range []string{shown, strings.ToLower(shown), code, "  " + shown + "  "} {
		normalized, ok := normalizeUserCode(typed)
		if !ok || normalized != code {
			t.Errorf("normalizeUserCode(%q) = %q, %v, want %q", typed, normalized, ok, code)
		}
	}
	for _, typed := range []string{"", "SHORT", code + "B", "AEIO" + code[4:]} {
		if _, ok := normalizeUserCode(typed); ok {
			t.Errorf("normalizeUserCode(%q) accepted a code it cannot be", typed)
		}
	}
}

func TestAMalformedCredentialNeverReachesTheDatabase(t *testing.T) {
	token, prefix, hash, err := newToken()
	if err != nil {
		t.Fatalf("new token: %v", err)
	}
	if !strings.HasPrefix(token, prefix+".") {
		t.Errorf("token %q does not start with its prefix", token)
	}
	got, ok := tokenHash(token)
	if !ok || string(got) != string(hash) {
		t.Error("a fresh credential does not hash to what was stored")
	}

	for _, malformed := range []string{"", "no-dot", "SHORT.abc", prefix + ".not base64", prefix} {
		if _, ok := tokenHash(malformed); ok {
			t.Errorf("tokenHash(%q) accepted a credential it cannot be", malformed)
		}
	}
}

func TestAnInstanceIsRefusedAScopeItWasNotGranted(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	service := NewService(pool, "http://localhost:3000")
	creator := insertCreator(t, pool)

	started, err := service.Start(ctx, StartInput{
		Name:   "Receiver",
		Scopes: []Scope{ScopeReceiveAssets},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := service.Approve(ctx, creator, started.UserCode); err != nil {
		t.Fatalf("approve: %v", err)
	}
	credential, linked, err := service.Poll(ctx, started.DeviceCode)
	if err != nil || !linked {
		t.Fatalf("poll: %v, linked %v", err, linked)
	}

	if _, err := service.Authenticate(ctx, credential.Token, ScopeReceiveAssets); err != nil {
		t.Errorf("granted scope refused: %v", err)
	}
	if _, err := service.Authenticate(ctx, credential.Token, ScopeSyncLibrary); !errors.Is(
		err, ErrInstanceMissingScope,
	) {
		t.Errorf("ungranted scope error = %v, want a refusal", err)
	}
	if _, err := service.Authenticate(ctx, credential.Token, ""); err != nil {
		t.Errorf("an endpoint needing no scope refused a live credential: %v", err)
	}
}

func insertCreator(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into users (id, username, email, email_source, email_verified_at)
		 values ($1, 'linking.creator', 'creator@example.com', 'creator', now())`,
		id); err != nil {
		t.Fatalf("create creator: %v", err)
	}
	return id
}
