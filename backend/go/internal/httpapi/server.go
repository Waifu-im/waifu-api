// Package httpapi contains the HTTP layer: routing, middleware and handlers.
//
// Routing uses the standard library's net/http ServeMux, whose pattern syntax
// gained method and wildcards in Go 1.22. That means no third-party router is
// needed, which keeps the binary small and the behaviour predictable.
//
// # Route registration
//
// Routes are declared in one table, Register, so the full surface is visible
// at a glance. Each entry names the controller path from the C# service so the
// two implementations can be compared route by route.
package httpapi

import (
	"context"
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/auth"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/discord"
	"waifuapi/internal/repo"
	"waifuapi/internal/storage"
	"waifuapi/internal/store"
)

// Server holds everything the handlers need.
//
// Dependencies are passed explicitly rather than through a container. At this
// size a plain struct is simpler than any DI framework, and it makes the set of
// dependencies obvious at a glance.
type Server struct {
	cfg     *config.Config
	db      *store.DB
	repo    *repo.Repo
	log     Logger
	authz   *authz.Checker
	storage storage.Store
	discord *discord.Client
	tokens  *auth.TokenIssuer
}

// NewServer builds a Server.
func NewServer(cfg *config.Config, db *store.DB, data *repo.Repo, log Logger, files storage.Store,
	discordClient *discord.Client, tokens *auth.TokenIssuer) *Server {
	return &Server{
		cfg:     cfg,
		db:      db,
		repo:    data,
		log:     log,
		authz:   authz.New(cfg),
		storage: files,
		discord: discordClient,
		tokens:  tokens,
	}
}

// Register mounts every route onto the mux.
//
// The base path from API_BASE_PATH is applied to all routes, mirroring
// app.UsePathBase in the C# service. A mux pattern must start with "/", so an
// empty base path becomes "/".
func (s *Server) Register(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	// Health and diagnostics. Unauthenticated, as in the C# service, because
	// the platform's health checker runs without credentials.
	mux.HandleFunc(base+"healthz", s.handleHealthz)
	mux.HandleFunc(base+"ping", s.handlePing)

	// POST /ping exists in the C# service and is part of the API contract;
	// it answers identically so existing health checks keep working.
	mux.HandleFunc("POST "+base+"ping", s.handlePing)

	s.registerImageRoutes(mux)
	s.registerUploadRoutes(mux)
	s.registerReviewRoutes(mux)
	s.registerCatalogWriteRoutes(mux)
	s.registerAuthRoutes(mux)
	s.registerAlbumRoutes(mux)
	s.registerReportRoutes(mux)
	s.registerUserRoutes(mux)
	s.registerCatalogRoutes(mux)
	s.registerBannerRoutes(mux)
	s.registerStatsRoutes(mux)
}

// handleHealthz reports process and database liveness.
//
// It returns 503 when the database is unreachable so an orchestrator can act
// on it, which is more useful than reporting a healthy process that cannot
// serve traffic.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.withTimeout(r)
	defer cancel()

	if s.db != nil {
		if err := s.db.Ping(ctx); err != nil {
			apierror.Internal("database is unreachable").Write(w, r.URL.Path)
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

// handlePing is the liveness probe.
//
// It deliberately does not touch the database, so a database outage does not
// cause the platform to restart an otherwise healthy process.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

// withTimeout applies a bounded context to inbound requests.
//
// Without this a slow query can hold a handler open indefinitely and exhaust
// the connection pool.
func (s *Server) withTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return withTimeout(r, defaultRequestTimeout)
}

// middleware wraps a handler with cross-cutting concerns.
type middleware func(http.Handler) http.Handler

// chain applies middleware in the order given, so the first listed is the
// outermost and therefore sees the request first.
func chain(handler http.Handler, middlewares ...middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// writeJSON writes a value as a JSON response.
func writeJSON(w http.ResponseWriter, status int, value any) {
	// Indented output is deliberately not used: this is a public API serving
	// mobile and CLI clients, and the byte overhead is real at scale.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if value == nil {
		return
	}
	_ = writeJSONBody(w, value)
}

// Handler wraps the mux with the standard middleware chain and returns the
// result ready to serve.
//
// The order matters and is outermost-first:
//
//	recovery    catches panics so the error contract survives them
//	maxBodySize rejects oversized uploads before they are parsed
//	timeout     bounds handler execution
//	logging     records the outcome, and therefore must run inside recovery
func (s *Server) Handler(mux *http.ServeMux, authenticator *auth.Authenticator, lookup auth.UserLookup) http.Handler {
	return chain(mux,
		withRecovery(s.log),
		withAuthentication(authenticator, lookup, s.log),
		withMaxBodySize(s.cfg.MaxUploadSizeBytes),
		withRequestTimeout(defaultRequestTimeout),
		withLogging(s.log),
	)
}

// pathWithin reports whether a request path sits inside the configured base
// path. Used by middleware that should only apply to API routes.
func pathWithin(path, base string) bool {
	if base == "" || base == "/" {
		return true
	}
	return path == base || strings.HasPrefix(path, base+"/")
}
