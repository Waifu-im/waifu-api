package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// stubServer stands in for the Discord API.
//
// The token and user endpoints are pinned to the real hosts in the client, so
// only the user endpoint is reachable in tests. That is deliberate: it keeps the
// code exchange pinned to Discord and still allows the profile fetch to be
// exercised without network access.
func stubServer(t *testing.T, profile *Profile, status int) *Client {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/@me", func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"invalid_token"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(profile)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return NewClient(Config{
		ClientID:     "client",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/auth/callback",
	}, server.URL)
}

func TestAuthorizeURLCarriesConfiguredClient(t *testing.T) {
	client := NewClient(Config{
		ClientID:    "my-client",
		RedirectURI: "http://localhost:8084/auth/callback",
	}, "")

	parsed, err := url.Parse(client.AuthorizeURL("state-123"))
	if err != nil {
		t.Fatalf("authorize url is not parseable: %v", err)
	}

	if parsed.Host != "discord.com" {
		t.Errorf("host = %q, want discord.com", parsed.Host)
	}

	query := parsed.Query()
	if query.Get("client_id") != "my-client" {
		t.Errorf("client_id = %q, want my-client", query.Get("client_id"))
	}
	if query.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", query.Get("response_type"))
	}
	if query.Get("state") != "state-123" {
		t.Errorf("state = %q, want state-123", query.Get("state"))
	}
	// Only identify is requested. Asking for more would be a privacy regression.
	if query.Get("scope") != "identify" {
		t.Errorf("scope = %q, want identify", query.Get("scope"))
	}
}

func TestProfileRejectsBadStatus(t *testing.T) {
	client := stubServer(t, nil, http.StatusUnauthorized)

	_, err := client.Profile(context.Background(), "token")
	if err == nil {
		t.Fatal("a 401 from Discord should be reported as an error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", apiErr.StatusCode)
	}
}

func TestProfileReturnsFields(t *testing.T) {
	avatar := "abc123hash"
	client := stubServer(t, &Profile{ID: "123456789", Username: "tester", Avatar: &avatar}, http.StatusOK)

	profile, err := client.Profile(context.Background(), "token")
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if profile.ID != "123456789" {
		t.Errorf("id = %q, want 123456789", profile.ID)
	}
	if profile.Username != "tester" {
		t.Errorf("username = %q, want tester", profile.Username)
	}
}

func TestProfileRequiresToken(t *testing.T) {
	client := stubServer(t, nil, http.StatusOK)

	if _, err := client.Profile(context.Background(), "  "); err == nil {
		t.Error("an empty access token should be rejected before any request")
	}
}

func TestProfileRejectsMissingID(t *testing.T) {
	// A profile with no id means the response was not what was expected, and
	// storing it would produce a user keyed on an empty string.
	client := stubServer(t, &Profile{Username: "no-id"}, http.StatusOK)

	if _, err := client.Profile(context.Background(), "token"); err == nil {
		t.Error("a profile with no id should be rejected")
	}
}

func TestAvatarURL(t *testing.T) {
	cases := []struct {
		name   string
		avatar *string
		want   string
	}{
		{
			name:   "hash present",
			avatar: strPtr("abc123"),
			want:   "https://cdn.discordapp.com/avatars/999/abc123.png?size=128",
		},
		{
			name:   "null hash means default avatar",
			avatar: nil,
			want:   "",
		},
		{
			name:   "empty hash means default avatar",
			avatar: strPtr(""),
			want:   "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			profile := Profile{ID: "999", Avatar: c.avatar}
			if got := profile.AvatarURL(0); got != c.want {
				t.Errorf("AvatarURL() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAvatarURLSizeIsHonoured(t *testing.T) {
	profile := Profile{ID: "999", Avatar: strPtr("abc")}

	got := profile.AvatarURL(256)
	if !strings.Contains(got, "size=256") {
		t.Errorf("AvatarURL(256) = %q, want it to request size 256", got)
	}
}

func TestValidateReportsEveryMissingSetting(t *testing.T) {
	client := NewClient(Config{}, "")

	err := client.Validate()
	if err == nil {
		t.Fatal("an empty configuration should be rejected")
	}

	for _, name := range []string{"Discord__ClientId", "Discord__ClientSecret", "Discord__RedirectUri"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name %s, got: %v", name, err)
		}
	}
}

func TestValidateAcceptsCompleteConfig(t *testing.T) {
	client := NewClient(Config{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost/cb",
	}, "")

	if err := client.Validate(); err != nil {
		t.Errorf("a complete configuration was rejected: %v", err)
	}
}

// strPtr returns a pointer to a string, for building test fixtures.
func strPtr(value string) *string { return &value }
