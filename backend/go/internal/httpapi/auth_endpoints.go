package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/auth"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/discord"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// registerAuthRoutes mounts the login and token endpoints.
func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"auth/discord", s.handleDiscordLoginStart)
	mux.HandleFunc("POST "+base+"auth/discord/callback", s.handleDiscordCallback)

	mux.HandleFunc("GET "+base+"users/me", s.handleGetMe)
	mux.HandleFunc("GET "+base+"users/me/permissions", s.handleGetMyPermissions)

	mux.HandleFunc("GET "+base+"api-keys", s.handleListAPIKeys)
	mux.HandleFunc("POST "+base+"api-keys", s.handleCreateAPIKey)
	mux.HandleFunc("DELETE "+base+"api-keys/{id}", s.handleDeleteAPIKey)
}

// discordLoginResponse tells the frontend where to send the browser.
type discordLoginResponse struct {
	// AuthorizeURL is the Discord authorisation endpoint with the configured
	// client id and redirect URI already applied.
	AuthorizeURL string `json:"authorizeUrl"`
}

// handleDiscordLoginStart serves GET /auth/discord.
//
// It returns a URL rather than redirecting, so the frontend controls the
// navigation and can carry its own state.
func (s *Server) handleDiscordLoginStart(w http.ResponseWriter, r *http.Request) {
	if err := s.discord.Validate(); err != nil {
		s.logError(r, "discord login start", err)
		apierror.Internal("Discord login is not configured.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, discordLoginResponse{
		AuthorizeURL: s.discord.AuthorizeURL(r.URL.Query().Get("state")),
	})
}

// discordCallbackRequest is the POST body from the frontend.
//
// The code arrives from the browser rather than being read from the query
// string, because the frontend receives the redirect on its own callback route
// and forwards the value.
type discordCallbackRequest struct {
	// Code is the authorisation code Discord supplied.
	Code string `json:"code"`

	// State echoes the value passed to the start endpoint. It is not verified
	// here because the frontend owns the cookie that does that; verifying it
	// server-side without a session would be theatre.
	State string `json:"state"`
}

// authResponse is returned on successful login.
type authResponse struct {
	Token     string              `json:"token"`
	ExpiresAt string              `json:"expiresAt"`
	User      *models.UserMinimal `json:"user"`
}

// handleDiscordCallback serves POST /auth/discord/callback.
//
// It exchanges the code, fetches the profile, upserts the user and issues a JWT.
// The upsert deliberately does not touch the stored role: promotion is an
// administrative action and re-logging in must not reset it.
func (s *Server) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	if err := s.discord.Validate(); err != nil {
		s.logError(r, "discord callback", err)
		apierror.Internal("Discord login is not configured.").Write(w, r.URL.Path)
		return
	}

	var body discordCallbackRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	if strings.TrimSpace(body.Code) == "" {
		writeAPIError(w, r, apierror.BadRequest("An authorization code is required."))
		return
	}

	ctx := r.Context()

	accessToken, err := s.discord.ExchangeCode(ctx, body.Code)
	if err != nil {
		s.writeDiscordError(w, r, "exchange code", err)
		return
	}

	profile, err := s.discord.Profile(ctx, accessToken)
	if err != nil {
		s.writeDiscordError(w, r, "fetch profile", err)
		return
	}

	avatarURL := profile.AvatarURL(256)

	user, err := s.repo.UpsertUserByDiscordID(ctx, profile.ID, profile.Username, &avatarURL)
	if err != nil {
		s.logError(r, "upsert user", err)
		internalServerError(w, r)
		return
	}

	// A blacklisted account must not receive a token, even though the
	// credentials are valid.
	if user.IsBlacklisted {
		reason := "This account has been suspended."
		if user.BlacklistReason != nil && *user.BlacklistReason != "" {
			reason = *user.BlacklistReason
		}
		apierror.Forbidden(reason).Write(w, r.URL.Path)
		return
	}

	role, err := config.ParseRole(roleNameFor(user.Role))
	if err != nil {
		// An unknown stored role is refused rather than defaulted, for the same
		// reason configuration is.
		s.logError(r, "parse stored role", err)
		internalServerError(w, r)
		return
	}

	token, expires, err := s.tokens.Issue(user.ID, user.Name, role, user.DiscordID)
	if err != nil {
		s.logError(r, "issue token", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token:     token,
		ExpiresAt: expires.Format("2006-01-02T15:04:05.000Z07:00"),
		User: &models.UserMinimal{
			ID:        user.ID,
			Name:      user.Name,
			DiscordID: user.DiscordID,
			AvatarURL: user.AvatarURL,
			Role:      role.String(),
		},
	})
}

// writeDiscordError maps a Discord failure onto an HTTP response.
//
// Discord's own status is passed through when it is a client error, because
// telling the user their authorisation code was rejected is more useful than a
// generic failure. A 5xx from Discord is reported as a 502, since the request
// itself was fine.
func (s *Server) writeDiscordError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	s.logError(r, operation, err)

	var apiErr *discord.APIError
	if ok := asDiscordError(err, &apiErr); ok {
		switch {
		case apiErr.StatusCode >= 400 && apiErr.StatusCode < 500:
			apierror.BadRequest("Discord rejected the login: "+apiErr.Message).
				Write(w, r.URL.Path)
		default:
			apierror.New(http.StatusBadGateway, "Bad Gateway",
				"Discord is currently unavailable.").Write(w, r.URL.Path)
		}
		return
	}
	internalServerError(w, r)
}

