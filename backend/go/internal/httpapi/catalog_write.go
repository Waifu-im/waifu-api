package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/store/models"
)

// registerCatalogWriteRoutes mounts the tag and artist mutations.
//
// The permission split is not uniform across the three verbs, and that is
// deliberate in the C# service rather than an oversight:
//
//   - POST checks the configured Permissions__*CreationMinRole, which may be as
//     low as User.
//   - PATCH is Moderator+, because editing published content is moderation work.
//   - DELETE is Admin+, because it cascades: removing a tag strips it from every
//     image carrying it.
//
// Each verb therefore uses a different guard, and conflating them would either
// let ordinary users edit the catalogue or stop trusted users contributing to it.
func (s *Server) registerCatalogWriteRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("POST "+base+"tags", s.handleCreateTag)
	mux.HandleFunc("PATCH "+base+"tags/{id}", s.handleUpdateTag)
	mux.HandleFunc("DELETE "+base+"tags/{id}", s.handleDeleteTag)

	mux.HandleFunc("POST "+base+"artists", s.handleCreateArtist)
	mux.HandleFunc("PATCH "+base+"artists/{id}", s.handleUpdateArtist)
	mux.HandleFunc("DELETE "+base+"artists/{id}", s.handleDeleteArtist)
}

// Field length limits, taken from the FluentValidation rules on the commands.
//
// They are duplicated here rather than derived because FluentValidation has no
// Go equivalent in this codebase, and a message that says "must not exceed 30
// characters" without enforcing it is worse than not having the message.
const (
	maxNameLength        = 30
	maxDescriptionLength = 150
	maxLinkLength        = 200
	maxSlugLength        = 50
)

// validationError renders a FluentValidation-shaped failure.
//
// The C# service returns a ValidationProblemDetails: a 400 whose body carries an
// "errors" object keyed by field name. The frontend reads that object to attach
// messages to inputs, so the shape is part of the contract.
func validationError(field, message string) *apierror.Error {
	return apierror.BadRequest(message).
		WithExtra("errors", map[string][]string{field: {message}})
}

// validationErrors renders several field failures at once, which is what
// FluentValidation does when more than one rule fails.
func validationErrors(failures map[string]string) *apierror.Error {
	grouped := make(map[string][]string, len(failures))
	first := ""
	for field, message := range failures {
		grouped[field] = []string{message}
		if first == "" {
			first = message
		}
	}
	return apierror.BadRequest(first).WithExtra("errors", grouped)
}

// createTagRequest is the body of POST /tags.
type createTagRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
}

// handleCreateTag serves POST /tags.
//
// A successful creation is 201 with a Location header pointing at the new tag,
// and always enqueues a review task. The task is what a moderator acts on; when
// the creator bypasses review the task is created already approved, so the queue
// keeps a record either way.
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
	description := strings.TrimSpace(body.Description)

	if failures := validateTagFields(name, description, body.Slug); len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	// An explicit slug wins; otherwise it is derived from the name. Either way it
	// goes through the same normalisation, so "Waifu Girl!" and "waifu-girl"
	// cannot both exist.
	slug := name
	if strings.TrimSpace(body.Slug) != "" {
		slug = body.Slug
	}
	slug = slugify(slug)
	if slug == "" {
		writeAPIError(w, r, validationError("Name", "Name must contain at least one letter or digit."))
		return
	}

	if existing, err := s.repo.FindTagByNameOrSlug(r.Context(), name, slug); err == nil {
		if existing.Slug == slug {
			writeAPIError(w, r, apierror.Conflict("Tag with slug '"+slug+"' already exists."))
		} else {
			writeAPIError(w, r, apierror.Conflict("Tag with name '"+name+"' already exists."))
		}
		return
	} else if !isNoRows(err) {
		s.logError(r, "check tag conflict", err)
		internalServerError(w, r)
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

	tag, err := s.repo.CreateTag(r.Context(), name, slug, description,
		&caller.UserID, reviewStatusFor(requiresReview))
	if err != nil {
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

	location := s.baseLocationPrefix() + "tags/" + itoa64(tag.ID)
	s.writeCreated(w, r, TagDto{
		ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
		ReviewStatus: strPtr(tag.ReviewStatus.String()), CreatorID: tag.CreatorID,
	}, location)
}

// validateTagFields applies the CreateTag/UpdateTag validator rules.
func validateTagFields(name, description, slug string) map[string]string {
	failures := map[string]string{}

	if name == "" {
		failures["Name"] = "Name is required."
	} else if len([]rune(name)) > maxNameLength {
		failures["Name"] = "Name must not exceed 30 characters."
	}

	if description == "" {
		failures["Description"] = "Description is required."
	} else if len([]rune(description)) > maxDescriptionLength {
		failures["Description"] = "Description must not exceed 150 characters."
	}

	if trimmed := strings.TrimSpace(slug); trimmed != "" {
		if len(trimmed) > maxSlugLength {
			failures["Slug"] = "Slug must not exceed 50 characters."
		} else if !isValidSlug(trimmed) {
			failures["Slug"] = "Slug can only contain lowercase letters, numbers, and hyphens."
		}
	}

	return failures
}

// isValidSlug mirrors the ^[a-z0-9-]+$ rule in the validators.
func isValidSlug(value string) bool {
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '-':
		default:
			return false
		}
	}
	return value != ""
}

