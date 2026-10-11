package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// nowRFC3339 is the timestamp format used for the nullable ResolvedAt column.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// This file ports EditPreparer and EditApplier, the two services behind
// POST /review/tasks and PATCH /review/tasks/{id}.
//
// They are the most intricate business logic in the C# codebase — 335 lines that
// mutate four entity types behind a diff — and PLAN.md §6.2 calls out reading
// them as the actual work rather than translating them. Two properties are
// load-bearing and easy to lose:
//
//   - The payload is a *delta*, not a replacement. A stored payload only
//     contains the fields the user actually changed, so two people proposing
//     different edits to the same image do not overwrite each other.
//   - Unknown members are rejected. The payload arrives from a browser and is
//     written straight to published content, so accepting an unrecognised field
//     would be a privilege-escalation hole: a client could smuggle reviewStatus
//     or uploaderId through a tag rename.

// imageEditPayload is the proposed change to an image.
//
// Source is a pointer because nil means "leave unchanged" while an empty string
// means "clear it" — the distinction the frontend relies on to remove a bad
// attribution.
type imageEditPayload struct {
	Source          *string   `json:"source,omitempty"`
	IsNsfw          *bool     `json:"isNsfw,omitempty"`
	AddTagSlugs     *[]string `json:"addTagSlugs,omitempty"`
	RemoveTagSlugs  *[]string `json:"removeTagSlugs,omitempty"`
	AddArtistIDs    *[]int64  `json:"addArtistIds,omitempty"`
	RemoveArtistIDs *[]int64  `json:"removeArtistIds,omitempty"`
}

// tagEditPayload is the proposed change to a tag.
//
// Slug is deliberately absent: it is part of the public API surface, so renaming
// it would break every existing integration that addresses the tag by slug.
type tagEditPayload struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// artistEditPayload is the proposed change to an artist.
type artistEditPayload struct {
	Name       *string `json:"name,omitempty"`
	Patreon    *string `json:"patreon,omitempty"`
	Pixiv      *string `json:"pixiv,omitempty"`
	Twitter    *string `json:"twitter,omitempty"`
	DeviantArt *string `json:"deviantArt,omitempty"`
}

// editError distinguishes the three ways a proposal can be refused, because each
// maps to a different status code.
type editError struct {
	kind   string // "invalid", "notFound", "conflict"
	detail string
}

func (e *editError) Error() string { return e.detail }

func invalidEdit(detail string) error { return &editError{kind: "invalid", detail: detail} }
func missingEdit(detail string) error { return &editError{kind: "notFound", detail: detail} }
func conflictingEdit(detail string) error {
	return &editError{kind: "conflict", detail: detail}
}

// writeEditError renders an editError as the right ProblemDetails status.
//
// A non-edit error means the failure was infrastructural rather than a bad
// proposal, so it falls through to a 500 and is logged by the caller's caller.
func writeEditError(w http.ResponseWriter, r *http.Request, err error) {
	var edit *editError
	if !errors.As(err, &edit) {
		internalServerError(w, r)
		return
	}
	switch edit.kind {
	case "notFound":
		apierror.NotFound(edit.detail).Write(w, r.URL.Path)
	case "conflict":
		apierror.Conflict(edit.detail).Write(w, r.URL.Path)
	default:
		apierror.BadRequest(edit.detail).Write(w, r.URL.Path)
	}
}

// decodeStrict unmarshals a payload, rejecting unknown members.
//
// This is the security boundary described at the top of the file. Go's default
// unmarshal silently ignores unknown fields, which is the opposite of what the
// C# service does, so the check is explicit and manual.
func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return invalidEdit("Invalid payload: " + cleanJSONError(err))
	}
	return nil
}

// cleanJSONError trims the Go decoder's noise so the message reads like the
// System.Text.Json one.
func cleanJSONError(err error) string {
	message := err.Error()
	if index := strings.Index(message, "json: unknown field "); index >= 0 {
		return strings.TrimSpace(message[index+len("json: unknown field "):]) +
			" is not a recognised field."
	}
	return message
}

