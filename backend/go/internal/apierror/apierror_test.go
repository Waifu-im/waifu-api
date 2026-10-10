package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestProblemDetailsShape pins the wire format.
//
// The frontend's axios interceptor (frontend/src/services/api.ts) parses this
// document. If the shape drifts, every error toast in the UI degrades silently
// and no backend test would notice.
func TestProblemDetailsShape(t *testing.T) {
	cases := []struct {
		name       string
		err        *Error
		wantStatus int
		wantTitle  string
	}{
		{"not found", NotFound("Image with ID 5 not found."), 404, "Not Found"},
		{"conflict", Conflict("Duplicate image found. ID: 12"), 409, "Conflict"},
		{"forbidden", Forbidden("This action requires the Moderator role or higher."), 403, "Forbidden"},
		{"unauthorized", Unauthorized("Authentication required."), 401, "Unauthorized"},
		{"bad request", BadRequest("Malformed query."), 400, "Bad Request"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c.err.Write(recorder, "/images/5")

			if recorder.Code != c.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, c.wantStatus)
			}

			contentType := recorder.Header().Get("Content-Type")
			if contentType != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", contentType)
			}

			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not valid JSON: %v\n%s", err, recorder.Body.String())
			}

			if got := body["title"]; got != c.wantTitle {
				t.Errorf("title = %v, want %q", got, c.wantTitle)
			}
			if got := body["status"]; got != float64(c.wantStatus) {
				t.Errorf("status member = %v, want %d", got, c.wantStatus)
			}
			if _, ok := body["type"]; !ok {
				t.Error("ProblemDetails must include a type member")
			}
			if got := body["instance"]; got != "/images/5" {
				t.Errorf("instance = %v, want /images/5", got)
			}
		})
	}
}

// TestExtraMembersAreMergedAtTopLevel checks validation errors, which the C#
// service returns under an "errors" member that the frontend reads.
func TestExtraMembersAreMergedAtTopLevel(t *testing.T) {
	err := UnprocessableEntity("One or more validation errors occurred.").
		WithExtra("errors", map[string][]string{
			"File": {"File is required."},
		})

	recorder := httptest.NewRecorder()
	err.Write(recorder, "/images/upload")

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	errorsMember, ok := body["errors"]
	if !ok {
		t.Fatal("errors member is missing")
	}

	errorsMap, ok := errorsMember.(map[string]any)
	if !ok {
		t.Fatalf("errors member has type %T, want object", errorsMember)
	}
	if _, ok := errorsMap["File"]; !ok {
		t.Errorf("errors.File missing, got %v", errorsMap)
	}
}

// TestInternalErrorDoesNotLeakInternals guards the boundary between what is
// logged and what is returned.
func TestInternalErrorDoesNotLeakInternals(t *testing.T) {
	recorder := httptest.NewRecorder()
	Internal("An unexpected error occurred.").Write(recorder, "/images")

	body := recorder.Body.String()
	for _, forbidden := range []string{"sql", "SELECT", "goroutine", ".go:", "/var/"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Errorf("internal error body leaks %q: %s", forbidden, body)
		}
	}
}

// TestErrorInterface covers the default Error method.
func TestErrorInterface(t *testing.T) {
	e := NotFound("Image with ID 5 not found.")
	message := e.Error()
	if !strings.Contains(message, "Not Found") || !strings.Contains(message, "ID 5") {
		t.Errorf("Error() = %q, want title and detail", message)
	}

	if bare := (&Error{Title: "Conflict"}).Error(); bare != "Conflict" {
		t.Errorf("Error() with no detail = %q, want \"Conflict\"", bare)
	}
}

// TestDefaultTypeForStatus pins the type URIs to the ASP.NET Core convention
// the existing clients were built against.
func TestDefaultTypeForStatus(t *testing.T) {
	if got := defaultTypeFor(http.StatusNotFound); !strings.Contains(got, "rfc9110") {
		t.Errorf("404 type = %q, want an RFC 9110 reference", got)
	}
	if got := defaultTypeFor(http.StatusTeapot); got != "about:blank" {
		t.Errorf("unknown status type = %q, want about:blank", got)
	}
}
