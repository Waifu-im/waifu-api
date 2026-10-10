package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"waifuapi/internal/auth"
	"waifuapi/internal/config"
)

// principal returns the authenticated caller for this request, or nil when the
// request is anonymous.
//
// The value is placed on the request context by the authentication middleware.
// Handlers read it through this method so they never touch the context directly.
func (s *Server) principal(r *http.Request) *principal {
	value, _ := r.Context().Value(principalKey{}).(*principal)
	return value
}

// principalKey is the context key type for the authenticated caller.
type principalKey struct{}

// logError records a handler failure.
//
// Every internal error is logged with the request path so failures can be traced
// back to a route, but the response body stays generic.
func (s *Server) logError(r *http.Request, operation string, err error) {
	if s.log == nil {
		return
	}
	s.log.Error("request failed",
		"operation", operation,
		"method", r.Method,
		"path", r.URL.Path,
		"error", err.Error(),
	)
}

// withAuthentication resolves credentials and stores the caller on the context.
//
// An invalid credential produces 401 rather than being ignored. Treating a bad
// token as anonymous would let a client believe it is unauthenticated and get a
// confusing 403 later instead of a clear 401 here.
func withAuthentication(authenticator *auth.Authenticator, lookup auth.UserLookup, log Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization := r.Header.Get("Authorization")
			apiKey := r.Header.Get("X-Api-Key")

			resolved, err := authenticator.Authenticate(r.Context(), authorization, apiKey, lookup)
			if err == nil {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, toPrincipalView(resolved))))
				return
			}

			if errorsIsNoCredentials(err) {
				// Anonymous is a valid state; public endpoints continue.
				next.ServeHTTP(w, r)
				return
			}

			if log != nil {
				log.Error("authentication failed", "path", r.URL.Path, "error", err.Error())
			}

			writeUnauthorized(w, r)
		})
	}
}

// toPrincipalView narrows an auth.Principal to what handlers use.
func toPrincipalView(p *auth.Principal) *principal {
	if p == nil {
		return nil
	}
	return &principal{
		UserID:    p.UserID,
		Name:      p.Name,
		Role:      roleValue(p.Role),
		ViaAPIKey: p.ViaAPIKey,
	}
}

// ensure slog and time remain referenced.
var (
	_ = slog.Default
	_ = time.Now
	_ = config.RoleUser
)