// submitEditRequest is the POST /review/tasks body.
type submitEditRequest struct {
	TargetType string          `json:"targetType"`
	TargetID   int64           `json:"targetId"`
	Reason     *string         `json:"reason"`
	Payload    json.RawMessage `json:"payload"`
}

// handleSubmitEdit serves POST /review/tasks.
//
// It creates an Edit task, or applies the change immediately and returns an
// already-approved task when the caller's role bypasses edit review.
func (s *Server) handleSubmitEdit(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionSubmitEdit) {
		return
	}
	caller := s.principal(r)

	var body submitEditRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	targetType, err := models.ParseReviewableType(body.TargetType)
	if err != nil {
		writeAPIError(w, r, validationError("TargetType", err.Error()))
		return
	}
	if body.TargetID <= 0 {
		writeAPIError(w, r, validationError("TargetId", "Invalid target ID."))
		return
	}
	if len(body.Payload) == 0 || string(body.Payload) == "null" {
		writeAPIError(w, r, validationError("Payload", "Proposed changes are required."))
		return
	}

	var reason *string
	if body.Reason != nil {
		reason = optionalString(*body.Reason)
		if reason != nil && len([]rune(*reason)) > maxEditReasonLength {
			writeAPIError(w, r, validationError("Reason",
				"Reason must not exceed 500 characters."))
			return
		}
	}

	// One open edit per (user, target). Without this a single user could queue
	// the same change repeatedly and the moderator would see it several times.
	open, err := s.repo.HasOpenEditForTarget(r.Context(), caller.UserID, targetType, body.TargetID)
	if err != nil {
		s.logError(r, "check open edit", err)
		internalServerError(w, r)
		return
	}
	if open {
		writeAPIError(w, r, apierror.Conflict(
			"You already have a pending edit for this item. Edit or cancel it first."))
		return
	}

	if limit := s.cfg.Review.MaxOpenEditsPerUser; limit > 0 {
		count, err := s.repo.CountOpenEdits(r.Context(), caller.UserID)
		if err != nil {
			s.logError(r, "count open edits", err)
			internalServerError(w, r)
			return
		}
		if count >= int64(limit) {
			writeAPIError(w, r, apierror.Conflict(
				"You have too many pending edits (limit "+itoa64(int64(limit))+
					"). Please wait for some to be reviewed."))
			return
		}
	}

	normalized, target, err := s.prepareEdit(r, targetType, body.TargetID, body.Payload)
	if err != nil {
		writeEditError(w, r, err)
		return
	}

	requiresReview := s.authz.RequiresReviewForEdit(roleOf(caller), targetType)

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin submit edit", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	status := models.TaskPending
	var resolvedAt *string
	reviewerNote := ""
	if !requiresReview {
		status = models.TaskApproved
		now := nowRFC3339()
		resolvedAt = &now
		reviewerNote = "Auto-applied (trusted role)."
	}

	taskID, err := s.repo.CreateEditTask(r.Context(), tx, targetType, body.TargetID,
		caller.UserID, normalized, reason, status, reviewerNote, resolvedAt)
	if err != nil {
		s.logError(r, "insert edit task", err)
		internalServerError(w, r)
		return
	}

	if !requiresReview {
		if err := s.applyEdit(r.Context(), tx, targetType, body.TargetID, normalized); err != nil {
			s.logError(r, "apply edit", err)
			internalServerError(w, r)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit submit edit", err)
		internalServerError(w, r)
		return
	}

	dto, err := s.buildReviewTaskDto(r, taskID, target)
	if err != nil {
		s.logError(r, "build review task", err)
		internalServerError(w, r)
		return
	}

	location := s.baseLocationPrefix() + "review/tasks/" + itoa64(taskID)
	s.writeCreated(w, r, dto, location)
}

// maxEditReasonLength is the validator's limit on the reason field.
const maxEditReasonLength = 500

