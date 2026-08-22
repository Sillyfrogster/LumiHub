// Package linking joins an application installation to an Illarin account.
package linking

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Scope is one permission granted to a linked instance.
type Scope string

const (
	ScopeReceiveAssets Scope = "asset:receive"
	ScopeSyncLibrary   Scope = "library:sync"
)

var scopeOrder = []Scope{ScopeReceiveAssets, ScopeSyncLibrary}

var (
	ErrInvalidName          = errors.New("application and instance names are required")
	ErrInvalidDeclaration   = errors.New("the instance declaration is not valid")
	ErrInvalidScopes        = errors.New("at least one known scope is required")
	ErrInvalidRedirect      = errors.New("the redirect is not an exact loopback callback")
	ErrInvalidPKCE          = errors.New("S256 PKCE data is not valid")
	ErrLinkRequestNotFound  = errors.New("no pending link request has that code")
	ErrLinkExpired          = errors.New("the link request expired")
	ErrAccessDenied         = errors.New("the creator denied the link request")
	ErrPollTooSoon          = errors.New("the client must slow down")
	ErrTooManyCodes         = errors.New("too many link codes were entered")
	ErrTooManyRequests      = errors.New("too many link requests were made")
	ErrInstanceNotFound     = errors.New("no live instance has that id")
	ErrInstanceCredential   = errors.New("the token does not identify a live instance")
	ErrRefreshReuse         = errors.New("a replaced refresh token was reused")
	ErrInstanceMissingScope = errors.New("the instance was not granted that scope")
)

// Declaration is self-asserted interoperability metadata, never authority.
type Declaration struct {
	ApplicationName    string
	InstanceName       string
	ApplicationVersion string
	ProtocolVersion    int
	Capabilities       []string
	AcceptedTargets    []string
}

// StartInput is the common input for either authorization path.
type StartInput struct {
	Declaration
	Scopes []Scope
}

// AuthorizationInput starts same-device browser authorization.
type AuthorizationInput struct {
	StartInput
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// Request is a device authorization request.
type Request struct {
	DeviceCode string
	UserCode   string
	VerifyURL  string
	ExpiresAt  time.Time
	Interval   time.Duration
}

// Authorization is the Illarin page a native application opens.
type Authorization struct {
	URL       string
	ExpiresAt time.Time
}

// Pending is what a creator reviews before deciding.
type Pending struct {
	Declaration
	Scopes        []Scope
	ExpiresAt     time.Time
	ApprovalToken string
}

// Redirect is a validated loopback destination after a browser decision.
type Redirect struct {
	URL string
}

// Instance is one independently authorised installation.
type Instance struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Declaration
	Prefix     string
	Scopes     []Scope
	LinkedAt   time.Time
	LastSeenAt *time.Time
	RevokedAt  *time.Time
}

// Grants reports whether the instance has a scope.
func (i Instance) Grants(scope Scope) bool {
	for _, held := range i.Scopes {
		if held == scope {
			return true
		}
	}
	return false
}

// TokenGrant is the only response that exposes a new token pair.
type TokenGrant struct {
	Instance             Instance
	AccessToken          string
	AccessTokenExpiresAt time.Time
	RefreshToken         string
}

const (
	maxNameLength       = 64
	maxVersionLength    = 64
	maxIdentifierLength = 64
	maxDeclarationItems = 32
	maxRedirectLength   = 512
	protocolVersion     = 1

	codeAlphabet           = "BCDFGHJKLMNPQRSTVWXZ23456789"
	codeLength             = 8
	codeGroupSize          = 4
	secretBytes            = 32
	opaqueCodeLength       = 43
	maxUserCodeInputLength = 16
	credentialLength       = 3 + 1 + codeLength + 1 + opaqueCodeLength

	accessTokenKind  = "ia1"
	refreshTokenKind = "ir1"
)

