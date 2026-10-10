// Package logging configures the process-wide structured logger.
//
// The service uses the standard library's log/slog rather than a third-party
// logger. That keeps the binary small, which matters on a 512 MB free tier,
// and produces JSON directly rather than through a formatter.
//
// Output format is JSON in production so the platform can index it, and human
// readable text in development so it can be read while working locally.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Options controls logger construction.
type Options struct {
	// Environment selects the output format. "development" and "test" produce
	// text; anything else produces JSON.
	Environment string

	// Level is the minimum record level: debug, info, warn or error.
	Level string

	// Writer defaults to os.Stdout when nil. Tests pass a buffer.
	Writer io.Writer
}

// New builds a configured logger.
func New(opts Options) *slog.Logger {
	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}

	level := parseLevel(opts.Level)

	var handler slog.Handler
	options := &slog.HandlerOptions{Level: level}

	if isDevelopment(opts.Environment) {
		handler = slog.NewTextHandler(writer, options)
	} else {
		handler = slog.NewJSONHandler(writer, options)
	}

	return slog.New(handler)
}

// parseLevel maps a configured string onto an slog level.
//
// An unrecognised value yields info rather than an error, because a typo in a
// log level should not stop the service from starting. Log verbosity is not
// worth failing a deployment over.
func parseLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// isDevelopment reports whether human-readable output is appropriate.
func isDevelopment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "development", "dev", "test", "local":
		return true
	default:
		return false
	}
}
