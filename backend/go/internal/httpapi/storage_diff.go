package httpapi

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// registerStorageDiffRoutes mounts the storage reconciliation endpoints.
//
// The whole controller is Admin-only in the C# service, not Moderator: deleting
// catalogue rows or stored objects on the strength of a listing is not a
// moderation decision, it is an operator one.
func (s *Server) registerStorageDiffRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"storage-diff", s.handleStorageDiff)
	mux.HandleFunc("DELETE "+base+"storage-diff/db", s.handleStorageDiffDeleteDB)
	mux.HandleFunc("DELETE "+base+"storage-diff/s3", s.handleStorageDiffDeleteS3)
}

// diffMode selects which direction of the drift to report.
//
// The values are the C# enum names because they travel in the query string and
// the frontend sends them verbatim. Getting this wrong is not a subtle
// failure: an unrecognised mode is a 400, so every storage-diff page would fail
// to load.
type diffMode string

const (
	// ModeOrphans reports objects in the bucket with no catalogue row.
	ModeOrphans diffMode = "OrphansInS3"

	// ModeMissing reports catalogue rows with no object.
	ModeMissing diffMode = "MissingFromS3"
)

// parseDiffMode reads the mode parameter.
func parseDiffMode(raw string) (diffMode, error) {
	switch diffMode(raw) {
	case ModeOrphans, ModeMissing:
		return diffMode(raw), nil
	default:
		return "", apierror.BadRequest("'mode' must be OrphansInS3 or MissingFromS3.")
	}
}

// healthcheckPrefix marks objects that are deliberately not catalogue entries.
//
// The bucket holds a healthcheck file used to verify connectivity. It is always
// an "orphan" by the strict definition, and listing it on every page would
// train the operator to ignore the screen.
const healthcheckPrefix = "healthcheck/"

// storageDiffDto is one discrepant entry.
//
// It is StorageDiffDto.cs: id and fileName identify the entry, and image carries
// the full record for the MissingFromS3 case so the operator can see what they
// are about to delete without opening it in another tab.
type storageDiffDto struct {
	ID       *int64    `json:"id"`
	FileName string    `json:"fileName"`
	Image    *ImageDto `json:"image"`
}

// handleStorageDiff serves GET /storage-diff.
func (s *Server) handleStorageDiff(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "reconcile object storage") {
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.StorageDiff)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	mode, err := parseDiffMode(r.URL.Query().Get("mode"))
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	dtos, err := s.computeStorageDiff(r, mode)
	if err != nil {
		s.logError(r, "storage diff", err)
		internalServerError(w, r)
		return
	}

	// Paginated in memory. The C# service does the same, because half of the
	// comparison is a bucket listing that cannot be expressed as a SQL join. A
	// drift large enough for this to hurt is an emergency, not a steady state.
	total := len(dtos)
	start := page.offset()
	if start > total {
		start = total
	}
	end := start + page.PageSize
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos[start:end], total))
}