var (
	capabilityPattern = regexp.MustCompile(`^[a-z][a-z0-9.-]*:[a-z][a-z0-9._-]*$`)
	targetPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	pkcePattern       = regexp.MustCompile(`^[A-Za-z0-9._~-]+$`)
)

func validateStart(in StartInput) (StartInput, error) {
	declaration, err := validateDeclaration(in.Declaration)
	if err != nil {
		return StartInput{}, err
	}
	scopes, err := canonicalScopes(in.Scopes)
	if err != nil {
		return StartInput{}, err
	}
	return StartInput{Declaration: declaration, Scopes: scopes}, nil
}

func validateDeclaration(in Declaration) (Declaration, error) {
	application, err := validateText(in.ApplicationName, maxNameLength)
	if err != nil {
		return Declaration{}, ErrInvalidName
	}
	instance, err := validateText(in.InstanceName, maxNameLength)
	if err != nil {
		return Declaration{}, ErrInvalidName
	}
	version := strings.TrimSpace(in.ApplicationVersion)
	if version != "" {
		if _, err := validateText(version, maxVersionLength); err != nil {
			return Declaration{}, ErrInvalidDeclaration
		}
	}
	if in.ProtocolVersion != protocolVersion || in.Capabilities == nil || in.AcceptedTargets == nil {
		return Declaration{}, ErrInvalidDeclaration
	}
	capabilities, err := canonicalIdentifiers(in.Capabilities, capabilityPattern)
	if err != nil {
		return Declaration{}, ErrInvalidDeclaration
	}
	targets, err := canonicalIdentifiers(in.AcceptedTargets, targetPattern)
	if err != nil {
		return Declaration{}, ErrInvalidDeclaration
	}
	return Declaration{
		ApplicationName: application, InstanceName: instance,
		ApplicationVersion: version, ProtocolVersion: protocolVersion,
		Capabilities: capabilities, AcceptedTargets: targets,
	}, nil
}

func validateText(raw string, limit int) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > limit {
		return "", ErrInvalidDeclaration
	}
	for _, char := range value {
		if !unicode.IsPrint(char) {
			return "", ErrInvalidDeclaration
		}
	}
	return value, nil
}

func canonicalIdentifiers(values []string, pattern *regexp.Regexp) ([]string, error) {
	if len(values) > maxDeclarationItems {
		return nil, ErrInvalidDeclaration
	}
	result := make([]string, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		if len(value) > maxIdentifierLength || !pattern.MatchString(value) {
			return nil, ErrInvalidDeclaration
		}
		if _, exists := seen[value]; exists {
			return nil, ErrInvalidDeclaration
		}
		seen[value] = struct{}{}
		result[index] = value
	}
	return result, nil
}

func canonicalScopes(requested []Scope) ([]Scope, error) {
	if len(requested) == 0 {
		return nil, ErrInvalidScopes
	}
	granted := make([]Scope, 0, len(scopeOrder))
	for _, known := range scopeOrder {
		count := 0
		for _, asked := range requested {
			if asked == known {
				count++
			}
		}
		if count > 1 {
			return nil, ErrInvalidScopes
		}
		if count == 1 {
			granted = append(granted, known)
		}
	}
	if len(granted) != len(requested) {
		return nil, ErrInvalidScopes
	}
	return granted, nil
}

func scopeStrings(scopes []Scope) []string {
	stored := make([]string, len(scopes))
	for index, scope := range scopes {
		stored[index] = string(scope)
	}
	return stored
}

func scopesFrom(stored []string) []Scope {
	scopes := make([]Scope, len(stored))
	for index, value := range stored {
		scopes[index] = Scope(value)
	}
	return scopes
}

