package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/imageproc"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// This file implements the moderator-only image mutation endpoints:
// PATCH /images/{id} and DELETE /images/{id}.
//
// Both are more capable than a name suggests, and both had a previous
// implementation that was quietly wrong in ways worth writing down.

// imageUpdateFields is the mutable subset of an image, as UpdateImageCommand
// computes it.
//
// The pointers matter. C# uses non-nullable bool for IsNsfw and assigns it
// unconditionally, so a patch that omits the form field writes `false` rather
// than leaving the current value; but Source, UserId, ReviewStatus, Tags and
// Artists are all nullable and have three distinguishable states. A plain struct
// cannot express "absent" separately from "present and empty", and here those
// two produce different results.
type imageUpdateFields struct {
	// Source is applied unconditionally, like C#: absent and empty both mean
	// "clear the attribution".
	source *string

	// isNsfwSet mirrors the C# non-nullable bool. When false the caller omitted
	// the field and the stored value stands; when true it is written as given.
	isNsfwSet bool
	isNsfw    bool

	// uploaderIDSet distinguishes "field present" from "field absent". Absent is
	// NOT the same as absent-means-keep in C#: UpdateImageCommand clears the
	// uploader when no UserId is supplied, so an admin editing an image's source
	// link without restating the uploader orphans it.
	uploaderIDSet bool
	uploaderID    *int64

	// reviewStatus is applied only when supplied.
	reviewStatus *models.ReviewStatus

	// tagSlugs present-but-empty clears every tag; absent leaves them alone.
	tagSlugsSet bool
	tagSlugs    []string

	// artistIDs behaves the same way as tagSlugs.
	artistIDsSet bool
	artistIDs    []int64

	// replacement carries a new file, if one was uploaded.
	replacement []byte
	// declaredType is the new file's Content-Type, checked against the same
	// allowlist the upload endpoint uses.
	declaredType    string
	replacementName string
}

// clearSource is the value C# writes when the source is absent or empty.
func clearSource() *string { return nil }

// parseImageUpdateForm reads the multipart body UpdateImageRequest binds from.
//
// ASP.NET binds this with [FromForm], so the request is multipart with repeated
// keys for the list fields. The previous implementation decoded JSON here, which
// meant the moderator screen — which sends multipart — could not have worked at
// all against this port.
func parseImageUpdateForm(r *http.Request, maxBytes int64) (imageUpdateFields, error) {
	var fields imageUpdateFields

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		return fields, apierror.BadRequest("Request body is not a valid multipart form.")
	}

	// Source and IsNsfw are always written, so their absence is still a value.
	fields.source = formSource(r.FormValue("source"))

	if raw := strings.TrimSpace(r.FormValue("isNsfw")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return fields, validationError("IsNsfw", "IsNsfw must be true or false.")
		}
		fields.isNsfwSet, fields.isNsfw = true, parsed
	}

	// An absent UserId clears the uploader in C#; only an explicit 0 does the
	// same thing. Both are honoured, and neither is silently "keep".
	if raw := strings.TrimSpace(r.FormValue("userId")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fields, validationError("UserId", "UserId must be a number.")
		}
		fields.uploaderIDSet = true
		if parsed == 0 {
			fields.uploaderID = clearUploader()
		} else {
			id := parsed
			fields.uploaderID = &id
		}
	} else {
		fields.uploaderIDSet, fields.uploaderID = true, clearUploader()
	}

	if values, present := r.MultipartForm.Value["tags"]; present {
		fields.tagSlugsSet, fields.tagSlugs = true, trimAll(values)
	}
	if values, present := r.MultipartForm.Value["artists"]; present {
		ids := make([]int64, 0, len(values))
		for _, raw := range values {
			for _, piece := range strings.Split(raw, ",") {
				piece = strings.TrimSpace(piece)
				if piece == "" {
					continue
				}
				parsed, err := strconv.ParseInt(piece, 10, 64)
				if err != nil {
					return fields, validationError("Artists", "Artists must be a list of ids.")
				}
				ids = append(ids, parsed)
			}
		}
		fields.artistIDsSet, fields.artistIDs = true, ids
	}

	if raw := strings.TrimSpace(r.FormValue("reviewStatus")); raw != "" {
		status, ok := models.ParseReviewStatus(raw)
		if !ok {
			if number, err := strconv.Atoi(raw); err == nil {
				switch models.ReviewStatus(number) {
				case models.ReviewPending, models.ReviewAccepted:
					status, ok = models.ReviewStatus(number), true
				}
			}
		}
		if !ok {
			return fields, validationError("ReviewStatus", "Invalid review status.")
		}
		fields.reviewStatus = &status
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return fields, nil
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		return fields, apierror.PayloadTooLarge("File exceeds the maximum allowed size.")
	}

	declared := header.Header.Get("Content-Type")
	if !imageproc.IsAllowedContentType(declared) {
		return fields, validationError("ContentType",
			"Only JPEG, PNG, GIF, and WEBP formats are supported.")
	}

	fields.replacement = data
	fields.declaredType = declared
	fields.replacementName = header.Filename

	return fields, nil
}

