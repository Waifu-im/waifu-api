package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// roleOf converts the HTTP layer's numeric role back into the typed one, so
// authorisation decisions are made against config.Role rather than a bare int.
func roleOf(p *principal) config.Role {
	if p == nil {
		// Anonymous is treated as the lowest role. Anonymous never reaches a
		// guarded handler because requireAuth runs first, but the default keeps
		// the behaviour safe if that ordering changes.
		return config.RoleUser
	}
	return config.Role(p.Role)
}

// writeForbidden renders an authorisation refusal.
//
// The wording is produced by the authz package so it matches the C# service.
func writeForbidden(w http.ResponseWriter, r *http.Request, err error) {
	var forbidden *authz.ErrForbidden
	if errors.As(err, &forbidden) {
		apierror.Forbidden(forbidden.Error()).Write(w, r.URL.Path)
		return
	}
	apierror.Forbidden("You do not have permission to perform this action.").Write(w, r.URL.Path)
}

// requirePermission enforces a configured minimum role.
//
// It returns false and has already written the response when the caller is not
// permitted, so handlers read as a sequence of guards.
func (s *Server) requirePermission(w http.ResponseWriter, r *http.Request, action authz.Action) bool {
	p := s.requireAuth(w, r)
	if p == nil {
		return false
	}
	if err := s.authz.Require(action, roleOf(p)); err != nil {
		writeForbidden(w, r, err)
		return false
	}
	return true
}

// writeRepoError maps a repository error onto an HTTP response.
//
// This keeps handlers from repeating the pgx.ErrNoRows check and stops internal
// driver detail from reaching a client.
func writeRepoError(w http.ResponseWriter, r *http.Request, operation string, err error, notFoundDetail string) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		apierror.NotFound(notFoundDetail).Write(w, r.URL.Path)
	case errors.Is(err, repo.ErrConflict):
		apierror.Conflict(err.Error()).Write(w, r.URL.Path)
	case errors.Is(err, repo.ErrNotFound):
		apierror.NotFound(notFoundDetail).Write(w, r.URL.Path)
	default:
		internalServerError(w, r)
	}
}

// slugify converts a tag name into a URL slug.
//
// The C# frontend calls this before sending an upload, so the server must apply
// the same rule or a tag would be looked up under a different key than the
// client expects.
func slugify(name string) string {
	var builder strings.Builder
	lastDash := false

	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastDash = false
		default:
			// Collapse every run of non-alphanumeric characters into one dash.
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

// reviewStatusFor maps a review decision onto the stored status.
func reviewStatusFor(requiresReview bool) models.ReviewStatus {
	if requiresReview {
		return models.ReviewPending
	}
	return models.ReviewAccepted
}
