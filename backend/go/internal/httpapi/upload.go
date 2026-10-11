package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/imageproc"
	"waifuapi/internal/repo"
	"waifuapi/internal/storage"
	"waifuapi/internal/store/models"
)

// maxSourceLength matches the C# validator's 500 character limit.
const maxSourceLength = 500

// uploadResponse is the body returned by POST /images/upload.
type uploadResponse struct {
	// ID is the new image ID.
	ID int64 `json:"id"`

	// ReviewStatus tells the client whether the upload is visible yet.
	ReviewStatus string `json:"reviewStatus"`

	// Message is shown to the user by the frontend notification.
	Message string `json:"message"`
}

// registerUploadRoutes mounts the upload endpoint.
func (s *Server) registerUploadRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()
	mux.HandleFunc("POST "+base+"images/upload", s.handleUploadImage)
}

// handleUploadImage serves POST /images/upload.
//
// The sequence matters and mirrors UploadImage/Command.cs:
//
//  1. check the configured minimum role
//  2. read and validate the upload
//  3. measure it, which produces the perceptual hash
//  4. reject exact and near duplicates
//  5. resolve the attached tags and artists, failing if any are unknown
//  6. in one transaction, insert the image, its links and its review task
//  7. upload the bytes to object storage, rolling back on failure
//
// The image row is written before the bytes are stored because the row supplies
// the object key. If the storage write then fails, the transaction is rolled
// back, so no row is left pointing at an object that does not exist.
func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionUploadImage) {
		return
	}
	caller := s.principal(r)

	// The body limit middleware already bounded the request, but the file part
	// is read explicitly so an oversized upload is reported as a 413 rather
	// than surfacing as a parse error.
	if err := r.ParseMultipartForm(s.cfg.MaxUploadSizeBytes); err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			apierror.PayloadTooLarge("Request body exceeds the maximum allowed size.").
				Write(w, r.URL.Path)
			return
		}
		apierror.BadRequest("Expected a multipart/form-data upload.").Write(w, r.URL.Path)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		apierror.UnprocessableEntity("A file is required.").WithExtra("errors",
			map[string][]string{"File": {"File is required."}}).Write(w, r.URL.Path)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		var maxBytes *http.MaxBytesError
		if errors.As(err, &maxBytes) {
			apierror.PayloadTooLarge("File exceeds the maximum allowed size.").Write(w, r.URL.Path)
			return
		}
		internalServerError(w, r)
		return
	}

	contentType := header.Header.Get("Content-Type")

	// Go's multipart writer, and some clients, send application/octet-stream for
	// every part. Trusting that blindly would reject perfectly good images, so
	// the declared type is only used when it actually names an image; otherwise
	// the type is sniffed from the bytes.
	//
	// This is a deliberate divergence from the C# validator, which checks the
	// declared ContentType and nothing else. Being stricter there buys nothing:
	// the format allowlist is still enforced, just against the real content.
	if !imageproc.IsAllowedContentType(contentType) {
		contentType = imageproc.SniffContentType(data)
	}

	// ---- 2. Measure ----
	metadata, err := imageproc.Process(data, header.Filename, contentType, imageproc.Limits{
		MinWidth:          s.cfg.Image.MinWidth,
		MinHeight:         s.cfg.Image.MinHeight,
		MaxWidth:          s.cfg.Image.MaxWidth,
		MaxHeight:         s.cfg.Image.MaxHeight,
		AnimatedMinWidth:  s.cfg.Image.AnimatedMinWidth,
		AnimatedMinHeight: s.cfg.Image.AnimatedMinHeight,
		AnimatedMaxWidth:  s.cfg.Image.AnimatedMaxWidth,
		AnimatedMaxHeight: s.cfg.Image.AnimatedMaxHeight,
	})
	if err != nil {
		s.writeImageProcessError(w, r, err)
		return
	}

	// ---- 3. Duplicate detection ----
	duplicateID, err := s.repo.FindDuplicateByHash(r.Context(), parseHashString(metadata.PerceptualHash))
	if err != nil {
		s.logError(r, "duplicate check", err)
		internalServerError(w, r)
		return
	}
	if duplicateID != 0 {
		apierror.Conflict("Duplicate image found. ID: "+itoa64(duplicateID)).Write(w, r.URL.Path)
		return
	}

	// ---- 4. Resolve relations ----
	tagSlugs := r.Form["tags"]
	artistIDs := parseFormInt64(r.Form["artists"])

	tags, err := s.repo.GetTagsBySlugs(r.Context(), tagSlugs)
	if err != nil {
		s.logError(r, "resolve tags", err)
		internalServerError(w, r)
		return
	}
	// An unknown slug fails the whole upload, matching UploadImage/Command.cs,
	// which throws KeyNotFoundException rather than silently skipping.
	if len(tags) != len(uniqueStrings(tagSlugs)) {
		apierror.NotFound("One or more tags do not exist.").Write(w, r.URL.Path)
		return
	}

	artists, err := s.repo.GetArtistsByIDs(r.Context(), artistIDs)
	if err != nil {
		s.logError(r, "resolve artists", err)
		internalServerError(w, r)
		return
	}
	if len(artists) != len(uniqueInt64s(artistIDs)) {
		apierror.NotFound("One or more artists do not exist.").Write(w, r.URL.Path)
		return
	}

	// ---- 5. Review decision ----
	requiresReview := s.authz.RequiresReviewForImage(roleOf(caller))

	source := optionalFormValue(r, "source")
	if source != nil && len(*source) > maxSourceLength {
		apierror.UnprocessableEntity("Source must not exceed 500 characters.").
			WithExtra("errors", map[string][]string{"Source": {"Source must not exceed 500 characters."}}).
			Write(w, r.URL.Path)
		return
	}

	isNsfw := optionalFormValue(r, "isNsfw") != nil &&
		*optionalFormValue(r, "isNsfw") == "true"

	tagIDs := make([]int64, 0, len(tags))
	for _, tag := range tags {
		tagIDs = append(tagIDs, tag.ID)
	}
	resolvedArtistIDs := make([]int64, 0, len(artists))
	for _, artist := range artists {
		resolvedArtistIDs = append(resolvedArtistIDs, artist.ID)
	}

	// ---- 6. Transaction ----
	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin upload transaction", err)
		internalServerError(w, r)
		return
	}
	// Rollback is a no-op once the transaction commits, so it is deferred
	// unconditionally rather than tracked with a flag.
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	image, err := s.repo.CreateImage(r.Context(), tx, repo.CreateImageInput{
		UploaderID:     &caller.UserID,
		Width:          int64(metadata.Width),
		Height:         int64(metadata.Height),
		ByteSize:       metadata.ByteSize,
		Extension:      metadata.Extension,
		DominantColor:  metadata.DominantColor,
		Source:         source,
		IsNsfw:         isNsfw,
		IsAnimated:     metadata.IsAnimated,
		PerceptualHash: parseHashString(metadata.PerceptualHash),
		ReviewStatus:   reviewStatusFor(requiresReview),
		TagIDs:         tagIDs,
		ArtistIDs:      resolvedArtistIDs,
	})
	if err != nil {
		s.logError(r, "insert image", err)
		internalServerError(w, r)
		return
	}

	if _, err := s.repo.CreateReviewTask(r.Context(), tx,
		models.KindNewContent, models.TargetImage, image.ID, &caller.UserID,
		nil, requiresReview); err != nil {
		s.logError(r, "insert review task", err)
		internalServerError(w, r)
		return
	}

	// ---- 7. Store the bytes ----
	//
	// The upload deliberately happens INSIDE the transaction window, before the
	// commit. A review task references its target by id rather than by foreign
	// key, so committing first and deleting the image afterwards would leave the
	// task orphaned with no way to reach its target. Uploading first means a
	// storage failure rolls back both rows together.
	key := storageKey(image.ID, image.Extension)

	if err := s.storage.Put(r.Context(), key, bytes.NewReader(data), contentType); err != nil {
		// The deferred rollback removes the image and the review task. The
		// object may be partially written, so remove it too; that is idempotent.
		_ = s.storage.Delete(contextWithoutCancel(r), key)
		s.logError(r, "store upload", err)
		apierror.Internal("The image could not be stored. Please try again.").Write(w, r.URL.Path)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		// The rows are gone but the object exists. Remove it, otherwise an
		// orphan is left in storage with no catalogue entry.
		_ = s.storage.Delete(contextWithoutCancel(r), key)
		s.logError(r, "commit upload", err)
		internalServerError(w, r)
		return
	}

	message := "Image uploaded."
	if requiresReview {
		message = "Upload pending review."
	}
	writeJSON(w, http.StatusOK, uploadResponse{
		ID:           image.ID,
		ReviewStatus: image.ReviewStatus.String(),
		Message:      message,
	})
}

// writeImageProcessError maps an imaging failure onto an HTTP response.
func (s *Server) writeImageProcessError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, imageproc.ErrUnsupportedFormat) {
		apierror.UnprocessableEntity(err.Error()).WithExtra("errors",
			map[string][]string{"File": {err.Error()}}).Write(w, r.URL.Path)
		return
	}

	var tooLarge *imageproc.ErrTooLarge
	if errors.As(err, &tooLarge) {
		apierror.UnprocessableEntity(err.Error()).WithExtra("errors",
			map[string][]string{"File": {err.Error()}}).Write(w, r.URL.Path)
		return
	}

	if errors.Is(err, errors.New("image data is empty")) {
		apierror.UnprocessableEntity("The uploaded file is empty.").Write(w, r.URL.Path)
		return
	}

	// A decode failure means the bytes were not a usable image.
	apierror.UnprocessableEntity("The uploaded file could not be decoded as an image.").
		WithExtra("errors", map[string][]string{"File": {"Unsupported or corrupt image."}}).
		Write(w, r.URL.Path)
}

// ensure storage is referenced from this file's imports.
var _ storage.Store
