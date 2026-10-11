package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"waifuapi/internal/repo"
	"waifuapi/internal/storage"
)

// Helpers used by the upload and review handlers. They are collected here so the
// handlers themselves read as the C# handlers do: a sequence of steps with the
// fiddly string handling out of sight.

// pool exposes the database pool for the few operations that need an explicit
// transaction.
func (s *Server) pool() *pgxpool.Pool { return s.db.Pool() }

// contextWithoutCancel returns a context that survives the request ending.
//
// Cleanup work after a failed request must still run, and a request context is
// cancelled the moment the client disconnects, which would abandon a rollback
// half-done.
func contextWithoutCancel(r *http.Request) context.Context {
	return context.WithoutCancel(r.Context())
}

// optionalFormValue returns a pointer to the first value of a form field, or nil
// when the field is absent.
func optionalFormValue(r *http.Request, name string) *string {
	values := r.Form[name]
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}

// parseFormInt64 reads repeated integer form fields, skipping values that are
// not integers.
func parseFormInt64(values []string) []int64 {
	out := make([]int64, 0, len(values))
	for _, raw := range values {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, parsed)
	}
	return out
}

// uniqueStrings removes duplicates, used when comparing requested slugs against
// the ones resolved from the database.
func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		key := trimLower(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

// uniqueInt64s removes duplicates from an ID list.
func uniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]bool, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// trimLower normalises a value for comparison.
func trimLower(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

// parseHashString converts the hex representation produced by imageproc back
// into a uint64.
//
// imageproc returns the hash as a 16 character hex string, which is what the API
// response and the database column use, so the handler converts at the boundary
// rather than threading a uint64 through.
func parseHashString(value string) uint64 {
	parsed, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// storageKey builds the object storage key for an image.
func storageKey(id int64, extension string) string {
	return storage.ObjectKey(id, extension)
}

// errIsConflict reports whether an error is a uniqueness conflict.
func errIsConflict(err error) bool { return errors.Is(err, repo.ErrConflict) }
