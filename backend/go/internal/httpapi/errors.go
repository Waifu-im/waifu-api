package httpapi

import (
	"errors"
	"net/http"

	"waifuapi/internal/apierror"
	"waifuapi/internal/auth"
)

// internalServerError writes a generic 500 ProblemDetails.
//
// The underlying cause is logged rather than returned, so internal detail such
// as SQL, file paths or driver versions never reaches a client.
func internalServerError(w http.ResponseWriter, r *http.Request) {
	apierror.Internal("An unexpected error occurred.").Write(w, r.URL.Path)
}

// payloadTooLarge writes a 413 ProblemDetails.
func payloadTooLarge(w http.ResponseWriter, r *http.Request, limit int64) {
	limitMB := limit / (1024 * 1024)
	apierror.PayloadTooLarge(
		"Request body exceeds the maximum allowed size of "+itoa(limitMB)+" MB.",
	).Write(w, r.URL.Path)
}

// itoa converts an int64 to a string without pulling in strconv at this call
// site. It exists because the value is always non-negative here, and a negative
// limit would be a configuration bug already rejected at startup.
func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// errorsIsNoCredentials reports whether an authentication error means the
// request simply carried no credentials, which is a valid anonymous state.
func errorsIsNoCredentials(err error) bool {
	return errors.Is(err, auth.ErrNoCredentials)
}

// writeUnauthorized sends a 401 ProblemDetails.
func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	apierror.Unauthorized("Authentication is required.").Write(w, r.URL.Path)
}