// reviewStatusInput accepts a ReviewStatus in either of the forms ASP.NET Core
// accepts.
//
// Program.cs registers a bare JsonStringEnumConverter, whose AllowIntegerValues
// defaults to true, so the C# service deserialises `"reviewStatus": "Accepted"`
// and `"reviewStatus": 1` equally well. A plain *string here would reject the
// numeric form with a 400 that the original never produces.
//
// The numeric form is the enum's underlying value, which is Pending = 0 and
// Accepted = 1; there is no other member.
type reviewStatusInput struct {
	value models.ReviewStatus
	set   bool
}

// UnmarshalJSON accepts a JSON string, a JSON number, or null.
func (r *reviewStatusInput) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		return nil
	}

	if trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		status, ok := models.ParseReviewStatus(text)
		if !ok {
			return fmt.Errorf("unknown review status %q", text)
		}
		r.value, r.set = status, true
		return nil
	}

	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("review status must be a string or a number: %w", err)
	}
	switch models.ReviewStatus(number) {
	case models.ReviewPending, models.ReviewAccepted:
		r.value, r.set = models.ReviewStatus(number), true
		return nil
	default:
		return fmt.Errorf("unknown review status %d", number)
	}
}

// updateTagRequest is the body of PATCH /tags/{id}.
type updateTagRequest struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Slug         string             `json:"slug"`
	ReviewStatus *reviewStatusInput `json:"reviewStatus"`
	CreatorID    *int64             `json:"creatorId"`
}

// handleUpdateTag serves PATCH /tags/{id}.
func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleModerator, "edit tags") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateTagRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	description := strings.TrimSpace(body.Description)
	if failures := validateTagFields(name, description, body.Slug); len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	if taken, err := s.repo.TagNameTaken(r.Context(), id, name); err != nil {
		s.logError(r, "check tag name", err)
		internalServerError(w, r)
		return
	} else if taken {
		writeAPIError(w, r, apierror.Conflict("Tag with name '"+name+"' already exists."))
		return
	}

	// An empty slug leaves the existing one alone rather than clearing it.
	slug := slugify(strings.TrimSpace(body.Slug))

	updated, err := s.repo.UpdateTag(r.Context(), id, name, slug, description)
	if err != nil {
		writeRepoError(w, r, "update tag", err, "Tag with ID "+itoa64(id)+" not found.")
		return
	}

	// A moderator may set the review status directly. Accepting a tag closes its
	// open queue task so the same submission does not appear twice in the queue.
	if body.ReviewStatus != nil && body.ReviewStatus.set {
		status, ok := body.ReviewStatus.value, true
		if !ok {
			writeAPIError(w, r, validationError("ReviewStatus",
				"'reviewStatus' must be Pending or Accepted."))
			return
		}
		if err := s.repo.SetTagReviewStatus(r.Context(), id, status); err != nil {
			writeRepoError(w, r, "set tag review status", err, "Tag not found.")
			return
		}
		if status == models.ReviewAccepted {
			if err := s.repo.ResolvePendingTask(r.Context(), models.KindNewContent,
				models.TargetTag, id); err != nil {
				s.logError(r, "resolve tag review task", err)
			}
		}
		updated.ReviewStatus = status
	}

	if body.CreatorID != nil {
		if err := s.repo.SetTagCreator(r.Context(), id, *body.CreatorID); err != nil {
			s.logError(r, "set tag creator", err)
		}
		updated.CreatorID = body.CreatorID
	}

	writeJSON(w, http.StatusOK, TagDto{
		ID: updated.ID, Name: updated.Name, Slug: updated.Slug,
		Description:  updated.Description,
		ReviewStatus: strPtr(updated.ReviewStatus.String()),
		CreatorID:    updated.CreatorID,
	})
}

