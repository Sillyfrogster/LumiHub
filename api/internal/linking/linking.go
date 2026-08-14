// Package linking joins a client application to a creator's account and holds
// the credential that join produces.
//
// LumiHub authenticates the creator and the individual installation, never the
// software vendor, so there is no registration step and any implementation may
// link. A client starts a link request, shows the creator a short code, and
// polls until the creator approves it.
package linking

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Scope is what a linked instance is permitted to do. A client asks for what it
// needs and the creator sees the list before approving.
type Scope string

const (
	ScopeReceiveAssets Scope = "asset:receive"
	ScopeSyncLibrary   Scope = "library:sync"
)

// scopeOrder is the order scopes are stored and shown in, so the same grant
// always reads the same way.
var scopeOrder = []Scope{ScopeReceiveAssets, ScopeSyncLibrary}

var (
	ErrInvalidName          = errors.New("a link request needs a name a creator can recognise")
	ErrInvalidScopes        = errors.New("a link request needs at least one known scope")
	ErrLinkRequestNotFound  = errors.New("no link request is waiting on that code")
	ErrPollTooSoon          = errors.New("polled faster than the interval")
	ErrTooManyCodes         = errors.New("too many link codes were entered")
	ErrInstanceNotFound     = errors.New("no live instance has that id")
	ErrInstanceCredential   = errors.New("credential does not identify a live instance")
	ErrInstanceMissingScope = errors.New("instance was not granted that scope")
)

// Request is what a client receives when it starts linking. The user code goes
// to the creator and the device code stays with the client.
type Request struct {
	DeviceCode  string
	UserCode    string
	VerifyURL   string
	CompleteURL string
	ExpiresAt   time.Time
	Interval    time.Duration
}

// Pending is what a creator sees before they approve.
type Pending struct {
	Name      string
	Scopes    []Scope
	ExpiresAt time.Time
}

// Instance is one installation a creator has authorised. A revoked one keeps
// its name and dates so the creator can see what they cut.
type Instance struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	Prefix     string
	Scopes     []Scope
	LinkedAt   time.Time
	LastSeenAt *time.Time
	RevokedAt  *time.Time
}

// Grants says whether the instance holds a scope.
func (i Instance) Grants(scope Scope) bool {
	for _, held := range i.Scopes {
		if held == scope {
			return true
		}
	}
	return false
}

// Credential is a new instance and the one time its secret is readable.
type Credential struct {
	Instance Instance
	Token    string
}

const (
	maxNameLength = 64

	// codeAlphabet leaves out vowels so a code cannot spell anything, and
	// leaves out the characters people confuse when reading one aloud.
	codeAlphabet  = "BCDFGHJKLMNPQRSTVWXZ23456789"
	codeLength    = 8
	codeGroupSize = 4
)

func validateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]rune(name)) > maxNameLength {
		return "", ErrInvalidName
	}
	for _, char := range name {
		if !unicode.IsPrint(char) {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

// canonicalScopes rejects an unknown or repeated scope and puts the rest in one
// fixed order.
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
	for i, scope := range scopes {
		stored[i] = string(scope)
	}
	return stored
}

func scopesFrom(stored []string) []Scope {
	scopes := make([]Scope, len(stored))
	for i, value := range stored {
		scopes[i] = Scope(value)
	}
	return scopes
}

// newCode draws characters from the alphabet without favouring any of them.
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

// FormatUserCode groups a user code so it is easier to read out.
func FormatUserCode(code string) string {
	return code[:codeGroupSize] + "-" + code[codeGroupSize:]
}

// normalizeUserCode accepts whatever a creator typed and returns the stored
// form, or false if those characters cannot be a code.
func normalizeUserCode(raw string) (string, bool) {
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

func newDeviceCode() (string, []byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("make device code: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(secret)
	return code, hashOf(code), nil
}

func deviceCodeHash(code string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	return hashOf(code), true
}

// newToken builds an instance credential. The prefix is readable and is kept so
// a creator can tell two links apart. The rest is the secret.
func newToken() (token, prefix string, hash []byte, err error) {
	prefix, err = newCode(codeLength)
	if err != nil {
		return "", "", nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", "", nil, fmt.Errorf("make credential: %w", err)
	}
	token = prefix + "." + base64.RawURLEncoding.EncodeToString(secret)
	return token, prefix, hashOf(token), nil
}

// tokenHash refuses anything that is not shaped like a credential before it
// reaches the database.
func tokenHash(token string) ([]byte, bool) {
	prefix, secret, found := strings.Cut(token, ".")
	if !found || len(prefix) != codeLength {
		return nil, false
	}
	for _, char := range prefix {
		if !strings.ContainsRune(codeAlphabet, char) {
			return nil, false
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(raw) != 32 {
		return nil, false
	}
	return hashOf(token), true
}

func hashOf(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}
