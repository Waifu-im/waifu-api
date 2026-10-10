package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/config"
)

// contextKey is the private key type used for values stored on a context.
//
// Using an unexported type means no other package can collide with these keys.
type contextKey int

const (
	principalContextKey contextKey = iota
)

// PrincipalFrom returns the authenticated caller, if any.
//
// Returns nil for anonymous requests. Handlers that require authentication check
// for nil and return 401.
func PrincipalFrom(ctx context.Context) *Principal {
	value, _ := ctx.Value(principalContextKey).(*Principal)
	return value
}

// WithPrincipal stores a principal on the context.
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, principal)
}

// Authenticator resolves credentials into a Principal.
type Authenticator struct {
	tokens *TokenIssuer
}

// NewAuthenticator builds an Authenticator.
func NewAuthenticator(tokens *TokenIssuer) *Authenticator {
	return &Authenticator{tokens: tokens}
}

// ErrNoCredentials means the request carried neither a bearer token nor an API
// key.
//
// It is separate from ErrUnauthorized because public endpoints want to treat
// this as "anonymous", not as a failure.
var ErrNoCredentials = errors.New("no credentials supplied")

// Authenticate resolves the caller from the request credentials.
//
// The API key path is tried first when the Authorization header is a bare key,
// which is the format the C# ApiKeyAuthenticationHandler accepts.
func (a *Authenticator) Authenticate(ctx context.Context, authorizationHeader, apiKeyHeader string, lookup UserLookup) (*Principal, error) {
	if token := ExtractBearerToken(authorizationHeader); token != "" {
		claims, err := a.tokens.Verify(token)
		if err != nil {
			return nil, err
		}

		role, err := config.ParseRole(claims.Role)
		if err != nil {
			// An unparseable role in an otherwise valid token means the token
			// was minted by something inconsistent. Reject rather than default,
			// for the same reason configuration does.
			return nil, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}

		return &Principal{
			UserID:    claims.UserID,
			Name:      claims.Name,
			Role:      role,
			DiscordID: claims.DiscordID,
		}, nil
	}

	if apiKeyHeader != "" {
		return a.authenticateAPIKey(ctx, strings.TrimSpace(apiKeyHeader), lookup)
	}

	return nil, ErrNoCredentials
}

// UserLookup resolves an API key hash to its owner.
type UserLookup interface {
	GetUserByAPIKeyHash(ctx context.Context, hash string) (APIKeyOwner, error)
}

// APIKeyOwner is the information needed to build a Principal from an API key.
type APIKeyOwner struct {
	UserID      int64
	Name        string
	DiscordID   string
	Role        int
	Blacklisted bool
}

// authenticateAPIKey resolves a plaintext API key to its owner.
//
// Only a SHA-256 hash of the key is stored, so the plaintext is never
// recoverable. That is why creating a key shows it exactly once.
func (a *Authenticator) authenticateAPIKey(ctx context.Context, plaintext string, lookup UserLookup) (*Principal, error) {
	if lookup == nil {
		return nil, ErrNoCredentials
	}

	owner, err := lookup.GetUserByAPIKeyHash(ctx, HashAPIKey(plaintext))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: unknown API key", ErrUnauthorized)
		}
		return nil, fmt.Errorf("look up API key: %w", err)
	}

	if owner.Blacklisted {
		return nil, fmt.Errorf("%w: this account is blacklisted", ErrForbidden)
	}

	role := config.Role(owner.Role)
	if !role.IsValid() {
		return nil, fmt.Errorf("%w: stored role %d is not recognised", ErrUnauthorized, owner.Role)
	}

	return &Principal{
		UserID:    owner.UserID,
		Name:      owner.Name,
		Role:      role,
		DiscordID: owner.DiscordID,
		ViaAPIKey: true,
	}, nil
}

// HashAPIKey returns the storage form of an API key.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// GenerateAPIKey returns a new plaintext API key.
//
// 32 random bytes gives 256 bits of entropy, which is far beyond what is needed
// for a credential that is stored hashed and rate limited by the caller.
func GenerateAPIKey() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate API key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
