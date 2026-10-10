// Package apierror provides the error type and RFC 7807 serialization that
// every API response failure flows through.
//
// # Why this package exists
//
// The React frontend has a global axios interceptor that parses ProblemDetails
// and renders a toast for every failure. See frontend/src/services/api.ts. If
// the error shape changes, every error message in the UI silently degrades to
// something unreadable, and nothing in backend-only testing would catch it.
//
// The C# service emits these shapes via its exception middleware. This package
// must reproduce them field for field.
package apierror

import (
	"encoding/json"
	"net/http"
)

// Error is an application-level error carrying the HTTP status and the
// ProblemDetails body that will be returned to the client.
//
// Use the constructors below rather than building one directly, so that every
// failure path in the service produces a consistent, documented shape.
type Error struct {
	// Status is the HTTP status code to respond with.
	Status int

	// Title is a short, human-readable summary of the error type. It should
	// not vary between instances of the same error kind, for example
	// "Not Found" or "Conflict".
	Title string

	// Detail explains this specific occurrence. It may vary, and is shown
	// directly to the user, so it must never leak internals such as stack
	// traces, SQL or file paths.
	Detail string

	// Type is a URI reference identifying the problem type. It defaults to a
	// value derived from the status code.
	Type string

	// Instance identifies this specific occurrence, conventionally a URI.
	Instance string

	// Extra carries additional members. The C# service adds entries such as
	// "errors" for validation failures, and the frontend renders them.
	Extra map[string]any
}

// Compile-time assertion that Error satisfies the error interface, so a
// handler can return it directly.
var _ error = (*Error)(nil)

// defaultTypeFor returns the ProblemDetails type URI for a status code.
//
// This mirrors the convention ASP.NET Core uses, which is what the existing
// clients have been built against.
func defaultTypeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.1"
	case http.StatusUnauthorized:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.2"
	case http.StatusForbidden:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.4"
	case http.StatusNotFound:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.5"
	case http.StatusConflict:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.10"
	case http.StatusUnprocessableEntity:
		return "https://tools.ietf.org/html/rfc9110#section-15.5.21"
	default:
		return "about:blank"
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Title
	}
	return e.Title + ": " + e.Detail
}

// New builds an Error, filling in the type URI from the status when not set.
func New(status int, title, detail string) *Error {
	return &Error{
		Status: status,
		Title:  title,
		Detail: detail,
		Type:   defaultTypeFor(status),
	}
}

// WithExtra attaches an additional member to the ProblemDetails body and
// returns the error so it can be used inline.
func (e *Error) WithExtra(key string, value any) *Error {
	if e.Extra == nil {
		e.Extra = make(map[string]any)
	}
	e.Extra[key] = value
	return e
}

// BadRequest reports a malformed or unusable request.
func BadRequest(detail string) *Error {
	return New(http.StatusBadRequest, "Bad Request", detail)
}

// Unauthorized reports a missing or invalid credential.
//
// The C# service uses 401 for unauthenticated access. Note that it does not
// implement a WWW-Authenticate challenge for browser flows, and clients do not
// depend on one.
func Unauthorized(detail string) *Error {
	return New(http.StatusUnauthorized, "Unauthorized", detail)
}

// Forbidden reports an authenticated caller that lacks permission.
//
// The permission text is phrased the same way as the C# service so existing
// users see familiar wording.
func Forbidden(detail string) *Error {
	return New(http.StatusForbidden, "Forbidden", detail)
}

// NotFound reports a missing resource.
func NotFound(detail string) *Error {
	return New(http.StatusNotFound, "Not Found", detail)
}

// Conflict reports a request that clashes with current state, such as the
// duplicate-image rejection at Hamming distance 4.
func Conflict(detail string) *Error {
	return New(http.StatusConflict, "Conflict", detail)
}

// UnprocessableEntity reports a syntactically valid request that failed
// validation.
func UnprocessableEntity(detail string) *Error {
	return New(http.StatusUnprocessableEntity, "Unprocessable Entity", detail)
}

// PayloadTooLarge reports a request body above the configured limit.
//
// The C# service produces this from Kestrel's MaxRequestBodySize, so the
// handler must do the same check to return a ProblemDetails body rather than
// an empty 413.
func PayloadTooLarge(detail string) *Error {
	return New(http.StatusRequestEntityTooLarge, "Payload Too Large", detail)
}

// Internal reports an unexpected server-side failure.
//
// The detail must stay generic. The underlying cause should be logged, not
// returned.
func Internal(detail string) *Error {
	return New(http.StatusInternalServerError, "Internal Server Error", detail)
}

// ProblemDetailsBody is the wire representation of an Error.
//
// Field names are lowerCamelCase to match the .NET System.Text.Json
// camelCase policy the existing clients were built against.
type ProblemDetailsBody struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`

	// Extra members are merged into the top-level object. ProblemDetails
	// explicitly allows this, and the frontend reads the "errors" member for
	// validation failures.
	Extra map[string]any `json:"-"`
}

// Write serialises the error as a ProblemDetails JSON response.
//
// It always writes the Content-Type the frontend interceptor expects, and it
// never leaks the underlying cause.
func (e *Error) Write(w http.ResponseWriter, instance string) {
	body := ProblemDetailsBody{
		Type:     e.Type,
		Title:    e.Title,
		Status:   e.Status,
		Detail:   e.Detail,
		Instance: instance,
		Extra:    e.Extra,
	}

	// Marshal to a map so Extra can be merged at the top level, then re-marshal.
	// This is done per response rather than cached because it is not hot, and
	// clarity is worth more here than avoiding one small allocation.
	raw, err := json.Marshal(body)
	if err != nil {
		// Marshalling our own struct cannot realistically fail, but if it
		// does, fall back to a minimal valid document rather than panicking
		// inside a request.
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Internal Server Error","status":500}`))
		return
	}

	if len(e.Extra) == 0 {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(e.Status)
		_, _ = w.Write(raw)
		return
	}

	// Merge: unmarshal the base object, add extras, marshal again.
	merged := make(map[string]any, len(e.Extra)+6)
	if err := json.Unmarshal(raw, &merged); err != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(e.Status)
		_, _ = w.Write(raw)
		return
	}
	for k, v := range e.Extra {
		merged[k] = v
	}

	final, err := json.Marshal(merged)
	if err != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(e.Status)
		_, _ = w.Write(raw)
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(e.Status)
	_, _ = w.Write(final)
}