// handleGetMe serves GET /users/me.
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	user, err := s.repo.GetUserByID(r.Context(), caller.UserID)
	if err != nil {
		s.logError(r, "get current user", err)
		internalServerError(w, r)
		return
	}

	role, err := config.ParseRole(roleNameFor(user.Role))
	if err != nil {
		s.logError(r, "parse stored role", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, &models.UserMinimal{
		ID:              user.ID,
		Name:            user.Name,
		DiscordID:       user.DiscordID,
		AvatarURL:       user.AvatarURL,
		Role:            role.String(),
		BlacklistReason: user.BlacklistReason,
	})
}

// handleGetMyPermissions serves GET /users/me/permissions.
//
// The frontend calls this to decide whether to show the banner editor. The
// answer is derived from configuration server-side rather than trusting a role
// comparison in the browser, so a changed Permissions__BannerManagementMinRole
// takes effect without a frontend deploy.
func (s *Server) handleGetMyPermissions(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"canManageBanner": s.authz.Allow(authz.ActionManageBanner, roleOf(caller)),
	})
}

// apiKeyDto is the wire form of an API key.
//
// The plaintext is never returned. It is present exactly once, in the response
// to the create call, because only its hash is stored.
type apiKeyDto struct {
	ID             int64   `json:"id"`
	Description    string  `json:"description"`
	CreatedAt      string  `json:"createdAt"`
	LastUsedAt     *string `json:"lastUsedAt,omitempty"`
	ExpirationDate *string `json:"expirationDate,omitempty"`
}

// createAPIKeyResponse includes the one-time plaintext.
type createAPIKeyResponse struct {
	apiKeyDto
	// Key is shown once and cannot be recovered afterwards.
	Key string `json:"key"`
}

// handleListAPIKeys serves GET /api-keys.
func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	keys, err := s.repo.ListAPIKeys(r.Context(), caller.UserID)
	if err != nil {
		s.logError(r, "list api keys", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]apiKeyDto, 0, len(keys))
	for _, key := range keys {
		dtos = append(dtos, toAPIKeyDto(key))
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": dtos})
}

// createAPIKeyRequest is the POST body.
type createAPIKeyRequest struct {
	Description string `json:"description"`

	// ExpiresInDays is optional. Zero means the key does not expire.
	ExpiresInDays int `json:"expiresInDays"`
}

// handleCreateAPIKey serves POST /api-keys.
//
// The plaintext key is generated here and returned once. Only its SHA-256 hash
// is stored, so a lost key cannot be recovered and has to be replaced.
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	var body createAPIKeyRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	description := strings.TrimSpace(body.Description)
	if description == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("A description is required.").
			WithExtra("errors", map[string][]string{
				"Description": {"A description is required."}}))
		return
	}

	// A negative interval is rejected rather than treated as "never", which
	// would silently produce a permanent key from a typo.
	var expiresAt *string
	if body.ExpiresInDays != 0 {
		if body.ExpiresInDays < 0 {
			writeAPIError(w, r, apierror.BadRequest("'expiresInDays' cannot be negative."))
			return
		}
		when := timeNow().AddDate(0, 0, body.ExpiresInDays).UTC()
		formatted := when.Format("2006-01-02T15:04:05Z")
		expiresAt = &formatted
	}

	plaintext, err := auth.GenerateAPIKey()
	if err != nil {
		s.logError(r, "generate api key", err)
		internalServerError(w, r)
		return
	}

	created, err := s.repo.CreateAPIKeyRecord(r.Context(), caller.UserID, description,
		auth.HashAPIKey(plaintext), expiresAt)
	if err != nil {
		s.logError(r, "create api key", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, createAPIKeyResponse{
		apiKeyDto: toAPIKeyRecordDto(created),
		Key:       plaintext,
	})
}

// handleDeleteAPIKey serves DELETE /api-keys/{id}.
//
// Ownership is enforced in the SQL WHERE clause, so a key belonging to someone
// else reports as not found rather than as forbidden, which avoids confirming
// that the id exists.
func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.DeleteAPIKey(r.Context(), id, caller.UserID); err != nil {
		writeRepoError(w, r, "delete api key", err, "API key not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// toAPIKeyDto converts a stored key for the response.
func toAPIKeyDto(key models.APIKey) apiKeyDto {
	dto := apiKeyDto{
		ID:          key.ID,
		Description: key.Description,
		CreatedAt:   key.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if key.LastUsedAt != nil {
		formatted := key.LastUsedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.LastUsedAt = &formatted
	}
	if key.ExpiresAt != nil {
		formatted := key.ExpiresAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.ExpirationDate = &formatted
	}
	return dto
}

// roleNameFor converts a stored integer role into its wire name.
//
// The mapping goes through config.Role so there is one table of role names
// rather than two.
func roleNameFor(value int) string {
	return config.Role(value).String()
}

// ensure the auth package stays referenced.
var _ = auth.TokenTTL

// toAPIKeyRecordDto converts a newly created key for the response.
//
// The record returned by creation carries a *time.Time, while the stored form
// uses formatted strings, so this bridges the two without changing either
// representation.
func toAPIKeyRecordDto(record repo.APIKeyRecord) apiKeyDto {
	dto := apiKeyDto{
		ID:          record.ID,
		Description: record.Description,
		CreatedAt:   record.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
	if record.ExpiresAt != nil {
		formatted := record.ExpiresAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.ExpirationDate = &formatted
	}
	return dto
}
