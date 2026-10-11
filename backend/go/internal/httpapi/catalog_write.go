package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// registerCatalogWriteRoutes mounts the tag and artist creation endpoints.
//
// Creating tags and artists is a separate permission from uploading images, and
// each goes through its own review queue. An image upload only attaches
// *existing* tags and artists, which is why these have to exist first.
func (s *Server) registerCatalogWriteRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("POST "+base+"tags", s.handleCreateTag)
	mux.HandleFunc("POST "+base+"artists", s.handleCreateArtist)
}

// createTagRequest is the body of POST /tags.
type createTagRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// createTagResponse is the body returned by POST /tags.
type createTagResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	ReviewStatus string `json:"reviewStatus"`
	Message      string `json:"message"`
}

// handleCreateTag serves POST /tags.
func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionCreateTag) {
		return
	}
	caller := s.principal(r)

	var body createTagRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("Name is required.").
			WithExtra("errors", map[string][]string{"Name": {"Name is required."}}))
		return
	}

	slug := slugify(name)
	if slug == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity(
			"Name must contain at least one letter or digit.").
			WithExtra("errors", map[string][]string{"Name": {"Name must contain at least one letter or digit."}}))
		return
	}

	requiresReview := s.authz.RequiresReviewForTag(roleOf(caller))

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin create tag", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	tag, err := s.repo.CreateTag(r.Context(), name, slug, body.Description,
		&caller.UserID, reviewStatusFor(requiresReview))
	if err != nil {
		// A duplicate slug is the common case, since Slug carries a unique
		// constraint. The driver error is not surfaced directly, so it is
		// reported as a conflict with a message the user can act on.
		if errIsConflict(err) {
			apierror.Conflict("A tag with this slug already exists.").Write(w, r.URL.Path)
			return
		}
		s.logError(r, "create tag", err)
		internalServerError(w, r)
		return
	}

	if _, err := s.repo.CreateReviewTask(r.Context(), tx,
		models.KindNewContent, models.TargetTag, tag.ID, &caller.UserID,
		nil, requiresReview); err != nil {
		s.logError(r, "insert tag review task", err)
		internalServerError(w, r)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit create tag", err)
		internalServerError(w, r)
		return
	}

	message := "Tag created."
	if requiresReview {
		message = "Tag pending review."
	}
	writeJSON(w, http.StatusOK, createTagResponse{
		ID: tag.ID, Name: tag.Name, Slug: tag.Slug,
		Description:  tag.Description,
		ReviewStatus: tag.ReviewStatus.String(),
		Message:      message,
	})
}

// createArtistRequest is the body of POST /artists.
type createArtistRequest struct {
	Name       string `json:"name"`
	DeviantArt string `json:"deviantArt"`
	Twitter    string `json:"twitter"`
	Patreon    string `json:"patreon"`
	Pixiv      string `json:"pixiv"`
}

// handleCreateArtist serves POST /artists.
func (s *Server) handleCreateArtist(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionCreateArtist) {
		return
	}
	caller := s.principal(r)

	var body createArtistRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("Name is required.").
			WithExtra("errors", map[string][]string{"Name": {"Name is required."}}))
		return
	}

	requiresReview := s.authz.RequiresReviewForArtist(roleOf(caller))

	artist := models.Artist{
		Name:         name,
		DeviantArt:   optionalString(body.DeviantArt),
		Twitter:      optionalString(body.Twitter),
		Patreon:      optionalString(body.Patreon),
		Pixiv:        optionalString(body.Pixiv),
		CreatorID:    &caller.UserID,
		ReviewStatus: reviewStatusFor(requiresReview),
	}

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin create artist", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	created, err := s.repo.CreateArtist(r.Context(), artist)
	if err != nil {
		s.logError(r, "create artist", err)
		internalServerError(w, r)
		return
	}

	if _, err := s.repo.CreateReviewTask(r.Context(), tx,
		models.KindNewContent, models.TargetArtist, created.ID, &caller.UserID,
		nil, requiresReview); err != nil {
		s.logError(r, "insert artist review task", err)
		internalServerError(w, r)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit create artist", err)
		internalServerError(w, r)
		return
	}

	message := "Artist created."
	if requiresReview {
		message = "Artist pending review."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           created.ID,
		"name":         created.Name,
		"deviantArt":   created.DeviantArt,
		"twitter":      created.Twitter,
		"patreon":      created.Patreon,
		"pixiv":        created.Pixiv,
		"reviewStatus": created.ReviewStatus.String(),
		"message":      message,
	})
}

// optionalString returns nil for an empty or whitespace-only value, so an
// omitted field and an empty string are stored the same way.
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// ensure repo is referenced.
var _ = repo.ErrConflict
