// Package discord implements the Discord OAuth login flow.
//
// # Flow
//
//  1. The frontend redirects the browser to Discord's authorisation URL.
//  2. Discord redirects back to Discord__RedirectUri with a ?code= query parameter.
//  3. This service exchanges the code for an access token.
//  4. It fetches the profile from /api/users/@me with that token.
//  5. The caller upserts the user and issues a JWT.
//
// Step 3 includes the redirect URI because Discord binds the code to it; sending
// a different value than the one used in step 1 fails the exchange, and the
// error from Discord is generic, so the value is always taken from configuration
// rather than from the request.
//
// # Scope
//
// Only the identify scope is requested. The site never reads a user's email,
// server membership or messages, so there is no reason to ask for more.
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
)

// defaultBaseURL is the real Discord API.
//
// Every endpoint is derived from the client's base URL rather than hard-coded,
// so the whole client can be pointed at a stub in tests. With the default base
// URL the resulting URLs are exactly Discord's own, so nothing changes in
// production.
const defaultBaseURL = "https://discord.com"

// Config holds the Discord application credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// Profile is the subset of a Discord account this service stores.
type Profile struct {
	// ID is Discord's snowflake, stored as text because it is an identifier
	// rather than a quantity.
	ID string `json:"id"`

	// Username is the account's display name. Discord also has a global name
	// and a discriminator; neither is stored, because both are mutable and the
	// profile name is what users recognise.
	Username string `json:"username"`

	// Avatar is the avatar hash, not a URL. A null hash means the user has
	// uploaded a custom avatar and the hash cannot be turned into a path.
	Avatar *string `json:"avatar"`

	// Banner is the profile banner hash, used for the larger images.
	Banner *string `json:"banner"`
}

// AvatarURL builds the CDN URL for a profile image.
//
// Returns an empty string when there is no custom avatar, in which case the
// caller should fall back to a generated identicon based on the user id. A
// hash of null means the user uses the default avatar, and there is no URL for
// it.
//
// The size argument is a Discord CDN power-of-two value: 128 for a small
// avatar, 1024 for a large one.
func (p *Profile) AvatarURL(size int) string {
	if p.Avatar == nil || *p.Avatar == "" {
		return ""
	}
	if size <= 0 {
		size = 128
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s?size=%d",
		p.ID, *p.Avatar, imageExtension(p.ID), size)
}

// BannerURL builds the banner CDN URL, with the same null-hash rule as the
// avatar.
func (p *Profile) BannerURL(size int) string {
	if p.Banner == nil || *p.Banner == "" {
		return ""
	}
	if size <= 0 {
		size = 1024
	}
	return fmt.Sprintf("https://cdn.discordapp.com/banners/%s/%s.%s?size=%d",
		p.ID, *p.Banner, imageExtension(p.ID), size)
}

// imageExtension returns the file extension for a Discord CDN asset.
//
// Discord stores most assets as PNG but switched animated assets to GIF, and the
// extension is not derivable from the hash. Hashes beginning with 'a_' are the
// animated variant.
func imageExtension(id string) string {
	if strings.HasPrefix(id, "a_") {
		return "gif"
	}
	return "png"
}

// APIError is a failure reported by Discord.
type APIError struct {
	// StatusCode is Discord's HTTP status.
	StatusCode int

	// Message describes the failure.
	Message string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("discord api error %d: %s", e.StatusCode, e.Message)
}

// Client calls the Discord API.
type Client struct {
	http    *http.Client
	config  Config
	baseURL string
}

// NewClient builds a Discord client.
//
// baseURL defaults to the real Discord API and is overridable so the flow can be
// exercised against a stub in tests without reaching the network.
func NewClient(cfg Config, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// A timeout is essential: this client sits on the login path, and a hung
	// request would hold a connection until the request context expires.
	return &Client{
		http:    &http.Client{Timeout: 15 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		config:  cfg,
	}
}

// AuthorizeURL builds the URL the browser is redirected to.
//
// scope is fixed to identify. The state parameter is passed through by the
// caller for CSRF protection and is not generated here, because the caller owns
// the cookie that validates it.
func (c *Client) AuthorizeURL(state string) string {
	query := url.Values{}
	query.Set("client_id", c.config.ClientID)
	query.Set("redirect_uri", c.config.RedirectURI)
	query.Set("response_type", "code")
	query.Set("scope", "identify")
	if state != "" {
		query.Set("state", state)
	}
	return c.baseURL + "/oauth2/authorize?" + query.Encode()
}

// tokenResponse is the payload from the token endpoint.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// ExchangeCode swaps an authorisation code for an access token.
//
// The redirect URI is sent because Discord binds the code to it. Omitting it, or
// sending a different value from the one used to obtain the code, fails the
// exchange.
func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", &APIError{StatusCode: http.StatusBadRequest, Message: "authorization code is empty"}
	}

	form := url.Values{}
	form.Set("client_id", c.config.ClientID)
	form.Set("client_secret", c.config.ClientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.config.RedirectURI)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", readAPIError(response)
	}

	var payload tokenResponse
	if err := decodeJSON(response.Body, &payload); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", &APIError{StatusCode: response.StatusCode, Message: "no access token in response"}
	}
	return payload.AccessToken, nil
}

// Profile fetches the authenticated user's Discord profile.
func (c *Client) Profile(ctx context.Context, accessToken string) (*Profile, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, &APIError{StatusCode: http.StatusUnauthorized, Message: "access token is empty"}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("build profile request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, readAPIError(response)
	}

	var profile Profile
	if err := decodeJSON(response.Body, &profile); err != nil {
		return nil, fmt.Errorf("decode profile: %w", err)
	}
	if profile.ID == "" {
		return nil, &APIError{StatusCode: response.StatusCode, Message: "profile has no id"}
	}
	return &profile, nil
}

// ErrNotConfigured reports missing Discord application settings.
var ErrNotConfigured = errors.New("discord oauth is not configured")

// Validate checks that every required setting is present.
func (c *Client) Validate() error {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(c.config.ClientID) == "" {
		missing = append(missing, "Discord__ClientId")
	}
	if strings.TrimSpace(c.config.ClientSecret) == "" {
		missing = append(missing, "Discord__ClientSecret")
	}
	if strings.TrimSpace(c.config.RedirectURI) == "" {
		missing = append(missing, "Discord__RedirectUri")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrNotConfigured, strings.Join(missing, ", "))
	}
	return nil
}

// readAPIError converts a non-2xx response into an APIError.
func readAPIError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(response.StatusCode)
	} else {
		// Discord returns JSON; keep only the human-readable part so a log line
		// stays readable.
		var structured struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if err := json.Unmarshal(body, &structured); err == nil {
			if structured.ErrorDescription != "" {
				message = structured.ErrorDescription
			} else if structured.Error != "" {
				message = structured.Error
			}
		}
	}

	return &APIError{StatusCode: response.StatusCode, Message: message}
}

// decodeJSON decodes a response body.
func decodeJSON(reader io.Reader, target any) error {
	return json.NewDecoder(reader).Decode(target)
}