// editReviewTaskRequest is the PATCH /review/tasks/{id} body.
type editReviewTaskRequest struct {
	Content json.RawMessage `json:"content"`
}

// handleEditReviewTask serves PATCH /review/tasks/{id}.
//
// This edits the *content* of a task and never its status. For an Edit task the
// content is the proposed change, which is re-validated against the current
// target; for a NewContent task it is the submitted entity's editable fields,
// which are written straight to the still-pending row.
func (s *Server) handleEditReviewTask(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body editReviewTaskRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	content := body.Content
	if len(content) == 0 || string(content) == "null" {
		content = json.RawMessage("{}")
	}

	task, err := s.repo.GetReviewTask(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get review task", err,
			"Review task with ID "+itoa64(id)+" not found.")
		return
	}

	if (task.SubmitterID == nil || *task.SubmitterID != caller.UserID) && !caller.CanModerate() {
		writeAPIError(w, r, apierror.Forbidden("You can only edit your own submissions."))
		return
	}
	if task.Status != models.TaskPending {
		writeAPIError(w, r, apierror.Conflict("This review task has already been resolved."))
		return
	}

	var target *reviewTargetDto
	var stored []byte

	if task.Kind == models.KindEdit {
		normalized, prepared, err := s.prepareEdit(r, task.TargetType, task.TargetId, content)
		if err != nil {
			writeEditError(w, r, err)
			return
		}
		stored = normalized
		target = prepared
	} else {
		applied, err := s.applyNewContent(r, task, content)
		if err != nil {
			writeEditError(w, r, err)
			return
		}
		target = applied
	}

	// A moderator correcting someone else's submission leaves a marker, so the
	// submitter can see that what they sent is not what is queued.
	moderatorEdit := caller.CanModerate() &&
		(task.SubmitterID == nil || *task.SubmitterID != caller.UserID)

	if err := s.repo.UpdateReviewTaskContent(r.Context(), id, stored, moderatorEdit); err != nil {
		s.logError(r, "update review task content", err)
		internalServerError(w, r)
		return
	}

	dto, err := s.buildReviewTaskDto(r, id, target)
	if err != nil {
		s.logError(r, "build review task", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dto)
}

// prepareEdit validates and normalises a proposed change.
//
// The return is the *normalised* payload to store — only the fields that
// actually differ from the current value — plus the target as it stands now, so
// the reviewer can see both sides.
func (s *Server) prepareEdit(r *http.Request, targetType models.ReviewableType, targetID int64, payload json.RawMessage) ([]byte, *reviewTargetDto, error) {
	switch targetType {
	case models.TargetImage:
		return s.prepareImageEdit(r, targetID, payload)
	case models.TargetTag:
		return s.prepareTagEdit(r, targetID, payload)
	case models.TargetArtist:
		return s.prepareArtistEdit(r, targetID, payload)
	default:
		return nil, nil, invalidEdit("Unsupported target type.")
	}
}

