package account

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMicrosoftGraphSendsIllarinMailAndReusesItsToken(t *testing.T) {
	var tokens atomic.Int32
	var messages atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tenant/oauth2/v2.0/token":
			tokens.Add(1)
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read token form: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if form.Get("client_id") != "client" || form.Get("client_secret") != "secret" ||
				form.Get("scope") != microsoftGraphScope || form.Get("grant_type") != "client_credentials" {
				t.Errorf("token form = %v", form)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":3600}`)
		case "/users/mail@illarin.test/sendMail":
			messages.Add(1)
			if r.Header.Get("Authorization") != "Bearer token" {
				t.Errorf("authorization = %q", r.Header.Get("Authorization"))
			}
			var body struct {
				Message struct {
					Subject string `json:"subject"`
					Body    struct {
						Content string `json:"content"`
					} `json:"body"`
					Recipients []struct {
						EmailAddress struct {
							Address string `json:"address"`
						} `json:"emailAddress"`
					} `json:"toRecipients"`
				} `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message: %v", err)
			}
			if !strings.Contains(body.Message.Subject, "Illarin") ||
				!strings.Contains(body.Message.Body.Content, "https://illarin.test/") ||
				body.Message.Recipients[0].EmailAddress.Address != "creator@example.com" {
				t.Errorf("message = %+v", body.Message)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	sender, err := newMicrosoftGraphSender(MicrosoftGraphSettings{
		TenantID: "tenant", ClientID: "client", ClientSecret: "secret", Mailbox: "mail@illarin.test",
	}, server.Client(), server.URL, server.URL)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	if err := sender.SendVerification(context.Background(), "creator@example.com", "https://illarin.test/verify"); err != nil {
		t.Fatalf("send verification: %v", err)
	}
	if err := sender.SendPasswordReset(context.Background(), "creator@example.com", "https://illarin.test/reset"); err != nil {
		t.Fatalf("send reset: %v", err)
	}
	if tokens.Load() != 1 || messages.Load() != 2 {
		t.Fatalf("token requests = %d, messages = %d", tokens.Load(), messages.Load())
	}
}

func TestMicrosoftGraphDoesNotHideARefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"access_token":"token","expires_in":3600}`)
			return
		}
		http.Error(w, `{"error":{"code":"ErrorAccessDenied"}}`, http.StatusForbidden)
	}))
	defer server.Close()

	sender, err := newMicrosoftGraphSender(MicrosoftGraphSettings{
		TenantID: "tenant", ClientID: "client", ClientSecret: "secret", Mailbox: "mail@illarin.test",
	}, server.Client(), server.URL, server.URL)
	if err != nil {
		t.Fatalf("new sender: %v", err)
	}
	err = sender.SendVerification(context.Background(), "creator@example.com", "https://illarin.test/verify")
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "ErrorAccessDenied") {
		t.Fatalf("send error = %v", err)
	}
}
