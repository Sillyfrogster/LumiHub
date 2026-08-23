package http

import (
	"testing"

	"github.com/Sillyfrogster/Illarin/api/openapi"
	"github.com/goccy/go-yaml"
)

func TestCredentialCookiesCarryTheProductName(t *testing.T) {
	names := map[string]string{
		"session":      sessionCookieName,
		"oauth state":  oauthStateCookieName,
		"oauth return": oauthReturnCookieName,
	}
	want := map[string]string{
		"session":      "illarin_session",
		"oauth state":  "illarin_discord_state",
		"oauth return": "illarin_discord_return",
	}
	for role, got := range names {
		if got != want[role] {
			t.Errorf("%s cookie = %q, want %q", role, got, want[role])
		}
	}
}

func TestContractNamesTheSessionCookieTheHandlersSet(t *testing.T) {
	var document struct {
		Components struct {
			SecuritySchemes struct {
				SessionCookie struct {
					Name string `yaml:"name"`
				} `yaml:"sessionCookie"`
			} `yaml:"securitySchemes"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(openapi.Contract, &document); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if got := document.Components.SecuritySchemes.SessionCookie.Name; got != sessionCookieName {
		t.Errorf("contract session cookie = %q, want %q", got, sessionCookieName)
	}
}
