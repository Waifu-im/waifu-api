package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"waifuapi/internal/apierror"
)

// principal is the authenticated caller for a request.
//
// The concrete type lives in the auth package; this alias keeps handler files
// from importing it repeatedly.
type principal = principalView

// principalView is the small slice of caller identity handlers need.
//
// Using a narrow view rather than the full auth.Principal keeps the coupling
// from the HTTP layer to the authentication mechanism small, so swapping JWTs
// for something else would not ripple through every handler.
type principalView struct {
	UserID    int64
	Name      string
	Role      roleValue
	ViaAPIKey bool
}

// roleValue is the numeric role, mirroring the persisted integer.
type roleValue = int

// constants mirroring config.Role so handlers read clearly.
const (
	roleUser        roleValue = 0
	roleTrustedUser roleValue = 1
	roleModerator   roleValue = 2
	roleAdmin       roleValue = 3
)

// CanModerate reports whether the caller may moderate.
func (p *principalView) CanModerate() bool {
	return p.Role == roleModerator || p.Role == roleAdmin
}

// IsAdmin reports whether the caller is an admin.
func (p *principalView) IsAdmin() bool {
	return p.Role == roleAdmin
}

// maxJSONBodyBytes bounds request bodies that are decoded into structs.
//
// This is separate from the upload limit because JSON bodies should be far
// smaller; a multi-megabyte JSON body is a mistake or an attack.
const maxJSONBodyBytes = 1 << 20 // 1 MiB

// decodeJSONBody reads and decodes a JSON request body.
//
// An oversized body is rejected with 413 and a malformed one with 400, both as
// ProblemDetails, so the frontend error interceptor handles them the same way it
// handles every other failure.
func decodeJSONBody(r *http.Request, destination any) error {
	if r.Body == nil {
		return apierror.BadRequest("Request body is required.")
	}

	reader := io.LimitReader(r.Body, maxJSONBodyBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return apierror.BadRequest("Could not read the request body.")
	}

	if len(data) > maxJSONBodyBytes {
		return apierror.PayloadTooLarge("Request body is too large.")
	}

	if len(data) == 0 {
		return apierror.BadRequest("Request body is required.")
	}

	if err := json.Unmarshal(data, destination); err != nil {
		return apierror.BadRequest("Request body is not valid JSON.")
	}
	return nil
}

// parsePathInt64 reads an integer path parameter.
func parsePathInt64(r *http.Request, name string) (int64, error) {
	raw := r.PathValue(name)
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, apierror.BadRequest("'" + name + "' must be an integer.")
	}
	return value, nil
}

// itoa64 converts an int64 to a string.
func itoa64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// canModerate reports whether the request is from a moderator or admin.
//
// Returns false for anonymous requests, so public handlers stay public.
func (s *Server) canModerate(r *http.Request) bool {
	p := s.principal(r)
	return p != nil && p.CanModerate()
}

// requireAuth returns the caller or a 401 ProblemDetails.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) *principal {
	p := s.principal(r)
	if p == nil {
		apierror.Unauthorized("Authentication is required.").Write(w, r.URL.Path)
		return nil
	}
	return p
}

// ensure context and errors are referenced so the imports stay meaningful.
var (
	_ = context.Background
	_ = errors.New
	_ = time.Now
)
