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

	// The declared content type is checked, and nothing else, matching
	// UploadImageCommandValidator:
	//
	//	RuleFor(x => x.ContentType)
	//	    .NotEmpty().WithMessage("Content type is required.")
	//	    .Must(x => x == "image/jpeg" || x == "image/png" || x == "image/gif" || x == "image/webp")
	//	    .WithMessage("Only JPEG, PNG, GIF, and WEBP formats are supported.");
	//
	// An earlier version sniffed the bytes whenever the declared type was not an
	// image, which was more forgiving: it accepted uploads the original rejects,
	// such as a correct PNG sent as application/octet-stream. That looked like a
	// strict improvement, and it was, but it is still a behaviour change, and a
	// rewrite that quietly widens what an endpoint accepts is the exact failure
	// this project exists to avoid. Clients keep the status codes they had.
	//
	// If the sniffing behaviour is genuinely wanted, it belongs in a change to
	// the C# service first, so both sides can be deployed together.
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		apierror.BadRequest("Content type is required.").WithExtra("errors",
			map[string][]string{"ContentType": {"Content type is required."}}).
			Write(w, r.URL.Path)
		return
	}
	if !imageproc.IsAllowedContentType(contentType) {
		apierror.BadRequest("Only JPEG, PNG, GIF, and WEBP formats are supported.").
			WithExtra("errors", map[string][]string{
				"ContentType": {"Only JPEG, PNG, GIF, and WEBP formats are supported."},
			}).Write(w, r.URL.Path)
		return
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

	// Source is trimmed and an empty value becomes null, matching
	// StringExtensions.ToNullIfEmpty in the upload command. Storing "" instead of
	// null would make the image page render an empty link.
	source := optionalFormValue(r, "source")
	if source != nil {
		source = optionalString(*source)
	}
	if source != nil {
		switch {
		case len([]rune(*source)) > maxSourceLength:
			writeAPIError(w, r, validationError("Source", "Source URL must not exceed 500 characters."))
			return
		case !isAbsoluteURL(*source):
			writeAPIError(w, r, validationError("Source", "Source must be a valid URL."))
			return
		}
	}

	isNsfw := truthy(r.FormValue("isNsfw"))

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

	// The response is the full ImageDto, not a status envelope. The controller
	// returns CreatedAtAction, and the frontend types the response as ImageDto
	// and navigates using its id; a trimmed-down body would break any client
	// reading reviewStatus or tags from the upload response.
	//
	// favourites and likedAt are deliberately left at zero: a freshly uploaded
	// image is in nobody's album, and the command that builds this response has no
	// viewer context.
	image.UploaderID = &caller.UserID

	dtos, err := s.toImageDtos(r, []models.Image{image}, repo.ImageFilter{
		ReviewStatus: models.FilterAll,
	}, 0)
	if err != nil || len(dtos) == 0 {
		// The upload committed; failing to re-read it for the response body
		// would misreport a successful upload as an error and invite a retry
		// that then trips duplicate detection.
		s.logError(r, "render uploaded image", err)
		internalServerError(w, r)
		return
	}

	location := s.baseLocationPrefix() + "images/" + itoa64(image.ID)
	s.writeCreated(w, r, dtos[0], location)
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
