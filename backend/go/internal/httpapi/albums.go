package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/repo"
)

// registerAlbumRoutes mounts the album endpoints.
//
// The route shape is AlbumsController.cs exactly. Two things about it are easy to
// get wrong from the controller alone:
//
//   - Both path segments are strings, not numbers. {userId} accepts "me" and
//     {id} accepts "favorites", and both are resolved by the same helpers the
//     C# service uses (ResolveUserIdAsync / ResolveAlbumIdAsync). There is no
//     separate /favorites route; the literal is resolved inside the {id}
//     handlers.
//   - The {userId} segment is only validated for access, not ownership: a
//     moderator may read and edit anyone's albums, and every mutating handler
//     still scopes the SQL by the resolved user id.
func (s *Server) registerAlbumRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"users/{userId}/albums", s.handleListAlbums)
	mux.HandleFunc("POST "+base+"users/{userId}/albums", s.handleCreateAlbum)
	mux.HandleFunc("GET "+base+"users/{userId}/albums/{id}", s.handleGetAlbum)
	mux.HandleFunc("PATCH "+base+"users/{userId}/albums/{id}", s.handleUpdateAlbum)
	mux.HandleFunc("DELETE "+base+"users/{userId}/albums/{id}", s.handleDeleteAlbum)
	mux.HandleFunc("GET "+base+"users/{userId}/albums/{id}/images", s.handleListAlbumImages)
	mux.HandleFunc("POST "+base+"users/{userId}/albums/{id}/images/{imageId}", s.handleAddAlbumItem)
	mux.HandleFunc("DELETE "+base+"users/{userId}/albums/{id}/images/{imageId}", s.handleRemoveAlbumItem)
}

