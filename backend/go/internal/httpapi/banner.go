package httpapi

import (
	"net/http"
	"strings"
	"time"

	"waifuapi/internal/authz"
	"waifuapi/internal/store/models"
)

// registerBannerRoutes mounts the banner endpoints.
//
// GET /banner is public — every visitor sees it. The two settings routes require
// the configured Permissions__BannerManagementMinRole, which defaults to
// Moderator. They are not hardcoded to Moderator the way the review queue is:
// an instance owner can hand banner management to a trusted user without also
// handing over approval rights.
func (s *Server) registerBannerRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"banner", s.handleGetBanner)
	mux.HandleFunc("GET "+base+"banner/settings", s.handleGetBannerSettings)
	mux.HandleFunc("PUT "+base+"banner/settings", s.handlePutBannerSettings)
}

// bannerDto is the banner shown to visitors.
type bannerDto struct {
	Message   string    `json:"message"`
	Variant   string    `json:"variant"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// handleGetBanner serves GET /banner.
//
// It answers 204 with no body when there is no active banner. That exact status
// is load-bearing: useBanner.ts treats 204 as "no banner", and a 200 with an
// empty object would render an empty banner box instead.
func (s *Server) handleGetBanner(w http.ResponseWriter, r *http.Request) {
	banner, err := s.repo.GetActiveBanner(r.Context())
	if err != nil {
		s.logError(r, "get banner", err)
		internalServerError(w, r)
		return
	}

	if banner == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	writeJSON(w, http.StatusOK, bannerDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		UpdatedAt: banner.UpdatedAt,
	})
}

// bannerSettingsDto is the editable banner configuration.
type bannerSettingsDto struct {
	Message   string          `json:"message"`
	Variant   string          `json:"variant"`
	IsEnabled bool            `json:"isEnabled"`
	UpdatedAt *time.Time      `json:"updatedAt"`
	UpdatedBy *userMinimalDto `json:"updatedBy"`
}

// handleGetBannerSettings serves GET /banner/settings.
func (s *Server) handleGetBannerSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionManageBanner) {
		return
	}

	banner, editor, err := s.repo.GetBannerSettings(r.Context())
	if err != nil {
		s.logError(r, "get banner settings", err)
		internalServerError(w, r)
		return
	}

	// A banner that has never been configured returns defaults rather than 404,
	// so the settings page always has something to render.
	if banner.ID == 0 {
		writeJSON(w, http.StatusOK, bannerSettingsDto{
			Variant: models.VariantInfo.String(),
		})
		return
	}

	updatedAt := banner.UpdatedAt
	dto := bannerSettingsDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		IsEnabled: banner.IsEnabled,
		UpdatedAt: &updatedAt,
	}
	if editor != nil {
		dto.UpdatedBy = &userMinimalDto{ID: editor.ID, Name: editor.Name, AvatarURL: editor.AvatarURL}
	}

	writeJSON(w, http.StatusOK, dto)
}

// updateBannerRequest is the PUT /banner/settings body.
type updateBannerRequest struct {
	Message   string `json:"message"`
	Variant   string `json:"variant"`
	IsEnabled bool   `json:"isEnabled"`
}

// maxBannerMessageLength matches the UpdateBanner validator.
const maxBannerMessageLength = 500

// handlePutBannerSettings serves PUT /banner/settings.
func (s *Server) handlePutBannerSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionManageBanner) {
		return
	}
	caller := s.principal(r)

	var body updateBannerRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	variant, knownVariant := models.ParseBannerVariant(body.Variant)

	// The rules below mirror UpdateBannerCommandValidator. The NotNull rule on
	// Message is deliberately absent: the controller coalesces a missing message
	// to string.Empty before building the command, so that rule can never fire in
	// the C# service and reproducing it here would only add a failure mode.
	failures := map[string]string{}
	if len([]rune(body.Message)) > maxBannerMessageLength {
		failures["Message"] = "Message must not exceed 500 characters."
	}
	if body.IsEnabled && strings.TrimSpace(body.Message) == "" {
		failures["Message"] = "An enabled banner needs a message."
	}
	if !knownVariant {
		failures["Variant"] = "Invalid banner variant."
	}
	if len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	banner, editor, err := s.repo.SaveBanner(r.Context(), body.Message,
		variant, body.IsEnabled, &caller.UserID)
	if err != nil {
		s.logError(r, "save banner", err)
		internalServerError(w, r)
		return
	}

	updatedAt := banner.UpdatedAt
	dto := bannerSettingsDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		IsEnabled: banner.IsEnabled,
		UpdatedAt: &updatedAt,
	}
	if editor != nil {
		dto.UpdatedBy = &userMinimalDto{ID: editor.ID, Name: editor.Name, AvatarURL: editor.AvatarURL}
	}

	writeJSON(w, http.StatusOK, dto)
}

// registerStatsRoutes mounts the public statistics endpoint.
func (s *Server) registerStatsRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()
	mux.HandleFunc("GET "+base+"stats/public", s.handlePublicStats)
}

// handlePublicStats serves GET /stats/public.
//
// Unauthenticated, like every other read endpoint. The counts exclude pending
// content, so the public figures only ever describe what a visitor can see.
func (s *Server) handlePublicStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.repo.GetPublicStats(r.Context())
	if err != nil {
		s.logError(r, "public stats", err)
		internalServerError(w, r)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