func (s *Server) prepareImageEdit(r *http.Request, targetID int64, payload json.RawMessage) ([]byte, *reviewTargetDto, error) {
	var request imageEditPayload
	if err := decodeStrict(payload, &request); err != nil {
		return nil, nil, err
	}

	images, err := s.repo.GetImagesByIDs(r.Context(), []int64{targetID},
		repo.ChildVisibility{Filter: models.FilterAll}, true)
	if err != nil {
		return nil, nil, err
	}
	if len(images) == 0 || images[0].ReviewStatus != models.ReviewAccepted {
		return nil, nil, missingEdit("Image with ID " + itoa64(targetID) + " not found.")
	}
	image := images[0]

	visibility := repo.ChildVisibility{Filter: models.FilterAll}
	tagsByImage, err := s.repo.GetTagsForImages(r.Context(), []int64{targetID}, visibility)
	if err != nil {
		return nil, nil, err
	}
	artistsByImage, err := s.repo.GetArtistsForImages(r.Context(), []int64{targetID}, visibility)
	if err != nil {
		return nil, nil, err
	}

	proposedSource := ""
	if request.Source != nil {
		proposedSource = strings.TrimSpace(*request.Source)
	}
	hasSource := proposedSource != ""
	switch {
	case hasSource && len([]rune(proposedSource)) > maxSourceLength:
		return nil, nil, invalidEdit("Source URL must not exceed 500 characters.")
	case hasSource && !isHTTPURL(proposedSource):
		return nil, nil, invalidEdit("Source must be a valid http(s) URL.")
	}

	addTags := cleanSlugs(request.AddTagSlugs)
	removeTags := cleanSlugs(request.RemoveTagSlugs)
	addArtists := cleanIDs(request.AddArtistIDs)
	removeArtists := cleanIDs(request.RemoveArtistIDs)

	if err := s.requireTagsExist(r, addTags); err != nil {
		return nil, nil, err
	}
	if err := s.requireArtistsExist(r, addArtists); err != nil {
		return nil, nil, err
	}

	// Drop no-op entries: adding a tag the image already has, or removing one it
	// does not, is not a change and would make an otherwise-empty edit look
	// substantive.
	currentSlugs := map[string]bool{}
	for _, tag := range tagsByImage[targetID] {
		currentSlugs[tag.Slug] = true
	}
	currentArtistIDs := map[int64]bool{}
	for _, artist := range artistsByImage[targetID] {
		currentArtistIDs[artist.ID] = true
	}

	effectiveAddTags := without(addTags, func(slug string) bool { return currentSlugs[slug] })
	effectiveRemoveTags := only(removeTags, func(slug string) bool { return currentSlugs[slug] })
	effectiveAddArtists := without(addArtists, func(id int64) bool { return currentArtistIDs[id] })
	effectiveRemoveArtists := only(removeArtists, func(id int64) bool { return currentArtistIDs[id] })

	sourceChanges := hasSource && (image.Source == nil || proposedSource != *image.Source)
	nsfwChanges := request.IsNsfw != nil && *request.IsNsfw != image.IsNsfw

	if !sourceChanges && !nsfwChanges &&
		len(effectiveAddTags) == 0 && len(effectiveRemoveTags) == 0 &&
		len(effectiveAddArtists) == 0 && len(effectiveRemoveArtists) == 0 {
		return nil, nil, invalidEdit(
			"The proposed changes are empty or identical to the current values.")
	}

	normalized := imageEditPayload{}
	if sourceChanges {
		normalized.Source = &proposedSource
	}
	if nsfwChanges {
		value := *request.IsNsfw
		normalized.IsNsfw = &value
	}
	if len(effectiveAddTags) > 0 {
		normalized.AddTagSlugs = &effectiveAddTags
	}
	if len(effectiveRemoveTags) > 0 {
		normalized.RemoveTagSlugs = &effectiveRemoveTags
	}
	if len(effectiveAddArtists) > 0 {
		normalized.AddArtistIDs = &effectiveAddArtists
	}
	if len(effectiveRemoveArtists) > 0 {
		normalized.RemoveArtistIDs = &effectiveRemoveArtists
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, invalidEdit("The proposed changes could not be encoded.")
	}

	target, err := s.reviewTargetForImage(r, targetID)
	if err != nil {
		return nil, nil, err
	}
	return encoded, target, nil
}