// albumDto is the wire form of an album.
//
// The shape is AlbumDto.cs: there is no embedded images array. Album contents
// come from the separate .../images endpoint, and imageCount is the only
// summary here.
type albumDto struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"isDefault"`
	UserID      int64  `json:"userId"`
	ImageCount  int    `json:"imageCount"`
}

// toAlbumDto converts an album row.
func toAlbumDto(album repo.Album) albumDto {
	return albumDto{
		ID:          album.ID,
		Name:        album.Name,
		Description: album.Description,
		IsDefault:   album.IsDefault,
		UserID:      album.UserID,
		ImageCount:  album.ImageCount,
	}
}

// resolveAlbumUserId resolves the {userId} segment and checks access.
//
// It mirrors ResolveUserIdAsync followed by CanAccess: "me" becomes the
// caller's id, anything else must parse as a number, and the caller must be the
// owner or an admin. A caller who is neither gets 404 rather than 403, so the
// existence of another account is not disclosed — which is what CanAccess
// followed by NotFound produces in the C# service.
func (s *Server) resolveAlbumUserId(w http.ResponseWriter, r *http.Request) (int64, bool) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return 0, false
	}

	raw := r.PathValue("userId")
	if strings.EqualFold(raw, "me") {
		return caller.UserID, true
	}

	target, err := parsePositiveInt64(raw)
	if err != nil {
		writeAPIError(w, r, apierror.BadRequest("Invalid User ID format."))
		return 0, false
	}

	if target != caller.UserID && !caller.IsAdmin() {
		apierror.NotFound("User not found.").Write(w, r.URL.Path)
		return 0, false
	}
	return target, true
}

// resolveAlbumID resolves the {id} segment.
//
// "favorites" is the user's default album and is created on demand, matching
// ResolveAlbumIdAsync, so the frontend can like an image before the album
// exists. A value that is neither "favorites" nor a number is a 400.
//
// The ownership check is not done here: CanAccess is about the *user*, and the
// album's own owner is enforced by the SQL in each repository call.
func (s *Server) resolveAlbumID(w http.ResponseWriter, r *http.Request, userID int64) (int64, bool) {
	raw := r.PathValue("id")

	if strings.EqualFold(raw, "favorites") {
		albumID, err := s.repo.EnsureDefaultAlbum(r.Context(), userID, "Favorites")
		if err != nil {
			s.logError(r, "resolve default album", err)
			internalServerError(w, r)
			return 0, false
		}
		return albumID, true
	}

	albumID, err := parsePositiveInt64(raw)
	if err != nil {
		writeAPIError(w, r, apierror.BadRequest("Invalid Album ID format."))
		return 0, false
	}
	return albumID, true
}

// handleListAlbums serves GET /users/{userId}/albums.
func (s *Server) handleListAlbums(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.Album)
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
		dtos = append(dtos, toAlbumDto(album))
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// handleGetAlbum serves GET /users/{userId}/albums/{id}.
func (s *Server) handleGetAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	album, err := s.repo.GetAlbum(r.Context(), albumID, userID)
	if err != nil {
		writeRepoError(w, r, "get album", err, "Album not found.")
		return
	}

	writeJSON(w, http.StatusOK, toAlbumDto(album))
}

// albumRequest is the body for creating and updating an album.
type albumRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleCreateAlbum serves POST /users/{userId}/albums.
func (s *Server) handleCreateAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	var body albumRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, albumNameRequired())
		return
	}

	album, err := s.repo.CreateAlbum(r.Context(), userID, name, strings.TrimSpace(body.Description))
	if err != nil {
		s.logError(r, "create album", err)
		internalServerError(w, r)
		return
	}

	location := s.baseLocationPrefix() + "users/" + itoa64(userID) +
		"/albums/" + itoa64(album.ID)
	s.writeCreated(w, r, toAlbumDto(album), location)
}

// albumNameRequired is the validation failure for an empty album name.
//
// The shape matters: the C# service runs this through FluentValidation, which
// produces a ValidationProblemDetails carrying an "errors" member keyed by field
// name. The frontend reads that member to highlight inputs.
func albumNameRequired() *apierror.Error {
	return apierror.BadRequest("Name is required.").
		WithExtra("errors", map[string][]string{"Name": {"Name is required."}})
}

// handleUpdateAlbum serves PATCH /users/{userId}/albums/{id}.
func (s *Server) handleUpdateAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	var body albumRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeAPIError(w, r, albumNameRequired())
		return
	}

	album, err := s.repo.UpdateAlbum(r.Context(), albumID, userID, name, strings.TrimSpace(body.Description))
	if err != nil {
		writeRepoError(w, r, "update album", err, "Album not found.")
		return
	}

	writeJSON(w, http.StatusOK, toAlbumDto(album))
}

// handleDeleteAlbum serves DELETE /users/{userId}/albums/{id}.
func (s *Server) handleDeleteAlbum(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	// The default album cannot be deleted: every user has exactly one and the
	// frontend addresses it by name, so removing it would break the like button
	// for a user who cannot create a replacement.
	if albumID != 0 {
		album, err := s.repo.GetAlbum(r.Context(), albumID, userID)
		if err != nil {
			writeRepoError(w, r, "get album for delete", err, "Album not found.")
			return
		}
		if album.IsDefault {
			writeAPIError(w, r, apierror.BadRequest("Cannot delete the default album."))
			return
		}
	}

	if err := s.repo.DeleteAlbum(r.Context(), albumID, userID); err != nil {
		writeRepoError(w, r, "delete album", err, "Album not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListAlbumImages serves GET /users/{userId}/albums/{id}/images.
//
// It runs the same query builder as /images with the album constraint added, so
// every filter and sort option is available here too.
func (s *Server) handleListAlbumImages(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	// The album must exist and belong to the addressed user before its contents
	// are listed, otherwise the response is an empty page rather than a 404.
	if _, err := s.repo.GetAlbum(r.Context(), albumID, userID); err != nil {
		writeRepoError(w, r, "get album", err, "Album not found.")
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.Image)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter, err := s.imageFilterFromRequest(r, &albumID)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	filter.Page = page.Page
	filter.PageSize = page.PageSize

	s.writeImagePage(w, r, filter, page, albumID)
}

// handleAddAlbumItem serves POST /users/{userId}/albums/{id}/images/{imageId}.
//
// A missing album or image is silently ignored rather than reported: the C#
// handler returns NoContent unconditionally, because the action is idempotent
// and the frontend treats the like button as fire-and-forget.
func (s *Server) handleAddAlbumItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	imageID, err := parsePathInt64(r, "imageId")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.AddAlbumItem(r.Context(), albumID, userID, imageID); err != nil {
		if err != repo.ErrNotFound {
			s.logError(r, "add album item", err)
			internalServerError(w, r)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveAlbumItem serves DELETE /users/{userId}/albums/{id}/images/{imageId}.
func (s *Server) handleRemoveAlbumItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.resolveAlbumUserId(w, r)
	if !ok {
		return
	}

	albumID, ok := s.resolveAlbumID(w, r, userID)
	if !ok {
		return
	}

	imageID, err := parsePathInt64(r, "imageId")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.RemoveAlbumItem(r.Context(), albumID, userID, imageID); err != nil {
		if err != repo.ErrNotFound {
			s.logError(r, "remove album item", err)
			internalServerError(w, r)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// viewerIDForAlbum reports whose per-viewer image fields to load on the album
// endpoint.
//
// The album belongs to the addressed user, but the liked/albums fields on each
// image are relative to the *caller*. A moderator browsing someone else's album
// must still see their own state, so this returns the caller's id and ignores
// the album's owner.
func (s *Server) viewerIDForAlbum(r *http.Request, albumID *int64) int64 {
	if albumID == nil {
		return 0
	}
	caller := s.principal(r)
	if caller == nil {
		return 0
	}
	return caller.UserID
}
