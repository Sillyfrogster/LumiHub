package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/account"
)

const maxResponseBytes = 1 << 20

type Config struct {
	ClientID              string
	ClientSecret          string
	RedirectURI           string
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserEndpoint          string
}

func DefaultConfig(clientID, clientSecret, redirectURI string) Config {
	return Config{
		ClientID:              clientID,
		ClientSecret:          clientSecret,
		RedirectURI:           redirectURI,
		AuthorizationEndpoint: "https://discord.com/oauth2/authorize",
		TokenEndpoint:         "https://discord.com/api/v10/oauth2/token",
		UserEndpoint:          "https://discord.com/api/v10/users/@me",
	}
}

type Client struct {
	config Config
	http   *http.Client
}

func NewClient(config Config, httpClient *http.Client) (*Client, error) {
	for name, value := range map[string]string{
		"client id":              config.ClientID,
		"client secret":          config.ClientSecret,
		"redirect URI":           config.RedirectURI,
		"authorization endpoint": config.AuthorizationEndpoint,
		"token endpoint":         config.TokenEndpoint,
		"user endpoint":          config.UserEndpoint,
	} {
		if value == "" {
			return nil, fmt.Errorf("Discord %s is required", name)
		}
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{config: config, http: httpClient}, nil
}

func (c *Client) AuthorizationURL(state string) string {
	query := url.Values{
		"client_id":     {c.config.ClientID},
		"redirect_uri":  {c.config.RedirectURI},
		"response_type": {"code"},
		"scope":         {"identify email"},
		"state":         {state},
	}
	return c.config.AuthorizationEndpoint + "?" + query.Encode()
}

func (c *Client) ExchangeProfile(ctx context.Context, code string) (account.DiscordProfile, error) {
	token, err := c.exchangeToken(ctx, code)
	if err != nil {
		return account.DiscordProfile{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.UserEndpoint, nil)
	if err != nil {
		return account.DiscordProfile{}, fmt.Errorf("build Discord user request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.http.Do(request)
	if err != nil {
		return account.DiscordProfile{}, fmt.Errorf("request Discord user: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return account.DiscordProfile{}, fmt.Errorf("Discord user response status %d", response.StatusCode)
	}
	var user struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
	}
	if err := decodeJSON(response.Body, &user); err != nil {
		return account.DiscordProfile{}, fmt.Errorf("decode Discord user: %w", err)
	}
	if user.ID == "" || user.Username == "" {
		return account.DiscordProfile{}, errors.New("Discord user response is incomplete")
	}
	return account.DiscordProfile{
		Subject:       user.ID,
		Username:      user.Username,
		Email:         user.Email,
		EmailVerified: user.Verified,
	}, nil
}

func (c *Client) exchangeToken(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURI},
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.config.TokenEndpoint, strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("build Discord token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("request Discord token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("Discord token response status %d", response.StatusCode)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := decodeJSON(response.Body, &token); err != nil {
		return "", fmt.Errorf("decode Discord token: %w", err)
	}
	if token.AccessToken == "" {
		return "", errors.New("Discord token response has no access token")
	}
	return token.AccessToken, nil
}

func decodeJSON(body io.Reader, destination any) error {
	decoder := json.NewDecoder(io.LimitReader(body, maxResponseBytes))
	return decoder.Decode(destination)
}
