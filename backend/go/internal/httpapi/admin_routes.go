package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// applyImageEdit writes the editable metadata of an image.
//
// Tag and artist membership is left alone: changing what an image depicts is a
// reviewed edit, not a metadata correction, and the two are separate routes in
// the C# service.
func (s *Server) applyImageEdit(r *http.Request, image models.Image, source *string, isNsfw bool) error {
	return s.repo.SetImageEditableFields(r.Context(), image.ID, source, isNsfw)
}

// This file holds the endpoints that are neither CRUD on a single resource nor
// part of the review queue: the moderation dashboard, API key maintenance and
// the image metadata edit.

// registerAdminStatsRoute mounts GET /stats/admin.
func (s *Server) registerAdminStatsRoute(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()
	mux.HandleFunc("GET "+base+"stats/admin", s.handleAdminStats)
}

// registerAPIKeyUpdateRoute mounts PATCH /auth/api-keys/{id}.
func (s *Server) registerAPIKeyUpdateRoute(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()
	mux.HandleFunc("PATCH "+base+"auth/api-keys/{id}", s.handleUpdateAPIKey)
}

// registerImageUpdateRoute mounts PATCH /images/{id}.
func (s *Server) registerImageMutationRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("PATCH "+base+"images/{id}", s.handleUpdateImage)
	mux.HandleFunc("DELETE "+base+"images/{id}", s.handleDeleteImage)
}

// adminStatsResponse is the dashboard payload.
//
// The shape is GetAdminStatsDto: a block of totals plus a per-day series. The
// frontend renders them as cards and a chart, so both halves are required.
type adminStatsResponse struct {
	TotalRequests int64            `json:"totalRequests"`
	TotalImages   int64            `json:"totalImages"`
	TotalTags     int64            `json:"totalTags"`
	TotalArtists  int64            `json:"totalArtists"`
	TotalUsers    int64            `json:"totalUsers"`
	PendingTasks  int64            `json:"pendingTasks"`
	OpenReports   int64            `json:"openReports"`
	DailyStats    []dailyStatPoint `json:"dailyStats"`
}

// dailyStatPoint is one day of request counts.
type dailyStatPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// handleAdminStats serves GET /stats/admin.
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "view admin statistics") {
		return
	}

	stats, err := s.repo.GetAdminStats(r.Context())
	if err != nil {
		s.logError(r, "admin stats", err)
		internalServerError(w, r)
		return
	}

	daily, err := s.repo.GetDailyStats(r.Context(), statsWindowDays)
	if err != nil {
		s.logError(r, "daily stats", err)
		internalServerError(w, r)
		return
	}

	points := make([]dailyStatPoint, 0, len(daily))
	for _, day := range daily {
		points = append(points, dailyStatPoint{
			Date:  day.Date.Format("2006-01-02"),
			Count: day.RequestCount,
		})
	}

	writeJSON(w, http.StatusOK, adminStatsResponse{
		TotalRequests: stats.TotalRequests,
		TotalImages:   stats.TotalImages,
		TotalTags:     stats.TotalTags,
		TotalArtists:  stats.TotalArtists,
		TotalUsers:    stats.TotalUsers,
		PendingTasks:  stats.PendingTasks,
		OpenReports:   stats.OpenReports,
		DailyStats:    points,
	})
}

// statsWindowDays is how many days of per-day counts the dashboard returns.
const statsWindowDays = 30

// updateAPIKeyRequest is the PATCH body.
type updateAPIKeyRequest struct {
	Description string `json:"description"`
}

// handleUpdateAPIKey serves PATCH /auth/api-keys/{id}.
//
// Only the description is editable. The secret cannot be rotated in place,
// because that would silently break whichever client was using it; a new key is
// created instead and the old one revoked.
func (s *Server) handleUpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateAPIKeyRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	description := strings.TrimSpace(body.Description)
	if description == "" {
		writeAPIError(w, r, validationError("Description", "Description is required."))
		return
	}

	key, err := s.repo.UpdateAPIKeyDescription(r.Context(), id, caller.UserID, description)
	if err != nil {
		writeRepoError(w, r, "update api key", err, "API key not found.")
		return
	}

	writeJSON(w, http.StatusOK, toAPIKeyDto(key))
}

// updateImageRequest is the PATCH body.
type updateImageRequest struct {
	Source *string `json:"source"`
	IsNsfw *bool   `json:"isNsfw"`
}