func (s *Server) prepareTagEdit(r *http.Request, targetID int64, payload json.RawMessage) ([]byte, *reviewTargetDto, error) {
	var request tagEditPayload
	if err := decodeStrict(payload, &request); err != nil {
		return nil, nil, err
	}

	tag, err := s.repo.GetTagByID(r.Context(), targetID)
	if err != nil {
		if isNoRows(err) {
			return nil, nil, missingEdit("Tag with ID " + itoa64(targetID) + " not found.")
		}
		return nil, nil, err
	}
	if tag.ReviewStatus != models.ReviewAccepted {
		return nil, nil, missingEdit("Tag with ID " + itoa64(targetID) + " not found.")
	}

	proposedName := ""
	if request.Name != nil {
		proposedName = strings.TrimSpace(*request.Name)
	}
	if proposedName != "" && len([]rune(proposedName)) > maxNameLength {
		return nil, nil, invalidEdit("Name must not exceed 30 characters.")
	}

	proposedDescription := ""
	if request.Description != nil {
		proposedDescription = strings.TrimSpace(*request.Description)
	}
	if proposedDescription != "" && len([]rune(proposedDescription)) > maxDescriptionLength {
		return nil, nil, invalidEdit("Description must not exceed 150 characters.")
	}

	nameChanges := proposedName != "" && proposedName != tag.Name
	descriptionChanges := proposedDescription != "" && proposedDescription != tag.Description

	if !nameChanges && !descriptionChanges {
		return nil, nil, invalidEdit(
			"The proposed changes are empty or identical to the current values.")
	}

	normalized := tagEditPayload{}
	if nameChanges {
		normalized.Name = &proposedName
	}
	if descriptionChanges {
		normalized.Description = &proposedDescription
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, invalidEdit("The proposed changes could not be encoded.")
	}

	status := tag.ReviewStatus.String()
	return encoded, &reviewTargetDto{Tag: &TagDto{
		ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
		ReviewStatus: &status, CreatorID: tag.CreatorID,
	}}, nil
}

func (s *Server) prepareArtistEdit(r *http.Request, targetID int64, payload json.RawMessage) ([]byte, *reviewTargetDto, error) {
	var request artistEditPayload
	if err := decodeStrict(payload, &request); err != nil {
		return nil, nil, err
	}

	artist, err := s.repo.GetArtistByID(r.Context(), targetID)
	if err != nil {
		if isNoRows(err) {
			return nil, nil, missingEdit("Artist with ID " + itoa64(targetID) + " not found.")
		}
		return nil, nil, err
	}
	if artist.ReviewStatus != models.ReviewAccepted {
		return nil, nil, missingEdit("Artist with ID " + itoa64(targetID) + " not found.")
	}

	proposedName := ""
	if request.Name != nil {
		proposedName = strings.TrimSpace(*request.Name)
	}
	if proposedName != "" && len([]rune(proposedName)) > maxNameLength {
		return nil, nil, invalidEdit("Name must not exceed 30 characters.")
	}
	nameChanges := proposedName != "" && proposedName != artist.Name

	patreon, err := normaliseLink(request.Patreon, "Patreon")
	if err != nil {
		return nil, nil, err
	}
	pixiv, err := normaliseLink(request.Pixiv, "Pixiv")
	if err != nil {
		return nil, nil, err
	}
	twitter, err := normaliseLink(request.Twitter, "Twitter")
	if err != nil {
		return nil, nil, err
	}
	deviantArt, err := normaliseLink(request.DeviantArt, "DeviantArt")
	if err != nil {
		return nil, nil, err
	}

	patreonChanges := patreon != nil && !sameString(patreon, artist.Patreon)
	pixivChanges := pixiv != nil && !sameString(pixiv, artist.Pixiv)
	twitterChanges := twitter != nil && !sameString(twitter, artist.Twitter)
	deviantArtChanges := deviantArt != nil && !sameString(deviantArt, artist.DeviantArt)

	if !nameChanges && !patreonChanges && !pixivChanges &&
		!twitterChanges && !deviantArtChanges {
		return nil, nil, invalidEdit(
			"The proposed changes are empty or identical to the current values.")
	}

	normalized := artistEditPayload{}
	if nameChanges {
		normalized.Name = &proposedName
	}
	if patreonChanges {
		normalized.Patreon = patreon
	}
	if pixivChanges {
		normalized.Pixiv = pixiv
	}
	if twitterChanges {
		normalized.Twitter = twitter
	}
	if deviantArtChanges {
		normalized.DeviantArt = deviantArt
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, invalidEdit("The proposed changes could not be encoded.")
	}

	return encoded, &reviewTargetDto{Artist: ptr(toArtistDto(artist))}, nil
}

