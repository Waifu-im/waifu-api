package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// defaultRequestTimeout bounds how long a handler may run.
//
// It is generous enough for a slow Postgres query but short enough that a stuck
// handler releases its connection instead of pinning it for the life of the
// process.
const defaultRequestTimeout = 30 * time.Second

// withTimeout derives a bounded context from a request.
func withTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

// writeJSONBody serialises a value to the response writer.
func writeJSONBody(w http.ResponseWriter, value any) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(value)
}

// withRequestTimeout returns middleware that applies a deadline to every
// request.
//
// The deadline is only useful if downstream code actually passes the context
// through to the database driver, so every query must accept a *http.Request's
// context rather than using context.Background().
func withRequestTimeout(d time.Duration) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// withRecovery converts a panic into a 500 ProblemDetails response.
//
// Without this a single panic in a handler is caught by the server and closes
// the connection without a body, which the frontend's interceptor cannot parse.
// Recovering keeps the error contract intact for clients even in failure.
func withRecovery(log Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error("panic in handler",
						"path", r.URL.Path,
						"method", r.Method,
						"panic", recovered,
					)
					internalServerError(w, r)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// withLogging records one line per request.
//
// The response status and duration are captured through a thin wrapper rather
// than a dependency, so the logging contract stays visible in this file.
func withLogging(log Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(recorder, r)

			log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		})
	}
}

// withMaxBodySize rejects request bodies above the configured limit.
//
// This mirrors Kestrel's MaxRequestBodySize, which produces the 413 in the C#
// service. Doing it in middleware rather than relying on http.MaxBytesReader in
// each handler means no route can forget it.
func withMaxBodySize(limit int64) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				payloadTooLarge(w, r, limit)
				return
			}
			if r.Body != nil {
				// Wrapping rather than rejecting lets the limit apply even
				// when Content-Length is absent, as with chunked uploads.
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder captures the status code a handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status before delegating.
func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// statusRecorder must preserve Flush and Hijack, which some handlers and
// middleware downstream rely on. Implementing Flush keeps the recorder
// compatible with streaming responses.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Logger is the subset of the standard logger used by this package.
//
// Declaring it here rather than importing slog directly keeps the middleware
// testable with a tiny fake and keeps the package's dependency surface obvious.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}
