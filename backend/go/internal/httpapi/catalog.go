package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/apierror"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// TagDto is the public representation of a tag.
type TagDto struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// ArtistDto is the public representation of an artist.
type ArtistDto struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	DeviantArt *string `json:"deviantArt,omitempty"`
	Twitter    *string `json:"twitter,omitempty"`
	Patreon    *string `json:"patreon,omitempty"`
	Pixiv      *string `json:"pixiv,omitempty"`
}

// registerCatalogRoutes mounts the tag and artist endpoints.
func (s *Server) registerCatalogRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"tags", s.handleListTags)
	mux.HandleFunc("GET "+base+"tags/{id}", s.handleGetTag)
	mux.HandleFunc("GET "+base+"artists", s.handleListArtists)
	mux.HandleFunc("GET "+base+"artists/{id}", s.handleGetArtist)
}

// handleListTags serves GET /tags.
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	search := r.URL.Query().Get("name")

	tags, total, err := s.repo.ListTags(r.Context(), search, page.offset(), page.PageSize)
	if err != nil {
		s.logError(r, "list tags", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]TagDto, 0, len(tags))
	for _, tag := range tags {
		dtos = append(dtos, TagDto{
			ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
}

// handleGetTag serves GET /tags/{id}.
func (s *Server) handleGetTag(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	tag, err := s.repo.GetTagByID(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			apierror.NotFound("Tag with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
			return
		}
		s.logError(r, "get tag", err)
		internalServerError(w, r)
		return
	}

	// Pending tags are visible only to moderators, matching the accepted-only
	// default filter on the list endpoint.
	if tag.ReviewStatus == models.ReviewPending && !s.canModerate(r) {
		apierror.NotFound("Tag with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, TagDto{
		ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
	})
}

// handleListArtists serves GET /artists.
func (s *Server) handleListArtists(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	search := r.URL.Query().Get("name")

	artists, total, err := s.repo.ListArtists(r.Context(), search, page.offset(), page.PageSize)
	if err != nil {
		s.logError(r, "list artists", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]ArtistDto, 0, len(artists))
	for _, artist := range artists {
		dtos = append(dtos, toArtistDto(artist))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
}

// handleGetArtist serves GET /artists/{id}.
func (s *Server) handleGetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	artist, err := s.repo.GetArtistByID(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			apierror.NotFound("Artist with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
			return
		}
		s.logError(r, "get artist", err)
		internalServerError(w, r)
		return
	}

	if artist.ReviewStatus == models.ReviewPending && !s.canModerate(r) {
		apierror.NotFound("Artist with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, toArtistDto(artist))
}

func toArtistDto(artist models.Artist) ArtistDto {
	return ArtistDto{
		ID: artist.ID, Name: artist.Name,
		DeviantArt: artist.DeviantArt, Twitter: artist.Twitter,
		Patreon: artist.Patreon, Pixiv: artist.Pixiv,
	}
}

// registerBannerRoutes mounts the banner endpoints.
func (s *Server) registerBannerRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"banner", s.handleGetBanner)
	mux.HandleFunc("GET "+base+"banner/settings", s.handleGetBannerSettings)
	mux.HandleFunc("PUT "+base+"banner/settings", s.handlePutBannerSettings)
}

// BannerDto is the banner shown to visitors.
type BannerDto struct {
	Message   string    `json:"message"`
	Variant   string    `json:"variant"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// handleGetBanner serves GET /banner.
//
// It answers 204 with no body when there is no active banner. That exact status
// is part of the contract: the frontend hook checks for it, and returning 200
// with an empty object instead would show an empty banner box.
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

	writeJSON(w, http.StatusOK, BannerDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		UpdatedAt: banner.UpdatedAt,
	})
}

// BannerSettingsDto is the editable banner configuration.
type BannerSettingsDto struct {
	Message   string              `json:"message"`
	Variant   string              `json:"variant"`
	IsEnabled bool                `json:"isEnabled"`
	UpdatedAt *time.Time          `json:"updatedAt,omitempty"`
	UpdatedBy *models.UserMinimal `json:"updatedBy,omitempty"`
}

// handleGetBannerSettings serves GET /banner/settings.
func (s *Server) handleGetBannerSettings(w http.ResponseWriter, r *http.Request) {
	if !s.canModerate(r) {
		apierror.Forbidden("This action requires the Moderator role or higher.").Write(w, r.URL.Path)
		return
	}

	banner, err := s.repo.GetBannerSettings(r.Context())
	if err != nil {
		s.logError(r, "get banner settings", err)
		internalServerError(w, r)
		return
	}

	if banner == nil {
		writeJSON(w, http.StatusOK, BannerSettingsDto{
			Variant: models.VariantInfo.String(),
		})
		return
	}

	updatedAt := banner.UpdatedAt
	writeJSON(w, http.StatusOK, BannerSettingsDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		IsEnabled: banner.IsEnabled,
		UpdatedAt: &updatedAt,
	})
}

// updateBannerRequest is the PUT /banner/settings body.
type updateBannerRequest struct {
	Message   string `json:"message"`
	Variant   string `json:"variant"`
	IsEnabled bool   `json:"isEnabled"`
}

// maxBannerMessageLength matches the C# validator.
const maxBannerMessageLength = 500

// handlePutBannerSettings serves PUT /banner/settings.
func (s *Server) handlePutBannerSettings(w http.ResponseWriter, r *http.Request) {
	if !s.canModerate(r) {
		apierror.Forbidden("This action requires the Moderator role or higher.").Write(w, r.URL.Path)
		return
	}

	var body updateBannerRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	if len(body.Message) > maxBannerMessageLength {
		writeAPIError(w, r, apierror.BadRequest(
			"Message must not exceed "+strconv.Itoa(maxBannerMessageLength)+" characters."))
		return
	}

	var editorID *int64
	if p := s.principal(r); p != nil {
		editorID = &p.UserID
	}

	banner, err := s.repo.SaveBanner(r.Context(), body.Message,
		models.ParseBannerVariant(body.Variant), body.IsEnabled, editorID)
	if err != nil {
		s.logError(r, "save banner", err)
		internalServerError(w, r)
		return
	}

	updatedAt := banner.UpdatedAt
	writeJSON(w, http.StatusOK, BannerSettingsDto{
		Message:   banner.Message,
		Variant:   banner.Variant.String(),
		IsEnabled: banner.IsEnabled,
		UpdatedAt: &updatedAt,
	})
}

// registerStatsRoutes mounts the public statistics endpoint.
func (s *Server) registerStatsRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()
	mux.HandleFunc("GET "+base+"stats/public", s.handlePublicStats)
}

// handlePublicStats serves GET /stats/public.
func (s *Server) handlePublicStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.repo.GetPublicStats(r.Context())
	if err != nil {
		s.logError(r, "public stats", err)
		internalServerError(w, r)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ensure the context import is used.
var _ = context.Background
var _ = repo.ImageFilter{}