// computeStorageDiff reconciles the bucket against the catalogue.
func (s *Server) computeStorageDiff(r *http.Request, mode diffMode) ([]storageDiffDto, error) {
	expected, err := s.repo.ListCatalogueKeys(r.Context())
	if err != nil {
		return nil, err
	}

	ctx := contextWithoutCancel(r)
	stored, err := s.storage.List(ctx)
	if err != nil {
		return nil, err
	}

	present := make(map[string]bool, len(stored))
	for _, key := range stored {
		if strings.HasPrefix(key, healthcheckPrefix) {
			continue
		}
		present[key] = true
	}

	if mode == ModeOrphans {
		entries := make([]storageDiffDto, 0, 16)
		for _, key := range stored {
			if strings.HasPrefix(key, healthcheckPrefix) {
				continue
			}
			if _, known := expected[key]; !known {
				entries = append(entries, storageDiffDto{FileName: key})
			}
		}
		// Sorted so paging is stable: an unsorted listing would let the same
		// orphan appear on page 1 and then again on page 2.
		sort.Slice(entries, func(i, j int) bool { return entries[i].FileName < entries[j].FileName })
		return entries, nil
	}

	// MissingFromS3: catalogue rows whose object is gone.
	//
	// expected is keyed by object name; the sort makes paging stable, which
	// matters because the in-memory page is recomputed on every request and an
	// unstable order would let a row appear on two pages.
	keys := make([]string, 0, len(expected))
	for key := range expected {
		if !present[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	entries := make([]storageDiffDto, 0, 16)
	for _, key := range keys {
		entries = append(entries, storageDiffDto{ID: pointerTo(expected[key]), FileName: key})
	}

	imageIDs := make([]int64, 0, len(entries))
	for _, entry := range entries {
		imageIDs = append(imageIDs, *entry.ID)
	}

	images, err := s.repo.GetImagesByIDs(r.Context(), imageIDs,
		repo.ChildVisibility{Filter: models.FilterAll}, true)
	if err != nil {
		return nil, err
	}
	imageDtos, err := s.toImageDtos(r, images, repo.ImageFilter{ReviewStatus: models.FilterAll}, 0)
	if err != nil {
		return nil, err
	}
	imagesByID := make(map[int64]ImageDto, len(imageDtos))
	for _, dto := range imageDtos {
		imagesByID[dto.ID] = dto
	}

	for i := range entries {
		if dto, ok := imagesByID[*entries[i].ID]; ok {
			entries[i].Image = &dto
		}
	}
	return entries, nil
}

// pointerTo returns a pointer to a copy of value.
//
// StorageDiffDto.Id is nullable because an orphaned object has no row id, so the
// field genuinely is two states rather than always present.
func pointerTo(value int64) *int64 { return &value }

// deleteDBRequest is the body of DELETE /storage-diff/db.
type deleteDBRequest struct {
	ImageIDs []int64 `json:"imageIds"`
}

// handleStorageDiffDeleteDB serves DELETE /storage-diff/db.
//
// Rows are removed; the objects are left alone, because an operator reaching for
// this screen has already established the files are gone. Any pending report
// against a deleted image is resolved rather than left dangling, so the reporter
// is not left waiting on a decision about an image that no longer exists.
func (s *Server) handleStorageDiffDeleteDB(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "delete catalogue records") {
		return
	}

	var body deleteDBRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if len(body.ImageIDs) == 0 {
		writeAPIError(w, r, validationError("ImageIds", "'imageIds' is required."))
		return
	}

	if _, err := s.repo.DeleteImages(r.Context(), body.ImageIDs); err != nil {
		s.logError(r, "delete db rows", err)
		internalServerError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteS3Request is the body of DELETE /storage-diff/s3.
type deleteS3Request struct {
	FileNames []string `json:"fileNames"`
}

// handleStorageDiffDeleteS3 serves DELETE /storage-diff/s3.
func (s *Server) handleStorageDiffDeleteS3(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "delete stored objects") {
		return
	}

	var body deleteS3Request
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if len(body.FileNames) == 0 {
		writeAPIError(w, r, validationError("FileNames", "'fileNames' is required."))
		return
	}

	var deleted []string
	for _, name := range body.FileNames {
		// Rejecting traversal is worth keeping even though the C# service does
		// not: the key comes straight from a request body, and ".." would
		// otherwise address something outside the bucket's image namespace.
		if !isSafeObjectKey(name) {
			s.log.Info("refused unsafe object key", "key", name)
			continue
		}
		if err := s.storage.Delete(contextWithoutCancel(r), name); err != nil {
			s.logError(r, "delete object", err)
			continue
		}
		deleted = append(deleted, name)
	}

	// The edge would keep serving the deleted objects until its TTL expired, so
	// the purge is what makes the deletion visible.
	s.purgeFiles(contextWithoutCancel(r), deleted)

	w.WriteHeader(http.StatusNoContent)
}

// purgeFiles invalidates the edge cache for a list of object names.
func (s *Server) purgeFiles(ctx context.Context, names []string) {
	if s.purger == nil || len(names) == 0 {
		return
	}
	s.purger.PurgeFiles(ctx, names)
}

// isSafeObjectKey rejects keys that could address outside the image namespace.
//
// The check permits dots because image objects are named "{id}{extension}" —
// "1234.png" — so rejecting dots would reject every legitimate key. What it
// refuses is traversal and separators, which no image key contains.
func isSafeObjectKey(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, char := range name {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}