// applyEdit writes a stored payload to its target.
//
// Everything the payload does not mention is passed through unchanged, because
// the underlying update treats an absent value as "clear it" rather than
// "leave it".
func (s *Server) applyEdit(ctx context.Context, tx repo.Tx, targetType models.ReviewableType, targetID int64, payload []byte) error {
	switch targetType {
	case models.TargetImage:
		var request imageEditPayload
		if err := json.Unmarshal(payload, &request); err != nil {
			return invalidEdit("Stored payload is unreadable.")
		}
		return s.repo.ApplyImageEdit(ctx, tx, targetID, repo.ImageEditFields{
			Source:          request.Source,
			IsNsfw:          request.IsNsfw,
			AddTagSlugs:     deref(request.AddTagSlugs),
			RemoveTagSlugs:  deref(request.RemoveTagSlugs),
			AddArtistIDs:    deref(request.AddArtistIDs),
			RemoveArtistIDs: deref(request.RemoveArtistIDs),
		})

	case models.TargetTag:
		var request tagEditPayload
		if err := json.Unmarshal(payload, &request); err != nil {
			return invalidEdit("Stored payload is unreadable.")
		}
		name, description, err := s.repo.CurrentTagFields(ctx, targetID)
		if err != nil {
			return err
		}
		if request.Name != nil {
			name = strings.TrimSpace(*request.Name)
		}
		if request.Description != nil {
			description = strings.TrimSpace(*request.Description)
		}
		if _, err := s.repo.UpdateTag(ctx, targetID, name, "", description); err != nil {
			return err
		}
		return nil

	case models.TargetArtist:
		var request artistEditPayload
		if err := json.Unmarshal(payload, &request); err != nil {
			return invalidEdit("Stored payload is unreadable.")
		}
		artist, err := s.repo.GetArtistByID(ctx, targetID)
		if err != nil {
			return err
		}
		if request.Name != nil {
			artist.Name = strings.TrimSpace(*request.Name)
		}
		if request.Patreon != nil {
			artist.Patreon = request.Patreon
		}
		if request.Pixiv != nil {
			artist.Pixiv = request.Pixiv
		}
		if request.Twitter != nil {
			artist.Twitter = request.Twitter
		}
		if request.DeviantArt != nil {
			artist.DeviantArt = request.DeviantArt
		}
		_, err = s.repo.UpdateArtist(ctx, artist)
		return err

	default:
		return invalidEdit("Unsupported target type.")
	}
}

// applyNewContent writes a moderator's corrected content onto a still-pending
// NewContent submission.
//
// The review status and the uploader are deliberately untouched: this corrects
// what was submitted, it does not publish it. Publishing is approve's job.
func (s *Server) applyNewContent(r *http.Request, task models.ReviewTask, content json.RawMessage) (*reviewTargetDto, error) {
	switch task.TargetType {
	case models.TargetImage:
		var request imageEditPayload
		if err := decodeStrict(content, &request); err != nil {
			return nil, err
		}
		if err := s.repo.ApplyImageEdit(r.Context(), nil, task.TargetId, repo.ImageEditFields{
			Source: request.Source,
			IsNsfw: request.IsNsfw,
		}); err != nil {
			return nil, err
		}
		return s.reviewTargetForImage(r, task.TargetId)

	case models.TargetTag:
		var request tagEditPayload
		if err := decodeStrict(content, &request); err != nil {
			return nil, err
		}
		name, description, err := s.repo.CurrentTagFields(r.Context(), task.TargetId)
		if err != nil {
			return nil, err
		}
		if request.Name != nil {
			name = strings.TrimSpace(*request.Name)
		}
		if request.Description != nil {
			description = strings.TrimSpace(*request.Description)
		}
		if _, err := s.repo.UpdateTag(r.Context(), task.TargetId, name, "", description); err != nil {
			return nil, err
		}
		return s.reviewTargetFor(r, models.TargetTag, task.TargetId)

	case models.TargetArtist:
		var request artistEditPayload
		if err := decodeStrict(content, &request); err != nil {
			return nil, err
		}
		artist, err := s.repo.GetArtistByID(r.Context(), task.TargetId)
		if err != nil {
			return nil, err
		}
		if request.Name != nil {
			artist.Name = strings.TrimSpace(*request.Name)
		}
		if request.Patreon != nil {
			artist.Patreon = request.Patreon
		}
		if request.Pixiv != nil {
			artist.Pixiv = request.Pixiv
		}
		if request.Twitter != nil {
			artist.Twitter = request.Twitter
		}
		if request.DeviantArt != nil {
			artist.DeviantArt = request.DeviantArt
		}
		if _, err := s.repo.UpdateArtist(r.Context(), artist); err != nil {
			return nil, err
		}
		return s.reviewTargetFor(r, models.TargetArtist, task.TargetId)

	default:
		return nil, invalidEdit("Unsupported target type.")
	}
}

