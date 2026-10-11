package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

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

// requireRole enforces a fixed minimum role.
//
// This is used where the C# service applies an authorization *policy* rather
// than a configured setting: approving review tasks and editing or deleting
// catalogue entries are moderator- and admin-only regardless of configuration,
// so no environment value can lower them.
//
// It returns false and has already written the response when refused.
func (s *Server) requireRole(w http.ResponseWriter, r *http.Request, minimum config.Role, action string) bool {
	p := s.requireAuth(w, r)
	if p == nil {
		return false
	}
	if err := s.authz.RequireAtLeast(roleOf(p), minimum); err != nil {
		writeForbidden(w, r, err)
		return false
	}
	return true
}

// isNoRows reports whether an error is the driver's "no such row" sentinel.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
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

// slugify mirrors StringExtensions.ToSlug in the C# application layer.
//
// The order of operations matters, and the intermediate steps are not
// interchangeable with the obvious "replace every run of junk with a dash"
// shortcut:
//
//  1. Decompose to NFD, drop combining marks, recompose to NFC. This strips
//     accents, so "Éclair" slugs to "eclair" instead of losing its first letter.
//  2. Lowercase.
//  3. Delete every character that is not a letter, digit, whitespace or hyphen.
//     Punctuation is DELETED, not turned into a separator, so "waifu!!!girl"
//     slugs to "waifugirl" and not "waifu-girl".
//  4. Collapse each whitespace run to a single hyphen. Runs of hyphens that were
//     already in the input are left alone, so "wi--fi" stays "wi--fi".
//  5. Trim hyphens from both ends.
//
// Slugs are public API surface: /tags/by-slug/{slug} and ?includedTags= both
// key off them, and stored image tags were written using this rule. A server
// that computed a different slug would make those rows unaddressable, so this
// has to match the C# byte for byte rather than merely producing "a nice slug".
func slugify(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range strings.ToLower(norm.NFC.String(stripCombiningMarks(text))) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			builder.WriteRune(r)
		case isDotNetSpace(r):
			builder.WriteRune(r)
		}
		// Everything else, punctuation and symbols alike, is dropped.
	}

	// Collapse whitespace runs to one hyphen, leaving hyphens untouched.
	var slug strings.Builder
	inSpace := false
	for _, r := range builder.String() {
		if isDotNetSpace(r) {
			if !inSpace {
				slug.WriteByte('-')
				inSpace = true
			}
			continue
		}
		slug.WriteRune(r)
		inSpace = false
	}

	return strings.Trim(slug.String(), "-")
}

// stripCombiningMarks decomposes text and removes the accents that NFD split
// off, which is how "é" becomes "e" rather than "e" plus a mark that the
// character filter would delete.
func stripCombiningMarks(text string) string {
	var builder strings.Builder
	for _, r := range norm.NFD.String(text) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// isDotNetSpace reports whether r belongs to .NET's \s class, [\f\n\r\t\v\x85\p{Z}].
//
// Go's regexp \s is ASCII-only, so it is not usable for the filter and
// replacement above: .NET keeps a non-breaking space in "waifu girl"
// and then turns it into a hyphen, where the ASCII class would have deleted it
// outright and produced "waifugirl".
func isDotNetSpace(r rune) bool {
	switch r {
	case '\f', '\n', '\r', '\t', '\v', '\u0085':
		return true
	}
	return unicode.In(r, unicode.Zs, unicode.Zl, unicode.Zp)
}

// reviewStatusFor maps a review decision onto the stored status.
func reviewStatusFor(requiresReview bool) models.ReviewStatus {
	if requiresReview {
		return models.ReviewPending
	}
	return models.ReviewAccepted
}