// formSource mirrors ToNullIfEmpty().Trim(): blank becomes NULL.
func formSource(raw string) *string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return clearSource()
	}
	return &trimmed
}

func trimAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, piece := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(piece); trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	return out
}

func clearUploader() *int64 { return nil }

// applyImageUpdate performs the whole patch in one transaction.
//
// Ordering follows UpdateImageCommand: scalar fields first, then the tag and
// artist link replacement, then the review-queue sync. The transaction is not
// cosmetic. In C# the tag set is validated before any save, so a bad slug throws
// and nothing is written; doing the same here means a moderator cannot end up
// with half-applied metadata because the fourth slug did not resolve.
func (s *Server) applyImageUpdate(ctx context.Context, id int64, fields imageUpdateFields) error {
	tx, err := s.pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin image update: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	image, err := s.repo.GetImageByID(ctx, id, true)
	if err != nil {
		return err
	}

	oldExtension := image.Extension

	// Set when a replacement file is stored, so the post-commit write below puts
	// the same bytes in the bucket that the row describes.
	var storedReplacement []byte
	var storedReplacementType string

	// A replacement file is measured before anything is written, so an
	// undecodable upload aborts with the original image untouched.
	if len(fields.replacement) > 0 {
		metadata, err := imageproc.Process(fields.replacement, fields.replacementName,
			fields.declaredType, imageproc.Limits{
				MinWidth:          s.cfg.Image.MinWidth,
				MinHeight:         s.cfg.Image.MinHeight,
				MaxWidth:          s.cfg.Image.MaxWidth,
				MaxHeight:         s.cfg.Image.MaxHeight,
				AnimatedMinWidth:  s.cfg.Image.AnimatedMinWidth,
				AnimatedMinHeight: s.cfg.Image.AnimatedMinHeight,
			})
		if err != nil {
			return fmt.Errorf("process replacement: %w", err)
		}

		// Re-encode after measuring, for the same reason the upload path does:
		// the perceptual hash stays derived from the original pixels, so a
		// converted replacement still collides with an existing raw upload of
		// the same image.
		replacement := fields.replacement
		replacementType := fields.declaredType

		if conversion, err := imageproc.TryConvertWebP(fields.replacement,
			metadata.Extension, metadata.IsAnimated, s.cfg.Image.WebpConversion()); err != nil {
			s.logConversionFailure(ctx, err)
		} else if conversion != nil {
			replacement = conversion.Content
			replacementType = conversion.ContentType
			metadata.Extension = conversion.Extension
			metadata.ByteSize = conversion.ByteSize
		}

		storedReplacement = replacement
		storedReplacementType = replacementType

		extension := metadata.Extension
		if extension != "" && extension != oldExtension {
			if _, err := tx.Exec(ctx,
				`UPDATE "Images" SET "Extension" = $1 WHERE "Id" = $2`, extension, id); err != nil {
				return fmt.Errorf("update extension: %w", err)
			}
			oldExtension = extension
		}

		// The column is bit(64) and Postgres rejects a plain string for it, so
		// the hash is converted rather than interpolated. Passing the
		// imageproc hex form directly is the error
		// `"D" is not a valid binary digit`.
		if _, err := tx.Exec(ctx, `
            UPDATE "Images"
            SET "Width" = $1, "Height" = $2, "ByteSize" = $3,
                "DominantColor" = $4, "IsAnimated" = $5,
                "PerceptualHash" = $6::bit(64)
            WHERE "Id" = $7`,
			metadata.Width, metadata.Height, metadata.ByteSize,
			metadata.DominantColor, metadata.IsAnimated,
			models.PerceptualHashToBitString(parseHashString(metadata.PerceptualHash)), id); err != nil {
			return fmt.Errorf("update replacement metadata: %w", err)
		}
	}

	if fields.uploaderIDSet {
		if fields.uploaderID != nil {
			var exists bool
			if err := tx.QueryRow(ctx,
				`SELECT EXISTS (SELECT 1 FROM "Users" WHERE "Id" = $1)`,
				*fields.uploaderID).Scan(&exists); err != nil {
				return fmt.Errorf("check uploader: %w", err)
			}
			if !exists {
				return repo.ErrNotFound
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE "Images" SET "Source" = $1, "IsNsfw" = $2, "UploaderId" = $3 WHERE "Id" = $4`,
			fields.source, fields.isNsfw, fields.uploaderID, id); err != nil {
			return fmt.Errorf("update image fields: %w", err)
		}
	}

	if fields.reviewStatus != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE "Images" SET "ReviewStatus" = $1 WHERE "Id" = $2`,
			int(*fields.reviewStatus), id); err != nil {
			return fmt.Errorf("update review status: %w", err)
		}
	}

	if fields.tagSlugsSet {
		tagIDs, err := s.resolveTagSlugs(ctx, fields.tagSlugs)
		if err != nil {
			return err
		}
		if err := replaceImageLinks(ctx, tx, id, "ImageTag", "TagsId", tagIDs); err != nil {
			return err
		}
	}

	if fields.artistIDsSet {
		artistIDs, err := s.resolveArtistIDs(ctx, fields.artistIDs)
		if err != nil {
			return err
		}
		if err := replaceImageLinks(ctx, tx, id, "ArtistImage", "ArtistsId", artistIDs); err != nil {
			return err
		}
	}

	// Accepting from the moderation screen rather than the queue has to close
	// the queue entry too, or the same upload stays on the list for the next
	// moderator to approve again.
	if fields.reviewStatus != nil && *fields.reviewStatus == models.ReviewAccepted {
		if _, err := tx.Exec(ctx, `
            UPDATE "ReviewTasks"
            SET "Status" = $1, "ResolvedAt" = now()
            WHERE "Kind" = $2 AND "TargetType" = $3 AND "TargetId" = $4 AND "Status" = $5`,
			int(models.TaskApproved), int(models.KindNewContent),
			int(models.TargetImage), id, int(models.TaskPending)); err != nil {
			return fmt.Errorf("close review task: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit image update: %w", err)
	}

	// Storage and cache work happens after the commit, so a storage failure
	// cannot roll back metadata that is already correct. The orphaned old object
	// becomes visible to storage-diff, which is where a genuinely unreferenced
	// file belongs anyway.
	if len(fields.replacement) > 0 {
		if err := s.storage.Put(context.WithoutCancel(ctx), storageKey(id, oldExtension),
			bytes.NewReader(storedReplacement), storedReplacementType); err != nil {
			return fmt.Errorf("store replacement: %w", err)
		}
		s.purger.PurgeImage(context.WithoutCancel(ctx), id, []string{image.Extension, oldExtension})
	}

	return nil
}

// resolveTagSlugs maps every slug or returns an error naming the ones missing.
//
// C# compares foundTags.Count against trimmedSlugs.Count and reports the
// difference, so one bad slug in a list of ten rejects the whole patch.
func (s *Server) resolveTagSlugs(ctx context.Context, slugs []string) ([]int64, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	found, err := s.repo.GetTagsBySlugs(ctx, slugs)
	if err != nil {
		return nil, fmt.Errorf("resolve tag slugs: %w", err)
	}

	bySlug := make(map[string]int64, len(found))
	for _, tag := range found {
		bySlug[strings.ToLower(tag.Slug)] = tag.ID
	}

	ids := make([]int64, 0, len(slugs))
	var missing []string
	for _, slug := range slugs {
		id, ok := bySlug[strings.ToLower(slug)]
		if !ok {
			missing = append(missing, slug)
			continue
		}
		ids = append(ids, id)
	}
	if len(missing) > 0 {
		return nil, validationError("Tags",
			"Unknown tag slug(s): "+strings.Join(missing, ", ")+".")
	}
	return ids, nil
}

// resolveArtistIDs mirrors resolveTagSlugs for artists, keyed by id.
func (s *Server) resolveArtistIDs(ctx context.Context, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	found, err := s.repo.GetArtistsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve artist ids: %w", err)
	}

	present := make(map[int64]bool, len(found))
	for _, artist := range found {
		present[artist.ID] = true
	}

	var missing []string
	resolved := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !present[id] {
			missing = append(missing, strconv.FormatInt(id, 10))
			continue
		}
		resolved = append(resolved, id)
	}
	if len(missing) > 0 {
		return nil, validationError("Artists",
			"Unknown artist id(s): "+strings.Join(missing, ", ")+".")
	}
	return resolved, nil
}

