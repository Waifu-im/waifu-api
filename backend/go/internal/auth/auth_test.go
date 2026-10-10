package auth

import (
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// testIssuer builds a TokenIssuer with known values.
func testIssuer() *TokenIssuer {
	return NewTokenIssuer(config.JWTConfig{
		Key:      strings.Repeat("k", 40),
		Issuer:   "waifu-api",
		Audience: "waifu-api-users",
	})
}

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	issuer := testIssuer()

	token, expires, err := issuer.Issue(42, "tester", config.RoleTrustedUser, "discord-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if !expires.After(nowForTest()) {
		t.Errorf("token should expire in the future, got %v", expires)
	}

	claims, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("user id = %d, want 42", claims.UserID)
	}
	if claims.Name != "tester" {
		t.Errorf("name = %q, want tester", claims.Name)
	}
	if claims.DiscordID != "discord-1" {
		t.Errorf("discord id = %q, want discord-1", claims.DiscordID)
	}
}

// TestRoleClaimIsTheRoleName guards the wire contract.
//
// The C# TokenService writes user.Role.ToString() into the claim, so a numeric
// role here would invalidate every token the old service issued.
func TestRoleClaimIsTheRoleName(t *testing.T) {
	issuer := testIssuer()

	for _, role := range []config.Role{
		config.RoleUser, config.RoleTrustedUser, config.RoleModerator, config.RoleAdmin,
	} {
		token, _, err := issuer.Issue(1, "u", role, "d")
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		claims, err := issuer.Verify(token)
		if err != nil {
			t.Fatalf("Verify: %v", err)
		}
		if claims.Role != role.String() {
			t.Errorf("role claim = %q, want %q", claims.Role, role.String())
		}
	}
}

func TestVerifyRejectsWrongSigningKey(t *testing.T) {
	token, _, err := testIssuer().Issue(1, "u", config.RoleUser, "d")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	other := NewTokenIssuer(config.JWTConfig{
		Key:      strings.Repeat("x", 40),
		Issuer:   "waifu-api",
		Audience: "waifu-api-users",
	})
	if _, err := other.Verify(token); err == nil {
		t.Error("a token signed with a different key must be rejected")
	}
}

func TestVerifyRejectsWrongIssuerAndAudience(t *testing.T) {
	token, _, err := testIssuer().Issue(1, "u", config.RoleUser, "d")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	wrongIssuer := NewTokenIssuer(config.JWTConfig{
		Key: strings.Repeat("k", 40), Issuer: "someone-else", Audience: "waifu-api-users",
	})
	if _, err := wrongIssuer.Verify(token); err == nil {
		t.Error("a token with a different issuer must be rejected")
	}

	wrongAudience := NewTokenIssuer(config.JWTConfig{
		Key: strings.Repeat("k", 40), Issuer: "waifu-api", Audience: "another-app",
	})
	if _, err := wrongAudience.Verify(token); err == nil {
		t.Error("a token with a different audience must be rejected")
	}
}

// TestVerifyRejectsAlgNone is the classic JWT vulnerability.
//
// Without pinning the signing method, a token declaring "alg": "none" could be
// accepted unsigned.
func TestVerifyRejectsAlgNone(t *testing.T) {
	// Header {"alg":"none","typ":"JWT"}, payload {"sub":"1","role":"Admin"}.
	const unsigned = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiIxIiwicm9sZSI6IkFkbWluIn0."

	if _, err := testIssuer().Verify(unsigned); err == nil {
		t.Error("an unsigned token must be rejected")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	issuer := NewTokenIssuer(config.JWTConfig{
		Key: strings.Repeat("k", 40), Issuer: "waifu-api", Audience: "waifu-api-users",
	})
	// An already-expired TTL, to avoid sleeping in the test.
	issuer.ttl = -1

	token, _, err := issuer.Issue(1, "u", config.RoleUser, "d")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := issuer.Verify(token); err == nil {
		t.Error("an expired token must be rejected")
	}
}

func TestExtractBearerToken(t *testing.T) {
	cases := map[string]string{
		"":                        "",
		"Bearer abc":              "abc",
		"bearer abc":              "abc",
		"BEARER abc":              "abc",
		"Bearer   spaced   ":      "spaced",
		"Basic abc":               "",
		"abc":                     "",
		"Bearer":                  "",
		"api-key-value-no-scheme": "",
	}
	for header, want := range cases {
		if got := ExtractBearerToken(header); got != want {
			t.Errorf("ExtractBearerToken(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestPrincipalModeration(t *testing.T) {
	cases := map[config.Role]bool{
		config.RoleUser:        false,
		config.RoleTrustedUser: false,
		config.RoleModerator:   true,
		config.RoleAdmin:       true,
	}
	for role, want := range cases {
		p := &Principal{Role: role}
		if got := p.CanModerate(); got != want {
			t.Errorf("%v CanModerate() = %v, want %v", role, got, want)
		}
	}

	if (&Principal{Role: config.RoleAdmin}).IsAdmin() != true {
		t.Error("Admin should report IsAdmin")
	}
	if (&Principal{Role: config.RoleModerator}).IsAdmin() != false {
		t.Error("Moderator must not report IsAdmin")
	}
}

func TestAPIKeyHashingIsDeterministicAndOpaque(t *testing.T) {
	// The same key must always hash the same way, or API key auth would break.
	first := HashAPIKey("secret-key")
	second := HashAPIKey("secret-key")
	if first != second {
		t.Errorf("hash is not deterministic: %s vs %s", first, second)
	}
	if first == "secret-key" {
		t.Error("the stored form must not be the plaintext")
	}
	if len(first) != 64 {
		t.Errorf("SHA-256 hex should be 64 characters, got %d", len(first))
	}

	if HashAPIKey("different") == first {
		t.Error("different keys must hash differently")
	}
}

func TestGenerateAPIKeyIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey: %v", err)
		}
		if len(key) < 40 {
			t.Fatalf("generated key is too short: %d chars", len(key))
		}
		if seen[key] {
			t.Fatal("generated a duplicate API key")
		}
		seen[key] = true
	}
}

// nowForTest returns the current time, isolated so tests can stub it.
func nowForTest() time.Time { return time.Now() }
