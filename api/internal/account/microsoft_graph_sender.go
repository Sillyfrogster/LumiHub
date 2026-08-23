package account

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	microsoftGraphScope = "https://graph.microsoft.com/.default"
	maxMicrosoftError   = 8 << 10
)

type MicrosoftGraphSettings struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	Mailbox      string
}

type MicrosoftGraphSender struct {
	settings MicrosoftGraphSettings
	client   *http.Client
	tokenURL string
	graphURL string

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func NewMicrosoftGraphSender(settings MicrosoftGraphSettings) (*MicrosoftGraphSender, error) {
	return newMicrosoftGraphSender(
		settings,
		&http.Client{Timeout: 15 * time.Second},
		"https://login.microsoftonline.com",
		"https://graph.microsoft.com/v1.0",
	)
}

func newMicrosoftGraphSender(
	settings MicrosoftGraphSettings,
	client *http.Client,
	tokenURL string,
	graphURL string,
) (*MicrosoftGraphSender, error) {
	for name, value := range map[string]string{
		"tenant id":     settings.TenantID,
		"client id":     settings.ClientID,
		"client secret": settings.ClientSecret,
		"mailbox":       settings.Mailbox,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("Microsoft 365 %s is required", name)
		}
	}
	if client == nil {
		return nil, fmt.Errorf("Microsoft 365 HTTP client is required")
	}
	return &MicrosoftGraphSender{
		settings: settings,
		client:   client,
		tokenURL: strings.TrimRight(tokenURL, "/"),
		graphURL: strings.TrimRight(graphURL, "/"),
	}, nil
}

func (s *MicrosoftGraphSender) SendVerification(ctx context.Context, address, link string) error {
	return s.send(
		ctx,
		address,
		"Verify your Illarin email",
		"Verify your Illarin email address by opening this link:\r\n\r\n"+link+"\r\n",
	)
}

func (s *MicrosoftGraphSender) SendPasswordReset(ctx context.Context, address, link string) error {
	return s.send(
		ctx,
		address,
		"Reset your Illarin password",
		"Set a new Illarin password by opening this link:\r\n\r\n"+link+"\r\n",
	)
}

func (s *MicrosoftGraphSender) send(ctx context.Context, address, subject, body string) error {
	token, err := s.token(ctx)
	if err != nil {
		return err
	}
	payload := struct {
		Message struct {
			Subject string `json:"subject"`
			Body    struct {
				ContentType string `json:"contentType"`
				Content     string `json:"content"`
			} `json:"body"`
			Recipients []struct {
				EmailAddress struct {
					Address string `json:"address"`
				} `json:"emailAddress"`
			} `json:"toRecipients"`
		} `json:"message"`
		SaveToSentItems bool `json:"saveToSentItems"`
	}{SaveToSentItems: true}
	payload.Message.Subject = subject
	payload.Message.Body.ContentType = "Text"
	payload.Message.Body.Content = body
	payload.Message.Recipients = make([]struct {
		EmailAddress struct {
			Address string `json:"address"`
		} `json:"emailAddress"`
	}, 1)
	payload.Message.Recipients[0].EmailAddress.Address = address

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Microsoft 365 message: %w", err)
	}
	endpoint := s.graphURL + "/users/" + url.PathEscape(s.settings.Mailbox) + "/sendMail"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(encoded)))
	if err != nil {
		return fmt.Errorf("build Microsoft 365 message request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send Microsoft 365 message: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return microsoftResponseError("send Microsoft 365 message", response)
	}
	return nil
}

func (s *MicrosoftGraphSender) token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && time.Now().Add(time.Minute).Before(s.expiresAt) {
		return s.accessToken, nil
	}

	form := url.Values{
		"client_id":     {s.settings.ClientID},
		"client_secret": {s.settings.ClientSecret},
		"grant_type":    {"client_credentials"},
		"scope":         {microsoftGraphScope},
	}
	endpoint := s.tokenURL + "/" + url.PathEscape(s.settings.TenantID) + "/oauth2/v2.0/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build Microsoft 365 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Microsoft 365 token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", microsoftResponseError("request Microsoft 365 token", response)
	}
	var token struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxMicrosoftError)).Decode(&token); err != nil {
		return "", fmt.Errorf("read Microsoft 365 token: %w", err)
	}
	if token.AccessToken == "" || token.ExpiresIn <= 0 {
		return "", fmt.Errorf("Microsoft 365 returned an incomplete token")
	}
	s.accessToken = token.AccessToken
	s.expiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return s.accessToken, nil
}

func microsoftResponseError(action string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxMicrosoftError))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s: HTTP %d", action, response.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", action, response.StatusCode, detail)
}
