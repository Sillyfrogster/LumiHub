package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/account"
)

func discordServer(t *testing.T, user string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"a-token"}`))
	})
	mux.HandleFunc("/users/@me", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(user))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func exchange(t *testing.T, user string) account.DiscordProfile {
	t.Helper()
	server := discordServer(t, user)
	config := DefaultConfig("id", "secret", "http://localhost:3000/callback")
	config.TokenEndpoint = server.URL + "/token"
	config.UserEndpoint = server.URL + "/users/@me"
	client, err := NewClient(config, server.Client())
	if err != nil {
		t.Fatalf("build the client: %v", err)
	}
	profile, err := client.ExchangeProfile(context.Background(), "a-code")
	if err != nil {
		t.Fatalf("exchange the profile: %v", err)
	}
	return profile
}

func TestTheProfileCarriesTheDiscordCDNURLs(t *testing.T) {
	profile := exchange(t, `{
		"id": "100000000000000001", "username": "riverstonekeep", "global_name": "Reed",
		"avatar": "0123456789abcdef0123456789abcdef",
		"banner": "a_fedcba9876543210fedcba9876543210"
	}`)

	wantAvatar := "https://cdn.discordapp.com/avatars/100000000000000001/" +
		"0123456789abcdef0123456789abcdef.png"
	wantBanner := "https://cdn.discordapp.com/banners/100000000000000001/" +
		"a_fedcba9876543210fedcba9876543210.png?size=1024"
	if profile.AvatarURL != wantAvatar {
		t.Errorf("avatar = %q, want %q", profile.AvatarURL, wantAvatar)
	}
	if profile.BannerURL != wantBanner {
		t.Errorf("banner = %q, want %q", profile.BannerURL, wantBanner)
	}
	if profile.DisplayName != "Reed" {
		t.Errorf("display name = %q, want the Discord global name", profile.DisplayName)
	}
}

func TestAnAccountWithNoImagesCarriesNoURLs(t *testing.T) {
	profile := exchange(t, `{"id": "7391", "username": "tallowmoth"}`)

	if profile.AvatarURL != "" || profile.BannerURL != "" {
		t.Errorf("URLs = %q and %q, want none", profile.AvatarURL, profile.BannerURL)
	}
	if profile.DisplayName != "tallowmoth" {
		t.Errorf("display name = %q, want the username as the fallback", profile.DisplayName)
	}
}