// handleDeleteTag serves DELETE /tags/{id}.
func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "delete tags") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.DeleteTag(r.Context(), id); err != nil {
		writeRepoError(w, r, "delete tag", err, "Tag with ID "+itoa64(id)+" not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// createArtistRequest is the body of POST /artists.
type createArtistRequest struct {
	Name       string `json:"name"`
	Patreon    string `json:"patreon"`
	Pixiv      string `json:"pixiv"`
	Twitter    string `json:"twitter"`
	DeviantArt string `json:"deviantArt"`
}

// handleCreateArtist serves POST /artists.
//
// Unlike tags, an artist is identified by its links as much as its name: the
// same person appears on Pixiv and Patreon with different spellings, so any
// shared field is treated as a duplicate.
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
	if failures := validateArtistFields(name, body.Patreon, body.Pixiv, body.Twitter, body.DeviantArt); len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	artist := models.Artist{
		Name:         name,
		Patreon:      optionalString(body.Patreon),
		Pixiv:        optionalString(body.Pixiv),
		Twitter:      optionalString(body.Twitter),
		DeviantArt:   optionalString(body.DeviantArt),
		CreatorID:    &caller.UserID,
		ReviewStatus: reviewStatusFor(s.authz.RequiresReviewForArtist(roleOf(caller))),
	}

	if conflict := s.artistConflict(w, r, 0, artist); conflict != nil {
		writeAPIError(w, r, conflict)
		return
	}

	requiresReview := s.authz.RequiresReviewForArtist(roleOf(caller))

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

	location := s.baseLocationPrefix() + "artists/" + itoa64(created.ID)
	s.writeCreated(w, r, toArtistDto(created), location)
}

// updateArtistRequest is the body of PATCH /artists/{id}.
type updateArtistRequest struct {
	Name         string             `json:"name"`
	Patreon      string             `json:"patreon"`
	Pixiv        string             `json:"pixiv"`
	Twitter      string             `json:"twitter"`
	DeviantArt   string             `json:"deviantArt"`
	ReviewStatus *reviewStatusInput `json:"reviewStatus"`
	CreatorID    *int64             `json:"creatorId"`
}

// handleUpdateArtist serves PATCH /artists/{id}.
func (s *Server) handleUpdateArtist(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleModerator, "edit artists") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateArtistRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	name := strings.TrimSpace(body.Name)
	if failures := validateArtistFields(name, body.Patreon, body.Pixiv, body.Twitter, body.DeviantArt); len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	artist := models.Artist{
		ID:         id,
		Name:       name,
		Patreon:    optionalString(body.Patreon),
		Pixiv:      optionalString(body.Pixiv),
		Twitter:    optionalString(body.Twitter),
		DeviantArt: optionalString(body.DeviantArt),
	}

	if conflict := s.artistConflict(w, r, id, artist); conflict != nil {
		writeAPIError(w, r, conflict)
		return
	}

	updated, err := s.repo.UpdateArtist(r.Context(), artist)
	if err != nil {
		writeRepoError(w, r, "update artist", err, "Artist with ID "+itoa64(id)+" not found.")
		return
	}

	if body.ReviewStatus != nil && body.ReviewStatus.set {
		status, ok := body.ReviewStatus.value, true
		if !ok {
			writeAPIError(w, r, validationError("ReviewStatus",
				"'reviewStatus' must be Pending or Accepted."))
			return
		}
		if err := s.repo.SetArtistReviewStatus(r.Context(), id, status); err != nil {
			writeRepoError(w, r, "set artist review status", err, "Artist not found.")
			return
		}
		if status == models.ReviewAccepted {
			if err := s.repo.ResolvePendingTask(r.Context(), models.KindNewContent,
				models.TargetArtist, id); err != nil {
				s.logError(r, "resolve artist review task", err)
			}
		}
		updated.ReviewStatus = status
	}

	if body.CreatorID != nil {
		if err := s.repo.SetArtistCreator(r.Context(), id, *body.CreatorID); err != nil {
			s.logError(r, "set artist creator", err)
		}
		updated.CreatorID = body.CreatorID
	}

	writeJSON(w, http.StatusOK, toArtistDto(updated))
}