// handleUpdateImage serves PATCH /images/{id}.
//
// Moderator only. ImagesController.Update carries
// [Authorize(Policy = AuthorizationPolicies.Moderator)] and UpdateImageCommand has
// no ownership check of its own, so an ordinary user cannot edit an image at
// all — including one they uploaded.
//
// The previous implementation here allowed the uploader in and read JSON. Both
// were wrong: the policy made it more permissive than the original, and [FromForm]
// on the C# side means the moderator screen sends multipart, which the JSON
// decoder would have rejected outright.
func (s *Server) handleUpdateImage(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleModerator, "edit images") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if _, err := s.repo.GetImageByID(r.Context(), id, true); err != nil {
		writeRepoError(w, r, "get image for update", err,
			"Image with ID "+itoa64(id)+" not found.")
		return
	}

	fields, err := parseImageUpdateForm(r, s.cfg.MaxUploadSizeBytes)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.applyImageUpdate(r.Context(), id, fields); err != nil {
		s.logError(r, "update image", err)
		writeImageUpdateError(w, r, id, err)
		return
	}

	updated, err := s.repo.GetImageByID(r.Context(), id, true)
	if err != nil {
		writeRepoError(w, r, "re-read image", err,
			"Image with ID "+itoa64(id)+" not found.")
		return
	}

	// includeUploaderId is true because UpdateImageCommand maps with
	// ToDto(_cdnBaseUrl, includeUploaderId: true).
	dtos, err := s.toImageDtos(r, []models.Image{updated}, repo.ImageFilter{
		ReviewStatus: models.FilterAll,
	}, 0)
	if err != nil || len(dtos) == 0 {
		s.logError(r, "resolve image relations", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dtos[0])
}

// handleDeleteImage serves DELETE /images/{id}.
//
// Admin only. ImagesController.Delete carries [Authorize(Policy =
// AuthorizationPolicies.Admin)], which short-circuits before DeleteImageCommand
// ever runs, so the command's ownership allowance is unreachable from the HTTP
// surface: an ordinary user cannot delete their own image here, and must cancel
// the upload instead.
//
// DeleteImageCommand does contain
// `if (image.UploaderId != request.RequesterId && !request.IsAdminOrModerator)`,
// and an earlier version of this handler implemented only that check. Reading the
// command and missing the policy on the controller is how a moderator-only
// endpoint quietly became user-accessible.
func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "delete images") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	image, err := s.repo.GetImageByID(r.Context(), id, true)
	if err != nil {
		writeRepoError(w, r, "get image for deletion", err,
			"Image with ID "+itoa64(id)+" not found.")
		return
	}

	if err := s.repo.DeleteImage(r.Context(), id); err != nil {
		s.logError(r, "delete image row", err)
		internalServerError(w, r)
		return
	}

	// Best effort: the row is already gone, and a lingering object is an
	// operational nuisance that storage-diff will surface rather than a
	// correctness problem.
	if err := s.storage.Delete(contextWithoutCancel(r), storageKey(id, image.Extension)); err != nil {
		s.logError(r, "delete stored object", err)
	}
	s.purger.PurgeImage(contextWithoutCancel(r), id, []string{image.Extension})

	w.WriteHeader(http.StatusNoContent)
}

// deleteImageObjects removes an image's stored object and purges the edge.
//
// extraExtensions lists any additional extension to remove, which is how a file
// replacement cleans up the old object alongside the new one. It is best effort
// throughout: the database row is already gone by the time this runs, so a
// storage failure is an operational nuisance for storage-diff to find, not a
// reason to fail a request that already succeeded.
func (s *Server) deleteImageObjects(r *http.Request, imageID int64, extraExtension string) {
	// The row is gone, so the extension has to come from the payload the task
	// carried; the caller passes it when it is known.
	image, err := s.repo.GetImageByID(contextWithoutCancel(r), imageID, true)
	var extension string
	if err == nil {
		extension = image.Extension
	}
	if extension == "" {
		extension = extraExtension
	}
	if extension == "" {
		return
	}

	if err := s.storage.Delete(contextWithoutCancel(r), storageKey(imageID, extension)); err != nil {
		s.logError(r, "delete stored object", err)
		return
	}
	s.purgeImage(contextWithoutCancel(r), imageID, []string{extension})
}
