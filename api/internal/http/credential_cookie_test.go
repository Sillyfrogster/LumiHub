package http

import (
	"testing"

	"github.com/Sillyfrogster/Illarin/api/openapi"
	"github.com/goccy/go-yaml"
)

func TestCredentialCookiesCarryTheProductName(t *testing.T) {
	cookies := []struct {
		role string
		got  string
		want string
	}{
		{"session", sessionCookieName, "illarin_session"},
		{"oauth state", oauthStateCookieName, "illarin_discord_state"},
		{"oauth return", oauthReturnCookieName, "illarin_discord_return"},
	}
	for _, cookie := range cookies {
		if cookie.got != cookie.want {
			t.Errorf("%s cookie = %q, want %q", cookie.role, cookie.got, cookie.want)
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
				BrowserMutation struct {
					Name string `yaml:"name"`
				} `yaml:"browserMutation"`
			} `yaml:"securitySchemes"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(openapi.Contract, &document); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if got := document.Components.SecuritySchemes.SessionCookie.Name; got != sessionCookieName {
		t.Errorf("contract session cookie = %q, want %q", got, sessionCookieName)
	}
	if got := document.Components.SecuritySchemes.BrowserMutation.Name; got != browserMutationHeader {
		t.Errorf("contract browser mutation header = %q, want %q", got, browserMutationHeader)
	}
}
