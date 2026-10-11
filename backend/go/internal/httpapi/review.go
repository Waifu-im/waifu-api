package httpapi

import (
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

// registerReviewRoutes mounts the moderation queue endpoints.
//
// Approve and reject require Moderator or Admin. That is NOT configurable,
// matching the hardcoded AuthorizationPolicies.Moderator in ReviewController.cs.
// No .env value can grant approve rights to a lower role.
func (s *Server) registerReviewRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("POST "+base+"review/tasks", s.handleSubmitEdit)
	mux.HandleFunc("GET "+base+"review/tasks", s.handleListReviewTasks)
	mux.HandleFunc("GET "+base+"review/tasks/{id}", s.handleGetReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}", s.handleEditReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/approve", s.handleApproveReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/reject", s.handleRejectReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/cancel", s.handleCancelReviewTask)
}

// reviewTaskDto is the wire form of a moderation task.
//
// It is ReviewTaskDto.cs. The submitter and reviewer blocks, the payload and the
// current target are all rendered by the review page, so none of them are
// optional decoration.
type reviewTaskDto struct {
	ID           int64            `json:"id"`
	Kind         string           `json:"kind"`
	SubmitterID  *int64           `json:"submitterId"`
	Submitter    *userMinimalDto  `json:"submitter"`
	TargetType   string           `json:"targetType"`
	TargetId     int64            `json:"targetId"`
	Payload      any              `json:"payload"`
	Target       *reviewTargetDto `json:"target"`
	Reason       *string          `json:"reason"`
	Status       string           `json:"status"`
	ReviewerID   *int64           `json:"reviewerId"`
	Reviewer     *userMinimalDto  `json:"reviewer"`
	ReviewerNote *string          `json:"reviewerNote"`
	// ModeratorEditedAt is set when a moderator corrected someone else's
	// submission; the review page shows the submitter a "modified" cue.
	ModeratorEditedAt *time.Time `json:"moderatorEditedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	ResolvedAt        *time.Time `json:"resolvedAt"`
}

// reviewTargetDto is the current state of a task's target.
//
// Exactly one member is populated, matching targetType. All three are null when
// the target no longer exists, which is what a rejected submission looks like.
type reviewTargetDto struct {
	Image  *ImageDto  `json:"image"`
	Tag    *TagDto    `json:"tag"`
	Artist *ArtistDto `json:"artist"`
}

// reviewTaskFromRow assembles the DTO from a row and its resolved relations.
func reviewTaskFromRow(task models.ReviewTask, payload any, target *reviewTargetDto,
	submitter, reviewer *models.User) reviewTaskDto {

	kind := "NewContent"
	if task.Kind == models.KindEdit {
		kind = "Edit"
	}

	var resolvedAt *time.Time
	if task.ResolvedAt != nil {
		value := *task.ResolvedAt
		resolvedAt = &value
	}

	return reviewTaskDto{
		ID:                task.ID,
		Kind:              kind,
		SubmitterID:       task.SubmitterID,
		Submitter:         minimalOf(submitter),
		TargetType:        task.TargetType.String(),
		TargetId:          task.TargetId,
		Payload:           payload,
		Target:            target,
		Reason:            optionalString(task.Reason),
		Status:            task.Status.String(),
		ReviewerID:        task.ReviewerID,
		Reviewer:          minimalOf(reviewer),
		ReviewerNote:      optionalString(task.ReviewerNote),
		ModeratorEditedAt: task.ModeratorEditedAt,
		CreatedAt:         task.CreatedAt,
		ResolvedAt:        resolvedAt,
	}
}

func minimalOf(user *models.User) *userMinimalDto {
	if user == nil {
		return nil
	}
	return &userMinimalDto{ID: user.ID, Name: user.Name, AvatarURL: user.AvatarURL}
}

// handleListReviewTasks serves GET /review/tasks.
//
// Non-moderators see only their own submissions, matching the scoping in
// ReviewController.cs. This is enforced here rather than relying on the caller,
// so a future route cannot forget it.
func (s *Server) handleListReviewTasks(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.Review)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter := repo.ReviewTaskFilter{
		Page:     page.offset(),
		PageSize: page.PageSize,
	}

	query := r.URL.Query()

	if raw := query.Get("status"); raw != "" {
		status, err := parseReviewTaskStatus(raw)
		if err != nil {
			writeAPIError(w, r, err)
			return
		}
		filter.Status = &status
	}

	// The parameter is targetType, matching GetReviewTasksRequest. An earlier
	// port read "type", which silently ignored the filter: the review page's
	// per-type tabs sent targetType and received the whole queue instead.
	if raw := query.Get("targetType"); raw != "" {
		targetType, err := models.ParseReviewableType(raw)
		if err != nil {
			writeAPIError(w, r, validationError("TargetType", err.Error()))
			return
		}
		filter.TargetType = &targetType
	}

	if raw := query.Get("kind"); raw != "" {
		kind, err := parseReviewTaskKind(raw)
		if err != nil {
			writeAPIError(w, r, err)
			return
		}
		filter.Kind = &kind
	}

	if raw := query.Get("targetId"); raw != "" {
		targetID, err := parsePositiveInt64(raw)
		if err != nil {
			writeAPIError(w, r, validationError("TargetId", "'targetId' must be a number."))
			return
		}
		filter.TargetID = &targetID
	}

	// Non-moderators are always scoped to their own submissions. A moderator may
	// ask for just theirs with mine=true, which is the "my submissions" view.
	if !caller.CanModerate() || truthy(query.Get("mine")) {
		filter.SubmitterID = &caller.UserID
	}

	tasks, total, err := s.repo.ListReviewTasks(r.Context(), filter)
	if err != nil {
		s.logError(r, "list review tasks", err)
		internalServerError(w, r)
		return
	}

	dtos, err := s.reviewTaskDtos(r, tasks)
	if err != nil {
		s.logError(r, "build review tasks", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// reviewTaskDtos resolves the submitter, reviewer, payload and target for a page
// of tasks.
//
// All four relations are loaded per page rather than per task. A default review
// page is 30 tasks; resolving them individually would be 120 round trips.
func (s *Server) reviewTaskDtos(r *http.Request, tasks []models.ReviewTask) ([]reviewTaskDto, error) {
	out := make([]reviewTaskDto, 0, len(tasks))
	if len(tasks) == 0 {
		return out, nil
	}

	var submitterIDs, reviewerIDs []int64
	imageIDs, tagIDs, artistIDs := []int64{}, []int64{}, []int64{}

	for _, task := range tasks {
		if task.SubmitterID != nil {
			submitterIDs = append(submitterIDs, *task.SubmitterID)
		}
		if task.ReviewerID != nil {
			reviewerIDs = append(reviewerIDs, *task.ReviewerID)
		}
		switch task.TargetType {
		case models.TargetImage:
			imageIDs = append(imageIDs, task.TargetId)
		case models.TargetTag:
			tagIDs = append(tagIDs, task.TargetId)
		case models.TargetArtist:
			artistIDs = append(artistIDs, task.TargetId)
		}
	}

	submitters, err := s.repo.GetUsersByIDs(r.Context(), submitterIDs)
	if err != nil {
		return nil, err
	}
	reviewers, err := s.repo.GetUsersByIDs(r.Context(), reviewerIDs)
	if err != nil {
		return nil, err
	}

	// The queue always shows the full target, including pending content and the
	// uploader id — that is the whole point of the moderation screen.
	visibility := repo.ChildVisibility{Filter: models.FilterAll}
	images, err := s.repo.GetImagesByIDs(r.Context(), imageIDs, visibility, true)
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

	tagsByID, err := s.repo.GetTagsByIDs(r.Context(), tagIDs)
	if err != nil {
		return nil, err
	}
	artistsByID, err := s.repo.GetArtistsByIDsMap(r.Context(), artistIDs)
	if err != nil {
		return nil, err
	}

	for _, task := range tasks {
		var target *reviewTargetDto
		switch task.TargetType {
		case models.TargetImage:
			if dto, ok := imagesByID[task.TargetId]; ok {
				target = &reviewTargetDto{Image: &dto}
			}
		case models.TargetTag:
			if tag, ok := tagsByID[task.TargetId]; ok {
				status := tag.ReviewStatus.String()
				target = &reviewTargetDto{Tag: &TagDto{
					ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
					ReviewStatus: &status, CreatorID: tag.CreatorID,
				}}
			}
		case models.TargetArtist:
			if artist, ok := artistsByID[task.TargetId]; ok {
				dto := toArtistDto(artist)
				target = &reviewTargetDto{Artist: &dto}
			}
		}

		var submitter, reviewer *models.User
		if task.SubmitterID != nil {
			if user, ok := submitters[*task.SubmitterID]; ok {
				submitter = &user
			}
		}
		if task.ReviewerID != nil {
			if user, ok := reviewers[*task.ReviewerID]; ok {
				reviewer = &user
			}
		}

		out = append(out, reviewTaskFromRow(task, decodePayloadValue(task.Payload), target,
			submitter, reviewer))
	}
	return out, nil
}

// decodePayloadValue turns the stored JSON into something json.Marshal will
// render back identically.
//
// The payload is opaque to the API: it round-trips the client's own object, so
// decoding into any preserves it while normalising it to a non-nil map (an
// empty payload is "{}" in the database and must not serialise as null).
func decodePayloadValue(raw []byte) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	if value == nil {
		return map[string]any{}
	}
	return value
}

// handleGetReviewTask serves GET /review/tasks/{id}.
func (s *Server) handleGetReviewTask(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	task, err := s.repo.GetReviewTask(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get review task", err,
			"Review task with ID "+itoa64(id)+" not found.")
		return
	}

	// A non-moderator may only read their own task, and is told it does not
	// exist rather than that it is forbidden.
	if !caller.CanModerate() && (task.SubmitterID == nil || *task.SubmitterID != caller.UserID) {
		apierror.NotFound("Review task with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
		return
	}

	dto, err := s.buildReviewTaskDto(r, id, nil)
	if err != nil {
		s.logError(r, "build review task", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dto)
}

// buildReviewTaskDto loads one task and renders it.
//
// whenTarget is used when the caller has just resolved the target itself, which
// saves a second read on the create and edit paths.
func (s *Server) buildReviewTaskDto(r *http.Request, id int64, whenTarget *reviewTargetDto) (reviewTaskDto, error) {
	task, err := s.repo.GetReviewTask(r.Context(), id)
	if err != nil {
		return reviewTaskDto{}, err
	}

	var submitter, reviewer *models.User
	if task.SubmitterID != nil {
		users, err := s.repo.GetUsersByIDs(r.Context(), []int64{*task.SubmitterID})
		if err != nil {
			return reviewTaskDto{}, err
		}
		if user, ok := users[*task.SubmitterID]; ok {
			submitter = &user
		}
	}
	if task.ReviewerID != nil {
		users, err := s.repo.GetUsersByIDs(r.Context(), []int64{*task.ReviewerID})
		if err != nil {
			return reviewTaskDto{}, err
		}
		if user, ok := users[*task.ReviewerID]; ok {
			reviewer = &user
		}
	}

	target := whenTarget
	if target == nil {
		resolved, err := s.reviewTargetFor(r, task.TargetType, task.TargetId)
		if err != nil {
			return reviewTaskDto{}, err
		}
		target = resolved
	}

	return reviewTaskFromRow(task, decodePayloadValue(task.Payload), target, submitter, reviewer), nil
}

// reviewTargetFor loads one target for the review screen.
func (s *Server) reviewTargetFor(r *http.Request, targetType models.ReviewableType, targetID int64) (*reviewTargetDto, error) {
	switch targetType {
	case models.TargetImage:
		images, err := s.repo.GetImagesByIDs(r.Context(), []int64{targetID},
			repo.ChildVisibility{Filter: models.FilterAll}, true)
		if err != nil || len(images) == 0 {
			return nil, err
		}
		dtos, err := s.toImageDtos(r, images, repo.ImageFilter{ReviewStatus: models.FilterAll}, 0)
		if err != nil || len(dtos) == 0 {
			return nil, err
		}
		return &reviewTargetDto{Image: &dtos[0]}, nil

	case models.TargetTag:
		tag, err := s.repo.GetTagByID(r.Context(), targetID)
		if err != nil {
			if isNoRows(err) {
				return nil, nil
			}
			return nil, err
		}
		status := tag.ReviewStatus.String()
		return &reviewTargetDto{Tag: &TagDto{
			ID: tag.ID, Name: tag.Name, Slug: tag.Slug, Description: tag.Description,
			ReviewStatus: &status, CreatorID: tag.CreatorID,
		}}, nil

	case models.TargetArtist:
		artist, err := s.repo.GetArtistByID(r.Context(), targetID)
		if err != nil {
			if isNoRows(err) {
				return nil, nil
			}
			return nil, err
		}
		dto := toArtistDto(artist)
		return &reviewTargetDto{Artist: &dto}, nil

	default:
		return nil, nil
	}
}

// handleApproveReviewTask serves PATCH /review/tasks/{id}/approve.
func (s *Server) handleApproveReviewTask(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionModerate) {
		return
	}
	reviewer := s.principal(r).UserID

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin approve transaction", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	if err := s.repo.ApproveReviewTask(r.Context(), tx, id, reviewer, s.applyStoredEdit); err != nil {
		s.writeReviewTransitionError(w, r, "approve review task", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit approve", err)
		internalServerError(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// rejectRequest is the body of the reject endpoint.
type rejectRequest struct {
	Reason string `json:"reason"`
}

// handleRejectReviewTask serves PATCH /review/tasks/{id}/reject.
//
// Rejecting deletes the submitted content rather than flagging it, which is why
// ReviewStatus has no Rejected member.
func (s *Server) handleRejectReviewTask(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionModerate) {
		return
	}
	reviewer := s.principal(r).UserID

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body rejectRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin reject transaction", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	// Read the task first so the stored object of a rejected upload can be
	// cleaned up after the commit. The delete happens inside the transaction;
	// the storage delete cannot, because it is a network call.
	task, err := s.repo.GetReviewTask(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get review task", err,
			"Review task with ID "+itoa64(id)+" not found.")
		return
	}

	if err := s.repo.RejectReviewTask(r.Context(), tx, id, reviewer, optionalString(body.Reason)); err != nil {
		s.writeReviewTransitionError(w, r, "reject review task", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit reject", err)
		internalServerError(w, r)
		return
	}

	// A rejected NewContent image has had its row deleted, so its stored object
	// is now an orphan. Removing it is best effort: storage-diff would surface a
	// leftover anyway, and failing the request after a successful commit would
	// misreport what actually happened.
	if task.Kind == models.KindNewContent && task.TargetType == models.TargetImage {
		s.deleteImageObjects(r, task.TargetId, "")
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCancelReviewTask serves PATCH /review/tasks/{id}/cancel.
//
// The submitter may withdraw their own task; a moderator may cancel any. The
// check lives in SQL so a task cannot be cancelled by someone else.
func (s *Server) handleCancelReviewTask(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	task, err := s.repo.GetReviewTask(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get review task", err,
			"Review task with ID "+itoa64(id)+" not found.")
		return
	}

	// Only the submitter or a moderator may cancel. The check is repeated in the
	// UPDATE's WHERE clause so a race cannot let one user close another's task.
	if (task.SubmitterID == nil || *task.SubmitterID != caller.UserID) && !caller.CanModerate() {
		writeAPIError(w, r, apierror.Forbidden("You can only cancel your own submissions."))
		return
	}
	if task.Status != models.TaskPending {
		writeAPIError(w, r, apierror.Conflict("This review task has already been resolved."))
		return
	}

	tx, err := s.pool().Begin(r.Context())
	if err != nil {
		s.logError(r, "begin cancel transaction", err)
		internalServerError(w, r)
		return
	}
	defer func() { _ = tx.Rollback(contextWithoutCancel(r)) }()

	if caller.CanModerate() {
		_, err = s.repo.CancelReviewTaskAsModerator(r.Context(), tx, id, caller.UserID)
	} else {
		_, err = s.repo.CancelReviewTask(r.Context(), tx, id, caller.UserID)
	}
	if err != nil {
		s.writeReviewTransitionError(w, r, "cancel review task", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit cancel", err)
		internalServerError(w, r)
		return
	}

	// Withdrawing a NewContent upload removes the object too, exactly as a
	// rejection does.
	if task.Kind == models.KindNewContent && task.TargetType == models.TargetImage {
		s.deleteImageObjects(r, task.TargetId, "")
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeReviewTransitionError maps the two ways a review transition can fail.
//
// They are distinct responses: a missing task is a 404, a task that was already
// decided is a 409, and conflating them would tell a moderator their click did
// nothing when in fact somebody beat them to it.
func (s *Server) writeReviewTransitionError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, repo.ErrConflict):
		apierror.Conflict("This review task has already been resolved.").Write(w, r.URL.Path)
	case isNoRows(err), errors.Is(err, repo.ErrNotFound):
		writeRepoError(w, r, operation, err, "Review task not found.")
	default:
		// An edit whose target has since been deleted is a genuine failure the
		// moderator needs to see, not a 404: the task exists, its target does not.
		var edit *editError
		if errors.As(err, &edit) {
			writeEditError(w, r, edit)
			return
		}
		s.logError(r, operation, err)
		internalServerError(w, r)
	}
}

// parseReviewTaskKind converts the wire kind name.
//
// The names are the C# enum names because the frontend sends
// ReviewTaskKind.Edit verbatim from the query string.
func parseReviewTaskKind(value string) (models.ReviewTaskKind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "newcontent":
		return models.KindNewContent, nil
	case "edit":
		return models.KindEdit, nil
	default:
		return 0, validationError("Kind", "'kind' must be NewContent or Edit.")
	}
}

// parseReviewTaskStatus converts the wire status name.
func parseReviewTaskStatus(value string) (models.ReviewTaskStatus, error) {
	switch value {
	case "Pending":
		return models.TaskPending, nil
	case "Approved":
		return models.TaskApproved, nil
	case "Rejected":
		return models.TaskRejected, nil
	case "Cancelled":
		return models.TaskCancelled, nil
	default:
		return 0, apierror.BadRequest(
			"'status' must be Pending, Approved, Rejected or Cancelled.")
	}
}