// handleDeleteArtist serves DELETE /artists/{id}.
func (s *Server) handleDeleteArtist(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "delete artists") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	if err := s.repo.DeleteArtist(r.Context(), id); err != nil {
		writeRepoError(w, r, "delete artist", err, "Artist with ID "+itoa64(id)+" not found.")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateArtistFields applies the CreateArtist/UpdateArtist validator rules.
func validateArtistFields(name, patreon, pixiv, twitter, deviantArt string) map[string]string {
	failures := map[string]string{}

	if name == "" {
		failures["Name"] = "Name is required."
	} else if len([]rune(name)) > maxNameLength {
		failures["Name"] = "Name must not exceed 30 characters."
	}

	links := []struct{ field, value, label string }{
		{"Patreon", patreon, "Patreon link"},
		{"Pixiv", pixiv, "Pixiv link"},
		{"Twitter", twitter, "Twitter link"},
		{"DeviantArt", deviantArt, "DeviantArt link"},
	}
	for _, link := range links {
		trimmed := strings.TrimSpace(link.value)
		if trimmed == "" {
			continue
		}
		if len([]rune(trimmed)) > maxLinkLength {
			failures[link.field] = link.label + " must not exceed 200 characters."
			continue
		}
		if !isAbsoluteURL(trimmed) {
			failures[link.field] = link.label + " must be a valid URL."
		}
	}

	return failures
}

// isAbsoluteURL mirrors Uri.TryCreate(url, UriKind.Absolute): a scheme is
// required, and a scheme alone counts.
func isAbsoluteURL(value string) bool {
	scheme, _, found := strings.Cut(value, ":")
	if !found || scheme == "" {
		return false
	}
	for i, char := range scheme {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9', char == '+', char == '-', char == '.':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// artistConflict returns the first colliding field as a 409, or nil when the
// artist is free.
//
// The order is fixed — name, then Patreon, Pixiv, Twitter, DeviantArt — so the
// same pair of inputs always produces the same message. A repository failure
// is logged and turned into a 500 by the caller's own write, so this stays a
// pure "is there a conflict" question.
func (s *Server) artistConflict(w http.ResponseWriter, r *http.Request, excludeID int64, artist models.Artist) *apierror.Error {
	existing, err := s.repo.FindArtistConflict(r.Context(), excludeID, artist)
	if err != nil {
		if !isNoRows(err) {
			s.logError(r, "check artist conflict", err)
		}
		return nil
	}

	if strings.EqualFold(existing.Name, artist.Name) {
		return apierror.Conflict("Artist with name '" + artist.Name + "' already exists.")
	}
	if artist.Patreon != nil && existing.Patreon != nil && *artist.Patreon == *existing.Patreon {
		return apierror.Conflict("Artist with Patreon '" + *artist.Patreon + "' already exists.")
	}
	if artist.Pixiv != nil && existing.Pixiv != nil && *artist.Pixiv == *existing.Pixiv {
		return apierror.Conflict("Artist with Pixiv '" + *artist.Pixiv + "' already exists.")
	}
	if artist.Twitter != nil && existing.Twitter != nil && *artist.Twitter == *existing.Twitter {
		return apierror.Conflict("Artist with Twitter '" + *artist.Twitter + "' already exists.")
	}
	if artist.DeviantArt != nil && existing.DeviantArt != nil && *artist.DeviantArt == *existing.DeviantArt {
		return apierror.Conflict("Artist with DeviantArt '" + *artist.DeviantArt + "' already exists.")
	}
	return nil
}

// optionalString returns nil for an empty or whitespace-only value, so an
// omitted field and an empty string are stored the same way.
//
// This is StringExtensions.ToNullIfEmpty, which every create/update command uses
// before writing an artist link.
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// strPtr returns a pointer to a copy of value.
func strPtr(value string) *string { return &value }

// ensure url stays imported for the escape helpers used by callers of this file.
var _ = url.PathEscape
