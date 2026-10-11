package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/repo"
)

// registerAlbumRoutes mounts the album endpoints.
//
// Albums live under /users/{userId}/albums in the C# service, so the user id is
// part of the path. Ownership is enforced on every mutating route rather than
// relying on the caller to pass their own id.
func (s *Server) registerAlbumRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"users/{userId}/albums", s.handleListAlbums)
	mux.HandleFunc("POST "+base+"users/{userId}/albums", s.handleCreateAlbum)
	mux.HandleFunc("GET "+base+"users/{userId}/albums/{id}", s.handleGetAlbum)
	mux.HandleFunc("DELETE "+base+"users/{userId}/albums/{id}", s.handleDeleteAlbum)
	mux.HandleFunc("PUT "+base+"users/{userId}/albums/{id}", s.handleUpdateAlbum)
	mux.HandleFunc("POST "+base+"users/{userId}/albums/{id}/items", s.handleAddAlbumItem)
	mux.HandleFunc("DELETE "+base+"users/{userId}/albums/{id}/items/{imageId}", s.handleRemoveAlbumItem)
	mux.HandleFunc("PATCH "+base+"users/{userId}/albums/default", s.handleSetDefaultAlbum)
}

// albumDto is the wire form of an album.
type albumDto struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsDefault   bool       `json:"isDefault"`
	Images      []ImageDto `json:"images"`
	ImageCount  int        `json:"imageCount"`
}

// requireOwnedUserId checks that the caller owns the addressed user.
//
// It returns the parsed id, and writes the response and returns 0 when the
// caller is not permitted. An administrator may act on any account, which is why
// this is not a simple equality check.
func (s *Server) requireOwnedUserId(w http.ResponseWriter, r *http.Request) (caller *principal, userID int64) {
	caller = s.requireAuth(w, r)
	if caller == nil {
		return nil, 0
	}

	userID, err := parsePathInt64(r, "userId")
	if err != nil {
		writeAPIError(w, r, err)
		return nil, 0
	}

	if caller.UserID != userID && !caller.IsAdmin() {
		// 404 rather than 403, so another account's existence is not disclosed.
		apierror.NotFound("User not found.").Write(w, r.URL.Path)
		return nil, 0
	}
	return caller, userID
}

// handleListAlbums serves GET /users/{userId}/albums.
func (s *Server) handleListAlbums(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	albums, total, err := s.repo.ListAlbums(r.Context(), userID, page.offset(), page.PageSize)
	if err != nil {
		s.logError(r, "list albums", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]albumDto, 0, len(albums))
	for _, album := range albums {
		dtos = append(dtos, s.toAlbumDto(r, album, true))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
}

// handleGetAlbum serves GET /users/{userId}/albums/{id}.
func (s *Server) handleGetAlbum(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	albumID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	album, err := s.repo.GetAlbum(r.Context(), albumID, userID)
	if err != nil {
		writeRepoError(w, r, "get album", err, "Album not found.")
		return
	}

	writeJSON(w, http.StatusOK, s.toAlbumDto(r, album, true))
}

// albumRequest is the body for creating and updating an album.
type albumRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleCreateAlbum serves POST /users/{userId}/albums.
func (s *Server) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	var body albumRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("A name is required.").
			WithExtra("errors", map[string][]string{"Name": {"A name is required."}}))
		return
	}

	album, err := s.repo.CreateAlbum(r.Context(), userID, name, strings.TrimSpace(body.Description))
	if err != nil {
		s.logError(r, "create album", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, s.toAlbumDto(r, album, true))
}

// handleUpdateAlbum serves PUT /users/{userId}/albums/{id}.
func (s *Server) handleUpdateAlbum(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	albumID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body albumRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("A name is required.").
			WithExtra("errors", map[string][]string{"Name": {"A name is required."}}))
		return
	}

	album, err := s.repo.UpdateAlbum(r.Context(), albumID, userID, name, strings.TrimSpace(body.Description))
	if err != nil {
		writeRepoError(w, r, "update album", err, "Album not found.")
		return
	}

	writeJSON(w, http.StatusOK, s.toAlbumDto(r, album, true))
}

// handleDeleteAlbum serves DELETE /users/{userId}/albums/{id}.
func (s *Server) handleDeleteAlbum(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	albumID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.DeleteAlbum(r.Context(), albumID, userID); err != nil {
		writeRepoError(w, r, "delete album", err, "Album not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// albumItemRequest is the body for adding an image to an album.
type albumItemRequest struct {
	ImageID int64 `json:"imageId"`
}

// handleAddAlbumItem serves POST /users/{userId}/albums/{id}/items.
func (s *Server) handleAddAlbumItem(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	albumID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body albumItemRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if body.ImageID == 0 {
		writeAPIError(w, r, apierror.UnprocessableEntity("An imageId is required."))
		return
	}

	if err := s.repo.AddAlbumItem(r.Context(), albumID, userID, body.ImageID); err != nil {
		writeRepoError(w, r, "add album item", err, "Album or image not found.")
		return
	}

	album, err := s.repo.GetAlbum(r.Context(), albumID, userID)
	if err != nil {
		writeRepoError(w, r, "get album after add", err, "Album not found.")
		return
	}

	writeJSON(w, http.StatusOK, s.toAlbumDto(r, album, true))
}

// handleRemoveAlbumItem serves DELETE /users/{userId}/albums/{id}/items/{imageId}.
func (s *Server) handleRemoveAlbumItem(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	albumID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	imageID, err := parsePathInt64(r, "imageId")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.RemoveAlbumItem(r.Context(), albumID, userID, imageID); err != nil {
		writeRepoError(w, r, "remove album item", err, "Album item not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSetDefaultAlbum serves PATCH /users/{userId}/albums/default.
//
// Only one album can be the default, so setting one clears the others. That is
// done in a transaction, otherwise two concurrent requests could leave two
// defaults behind.
func (s *Server) handleSetDefaultAlbum(w http.ResponseWriter, r *http.Request) {
	_, userID := s.requireOwnedUserId(w, r)
	if userID == 0 {
		return
	}

	var body albumItemRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin set default album", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	if err := s.repo.SetDefaultAlbum(r.Context(), tx, body.ImageID, userID); err != nil {
		writeRepoError(w, r, "set default album", err, "Album not found.")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit set default album", err)
		internalServerError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// toAlbumDto converts an album, optionally resolving its images.
func (s *Server) toAlbumDto(r *http.Request, album repo.Album, includeImages bool) albumDto {
	dto := albumDto{
		ID:          album.ID,
		Name:        album.Name,
		Description: album.Description,
		IsDefault:   album.IsDefault,
		Images:      []ImageDto{},
		ImageCount:  album.ImageCount,
	}

	if !includeImages {
		return dto
	}

	images, err := s.repo.GetAlbumImages(r.Context(), album.ID, album.UserID)
	if err != nil {
		s.logError(r, "list album images", err)
		return dto
	}

	dto.Images = s.toImageDtos(r.Context(), images)
	dto.ImageCount = len(dto.Images)
	return dto
}

// ensure repo is referenced.
var _ = repo.ErrNotFound