func newCode(length int) (string, error) {
	limit := 256 / len(codeAlphabet) * len(codeAlphabet)
	code := make([]byte, 0, length)
	buffer := make([]byte, length)
	for len(code) < length {
		if _, err := rand.Read(buffer); err != nil {
			return "", fmt.Errorf("make code: %w", err)
		}
		for _, value := range buffer {
			if int(value) < limit && len(code) < length {
				code = append(code, codeAlphabet[int(value)%len(codeAlphabet)])
			}
		}
	}
	return string(code), nil
}

// FormatUserCode groups a user code for reading aloud.
func FormatUserCode(code string) string {
	return code[:codeGroupSize] + "-" + code[codeGroupSize:]
}

func normalizeUserCode(raw string) (string, bool) {
	if len(raw) > maxUserCodeInputLength {
		return "", false
	}
	var code strings.Builder
	for _, char := range strings.ToUpper(raw) {
		if strings.ContainsRune(codeAlphabet, char) {
			code.WriteRune(char)
		} else if char != '-' && char != ' ' {
			return "", false
		}
	}
	if code.Len() != codeLength {
		return "", false
	}
	return code.String(), true
}

func newOpaqueCode() (string, []byte, error) {
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("make secret: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(secret)
	return code, hashOf(code), nil
}

func opaqueCodeHash(code string) ([]byte, bool) {
	if len(code) != opaqueCodeLength {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || len(raw) != secretBytes {
		return nil, false
	}
	return hashOf(code), true
}

func newCredential(kind string) (token, prefix string, hash []byte, err error) {
	prefix, err = newCode(codeLength)
	if err != nil {
		return "", "", nil, err
	}
	secret := make([]byte, secretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", "", nil, fmt.Errorf("make token: %w", err)
	}
	token = kind + "." + prefix + "." + base64.RawURLEncoding.EncodeToString(secret)
	return token, prefix, hashOf(token), nil
}

func credentialHash(token, kind string) ([]byte, bool) {
	if len(token) != credentialLength {
		return nil, false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != kind || len(parts[1]) != codeLength {
		return nil, false
	}
	for _, char := range parts[1] {
		if !strings.ContainsRune(codeAlphabet, char) {
			return nil, false
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(raw) != secretBytes {
		return nil, false
	}
	return hashOf(token), true
}

func validateAuthorization(in AuthorizationInput) (AuthorizationInput, error) {
	start, err := validateStart(in.StartInput)
	if err != nil {
		return AuthorizationInput{}, err
	}
	if !validLoopbackRedirect(in.RedirectURI) {
		return AuthorizationInput{}, ErrInvalidRedirect
	}
	if len(in.State) < 32 || len(in.State) > 128 || !pkcePattern.MatchString(in.State) {
		return AuthorizationInput{}, ErrInvalidPKCE
	}
	if in.CodeChallengeMethod != "S256" || !validChallenge(in.CodeChallenge) {
		return AuthorizationInput{}, ErrInvalidPKCE
	}
	return AuthorizationInput{
		StartInput: start, RedirectURI: in.RedirectURI, State: in.State,
		CodeChallenge: in.CodeChallenge, CodeChallengeMethod: "S256",
	}, nil
}

func validLoopbackRedirect(raw string) bool {
	if len(raw) > maxRedirectLength {
		return false
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme != "http" || parsed.Opaque != "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" {
		return false
	}
	host := parsed.Hostname()
	if host != "127.0.0.1" && host != "::1" {
		return false
	}
	port, err := strconv.Atoi(parsed.Port())
	return err == nil && port > 0 && port <= 65535
}

func validChallenge(value string) bool {
	if len(value) != 43 || !pkcePattern.MatchString(value) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(raw) == sha256.Size
}

func challengeMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 || !pkcePattern.MatchString(verifier) {
		return false
	}
	digest := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}

func redirectWith(raw, key, value, state string) (string, error) {
	if !validLoopbackRedirect(raw) {
		return "", ErrInvalidRedirect
	}
	parsed, _ := url.Parse(raw)
	query := parsed.Query()
	query.Set(key, value)
	query.Set("state", state)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func hashOf(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
