package linking

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

func TestDeclarationIdentifiersAreNamespacedBoundedAndOrdered(t *testing.T) {
	declaration, err := validateDeclaration(testDeclaration())
	if err != nil {
		t.Fatalf("valid declaration: %v", err)
	}
	if strings.Join(declaration.Capabilities, ",") != "paper-lantern:install,paper-lantern:sync" {
		t.Errorf("capability order = %v", declaration.Capabilities)
	}
	if strings.Join(declaration.AcceptedTargets, ",") != "character-card-v3,lorebook-v2" {
		t.Errorf("target order = %v", declaration.AcceptedTargets)
	}

	invalid := []Declaration{
		{ApplicationName: "App", InstanceName: "Desk", ProtocolVersion: 1},
		withCapabilities(testDeclaration(), []string{"not-namespaced"}),
		withCapabilities(testDeclaration(), []string{"paper-lantern:sync", "paper-lantern:sync"}),
		withTargets(testDeclaration(), []string{"UPPERCASE"}),
		withProtocol(testDeclaration(), 2),
	}
	for _, candidate := range invalid {
		if _, err := validateDeclaration(candidate); !errors.Is(err, ErrInvalidDeclaration) {
			t.Errorf("validateDeclaration(%+v) error = %v, want a refusal", candidate, err)
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

func TestCredentialsHaveSeparateKindsAndRejectMalformedValues(t *testing.T) {
	for _, kind := range []string{accessTokenKind, refreshTokenKind} {
		token, prefix, hash, err := newCredential(kind)
		if err != nil {
			t.Fatalf("new %s credential: %v", kind, err)
		}
		if !strings.HasPrefix(token, kind+"."+prefix+".") {
			t.Errorf("token %q does not carry kind and prefix", token)
		}
		got, ok := credentialHash(token, kind)
		if !ok || string(got) != string(hash) {
			t.Error("a fresh credential does not hash to what was stored")
		}
		other := accessTokenKind
		if kind == accessTokenKind {
			other = refreshTokenKind
		}
		if _, ok := credentialHash(token, other); ok {
			t.Errorf("%s credential was accepted as %s", kind, other)
		}
	}

	for _, malformed := range []string{"", "no-dot", "ia1.SHORT.abc", "ia1.BAD-CODE.not-base64"} {
		if _, ok := credentialHash(malformed, accessTokenKind); ok {
			t.Errorf("credentialHash(%q) accepted a malformed value", malformed)
		}
	}
}

func TestAuthorizationAcceptsOnlyExactLoopbackCallbacksAndS256(t *testing.T) {
	verifier := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	base := AuthorizationInput{
		StartInput: StartInput{
			Declaration: testDeclaration(),
			Scopes:      []Scope{ScopeReceiveAssets},
		},
		RedirectURI:         "http://127.0.0.1:49152/link/callback",
		State:               strings.Repeat("s", 43),
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	}
	if _, err := validateAuthorization(base); err != nil {
		t.Fatalf("valid authorization: %v", err)
	}
	if !challengeMatches(verifier, challenge) || challengeMatches(strings.Repeat("x", 64), challenge) {
		t.Error("S256 verifier comparison did not bind the original verifier")
	}

	for _, callback := range []string{
		"http://[::1]:49152/link/callback",
		"http://127.0.0.1:1/link/callback",
	} {
		candidate := base
		candidate.RedirectURI = callback
		if _, err := validateAuthorization(candidate); err != nil {
			t.Errorf("loopback callback %q: %v", callback, err)
		}
	}
	for _, callback := range []string{
		"http://localhost:49152/link/callback",
		"http://127.0.0.2:49152/link/callback",
		"http://[::ffff:127.0.0.1]:49152/link/callback",
		"http://[0:0:0:0:0:0:0:1]:49152/link/callback",
		"http://192.168.1.4:49152/link/callback",
		"https://127.0.0.1:49152/link/callback",
		"http://127.0.0.1/link/callback",
		"http://127.0.0.1:49152/link/callback?code=old",
		"http://user@127.0.0.1:49152/link/callback",
		"http://127.0.0.1:49152/" + strings.Repeat("x", maxRedirectLength),
	} {
		candidate := base
		candidate.RedirectURI = callback
		if _, err := validateAuthorization(candidate); !errors.Is(err, ErrInvalidRedirect) {
			t.Errorf("callback %q error = %v, want refusal", callback, err)
		}
	}
}

func TestAnInstanceIsRefusedAScopeItWasNotGranted(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	service := NewService(
		pool,
		"http://localhost:3000",
		[]byte("01234567890123456789012345678901"),
	)
	creator := insertCreator(t, pool)

	started, err := service.Start(ctx, "127.0.0.1", StartInput{
		Declaration: testDeclaration(),
		Scopes:      []Scope{ScopeReceiveAssets},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	pending, err := service.Pending(ctx, creator, started.UserCode)
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if _, err := service.Approve(ctx, creator, started.UserCode, pending.ApprovalToken); err != nil {
		t.Fatalf("approve: %v", err)
	}
	grant, linked, err := service.Poll(ctx, "127.0.0.1", started.DeviceCode)
	if err != nil || !linked {
		t.Fatalf("poll: %v, linked %v", err, linked)
	}

	if _, err := service.Authenticate(ctx, grant.AccessToken, ScopeReceiveAssets); err != nil {
		t.Errorf("granted scope refused: %v", err)
	}
	if _, err := service.Authenticate(ctx, grant.AccessToken, ScopeSyncLibrary); !errors.Is(
		err, ErrInstanceMissingScope,
	) {
		t.Errorf("ungranted scope error = %v, want a refusal", err)
	}
	if _, err := service.Authenticate(ctx, grant.AccessToken, ""); err != nil {
		t.Errorf("an endpoint needing no scope refused a live credential: %v", err)
	}
}

func TestRevokingALegacyInstanceErasesItsExchangeHash(t *testing.T) {
	ctx := context.Background()
	pool := testdb.Connect(t)
	service := NewService(
		pool,
		"http://localhost:3000",
		[]byte("01234567890123456789012345678901"),
	)
	creator := insertCreator(t, pool)
	instanceID := uuid.New()
	legacyHash := sha256.Sum256([]byte("old-permanent-token"))
	if _, err := pool.Exec(ctx,
		`insert into linked_instances (
			id, user_id, application_name, instance_name,
			legacy_token_hash, refresh_token_prefix, scopes
		) values ($1, $2, 'Old application', 'old workstation', $3, 'BCDF2345', $4)`,
		instanceID, creator, legacyHash[:], []string{"asset:receive"}); err != nil {
		t.Fatalf("create legacy instance: %v", err)
	}

	if err := service.Revoke(ctx, creator, instanceID); err != nil {
		t.Fatalf("revoke legacy instance: %v", err)
	}
	var storedHash []byte
	var revoked bool
	if err := pool.QueryRow(ctx,
		`select legacy_token_hash, revoked_at is not null
		   from linked_instances where id = $1`, instanceID,
	).Scan(&storedHash, &revoked); err != nil {
		t.Fatalf("read revoked legacy instance: %v", err)
	}
	if storedHash != nil || !revoked {
		t.Errorf("revoked legacy hash = %x, revoked = %v", storedHash, revoked)
	}
}

func testDeclaration() Declaration {
	return Declaration{
		ApplicationName: "Paper Lantern", InstanceName: "studio workstation",
		ApplicationVersion: "2.4.0", ProtocolVersion: 1,
		Capabilities:    []string{"paper-lantern:install", "paper-lantern:sync"},
		AcceptedTargets: []string{"character-card-v3", "lorebook-v2"},
	}
}

func withCapabilities(declaration Declaration, capabilities []string) Declaration {
	declaration.Capabilities = capabilities
	return declaration
}

func withTargets(declaration Declaration, targets []string) Declaration {
	declaration.AcceptedTargets = targets
	return declaration
}

func withProtocol(declaration Declaration, version int) Declaration {
	declaration.ProtocolVersion = version
	return declaration
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