// requireTagsExist fails if any proposed tag slug does not exist.
//
// A proposal naming an unknown slug is refused rather than partially applied, so
// a typo surfaces as a 404 instead of silently dropping a tag.
func (s *Server) requireTagsExist(r *http.Request, slugs []string) error {
	for _, slug := range slugs {
		if _, err := s.repo.GetTagBySlug(r.Context(), slug); err != nil {
			return missingEdit("Tags with slugs " + slug + " not found.")
		}
	}
	return nil
}

// requireArtistsExist fails if any proposed artist id does not exist.
func (s *Server) requireArtistsExist(r *http.Request, ids []int64) error {
	for _, id := range ids {
		if _, err := s.repo.GetArtistByID(r.Context(), id); err != nil {
			return missingEdit("Artists with IDs " + itoa64(id) + " not found.")
		}
	}
	return nil
}

// reviewTargetForImage renders one image as a review target.
func (s *Server) reviewTargetForImage(r *http.Request, imageID int64) (*reviewTargetDto, error) {
	return s.reviewTargetFor(r, models.TargetImage, imageID)
}

// deref unwraps an optional list.
func deref[T any](values *[]T) []T {
	if values == nil {
		return nil
	}
	return *values
}

// cleanSlugs trims, drops blanks and de-duplicates while preserving order.
func cleanSlugs(values *[]string) []string {
	if values == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(*values))
	for _, raw := range *values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}

// cleanIDs drops non-positive ids and de-duplicates while preserving order.
func cleanIDs(values *[]int64) []int64 {
	if values == nil {
		return nil
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(*values))
	for _, id := range *values {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func without[T any](values []T, drop func(T) bool) []T {
	out := make([]T, 0, len(values))
	for _, value := range values {
		if !drop(value) {
			out = append(out, value)
		}
	}
	return out
}

func only[T any](values []T, keep func(T) bool) []T {
	return without(values, func(value T) bool { return !keep(value) })
}

// normaliseLink trims and validates one artist link.
func normaliseLink(value *string, field string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	if len([]rune(trimmed)) > maxLinkLength {
		return nil, invalidEdit(field + " link must not exceed 200 characters.")
	}
	if !isHTTPURL(trimmed) {
		return nil, invalidEdit(field + " link must be a valid http(s) URL.")
	}
	return &trimmed, nil
}

// isHTTPURL matches Uri.TryCreate plus the http/https scheme check.
func isHTTPURL(value string) bool {
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	rest := value[strings.Index(value, "://")+3:]
	return rest != ""
}

func sameString(value *string, other *string) bool {
	if value == nil || other == nil {
		return value == nil && other == nil
	}
	return *value == *other
}

func ptr[T any](value T) *T { return &value }

// applyStoredEdit writes a task's stored payload to its target.
//
// This is the EditApplier half of the pipeline, wired into approve. It takes the
// task rather than a payload because the target type and id come from the task,
// and that is the thing being approved — not a re-read of the request body.
func (s *Server) applyStoredEdit(ctx context.Context, tx repo.Tx, task models.ReviewTask) error {
	return s.applyEdit(ctx, tx, task.TargetType, task.TargetId, task.Payload)
}
