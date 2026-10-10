// Package auth handles authentication and authorisation.
//
// # Two credential types
//
// The API accepts a JWT bearer token or an API key, both of which resolve to the
// same claims. Both C# handlers emit the role as a claim *name* rather than a
// number, so the wire format here is the string "TrustedUser" and not 1.
// Changing that would invalidate every issued credential.
//
// # What is deliberately absent
//
// There is no token revocation list. Revoking a JWT means waiting for it to
// expire, which is why the tokens are short lived. API keys can be deleted,
// which is the escape hatch for a leaked key.
//
// # Blacklisting
//
// Users carry an IsBlacklisted flag that the blacklist middleware enforces on
// every authenticated request. That mirrors BlacklistMiddleware in the C#
// service, which rejects blacklisted users before reaching any handler.
package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"waifuapi/internal/config"
)

// TokenTTL is how long an issued JWT remains valid.
//
// Short by design, since there is no revocation list.
const TokenTTL = 24 * time.Hour

// ErrUnauthorized is returned when a credential is missing, malformed or
// invalid.
var ErrUnauthorized = errors.New("unauthorized")

// ErrForbidden is returned when a credential is valid but lacks permission.
var ErrForbidden = errors.New("forbidden")

// Claims is the JWT payload.
//
// The role is a string name to match the C# TokenService, which writes
// user.Role.ToString() into the claim.
type Claims struct {
	UserID    int64  `json:"sub"`
	Name      string `json:"unique_name"`
	Role      string `json:"role"`
	DiscordID string `json:"discord_id"`

	jwt.RegisteredClaims
}

// RoleName returns the role claim.
func (c *Claims) RoleName() string { return c.Role }

// UserID returns the subject claim.
func (c *Claims) UserIDValue() int64 { return c.UserID }

// TokenIssuer signs and validates JWTs.
type TokenIssuer struct {
	key      []byte
	issuer   string
	audience string
	ttl      time.Duration
}

// NewTokenIssuer builds a TokenIssuer.
func NewTokenIssuer(cfg config.JWTConfig) *TokenIssuer {
	return &TokenIssuer{
		key:      []byte(cfg.Key),
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		ttl:      TokenTTL,
	}
}

// Issue creates a signed token for a user.
//
// The role is written using Role.String so the claim always matches the values
// the C# service emits.
func (t *TokenIssuer) Issue(userID int64, name string, role config.Role, discordID string) (string, time.Time, error) {
	expires := time.Now().Add(t.ttl)

	claims := Claims{
		UserID:    userID,
		Name:      name,
		Role:      role.String(),
		DiscordID: discordID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			Issuer:    t.issuer,
			Audience:  jwt.ClaimStrings{t.audience},
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(t.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, expires, nil
}

// Verify parses and validates a token, returning its claims.
//
// The signing method is pinned to HMAC. Without that check a token could
// present "alg": "none" and be accepted unsigned.
func (t *TokenIssuer) Verify(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
		}
		return t.key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithAudience(t.audience),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// Principal is the authenticated caller, independent of how they authenticated.
//
// Both JWT and API key requests resolve to this, so handlers never branch on
// credential type.
type Principal struct {
	UserID    int64
	Name      string
	Role      config.Role
	DiscordID string

	// ViaAPIKey records that the caller authenticated with an API key rather
	// than a JWT. It is informational; the C# service tracks separate request
	// counters for the two.
	ViaAPIKey bool
}

// CanModerate reports whether the principal may approve or reject tasks.
func (p *Principal) CanModerate() bool {
	return p.Role == config.RoleModerator || p.Role == config.RoleAdmin
}

// IsAdmin reports whether the principal is an administrator.
func (p *Principal) IsAdmin() bool {
	return p.Role == config.RoleAdmin
}

// ExtractBearerToken pulls the token out of an Authorization header.
//
// Returns an empty string when the header is absent, which callers treat as
// anonymous rather than as an error, so public endpoints do not have to
// distinguish "no header" from "bad header".
func ExtractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