// replaceImageLinks swaps an image's membership of a join table.
//
// Deleting and re-inserting inside the caller's transaction is what C#
// image.Tags.Clear() followed by a re-add does, and it is why the link set can
// be written in one pass instead of a read-modify-write.
func replaceImageLinks(ctx context.Context, tx repo.Tx, imageID int64,
	table, column string, ids []int64) error {
	if _, err := tx.Exec(ctx,
		fmt.Sprintf(`DELETE FROM %q WHERE "ImagesId" = $1`, table), imageID); err != nil {
		return fmt.Errorf("clear %s: %w", table, err)
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %q ("ImagesId", %q) VALUES ($1, $2)`, table, column),
			imageID, id); err != nil {
			return fmt.Errorf("link %s %d: %w", table, id, err)
		}
	}
	return nil
}

// writeImageUpdateError maps a patch failure onto the right status.
//
// The distinction that matters is 404 for a missing image or a missing
// referenced row, and 400 for a validation failure raised inside the command.
func writeImageUpdateError(w http.ResponseWriter, r *http.Request, id int64, err error) {
	var apiErr *apierror.Error
	if errors.As(err, &apiErr) {
		apiErr.Write(w, r.URL.Path)
		return
	}
	if errors.Is(err, repo.ErrNotFound) {
		apierror.NotFound("Image with ID "+strconv.FormatInt(id, 10)+" not found.").
			Write(w, r.URL.Path)
		return
	}

	// A bad uploader reference is the one 404 case UpdateImageCommand raises
	// through the command rather than the controller.
	if strings.Contains(err.Error(), "unknown tag slug") ||
		strings.Contains(err.Error(), "unknown artist id") {
		writeAPIError(w, r, err)
		return
	}

	internalServerError(w, r)
}
