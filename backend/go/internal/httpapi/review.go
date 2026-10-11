package httpapi

import (
	"net/http"

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

	mux.HandleFunc("GET "+base+"review/tasks", s.handleListReviewTasks)
	mux.HandleFunc("GET "+base+"review/tasks/{id}", s.handleGetReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/approve", s.handleApproveReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/reject", s.handleRejectReviewTask)
	mux.HandleFunc("PATCH "+base+"review/tasks/{id}/cancel", s.handleCancelReviewTask)
}

// reviewTaskDto is the wire form of a moderation task.
type reviewTaskDto struct {
	ID           int64   `json:"id"`
	Kind         string  `json:"kind"`
	TargetType   string  `json:"targetType"`
	TargetId     int64   `json:"targetId"`
	SubmitterID  *int64  `json:"submitterId,omitempty"`
	Status       string  `json:"status"`
	Reason       string  `json:"reason,omitempty"`
	ReviewerNote string  `json:"reviewerNote,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	ResolvedAt   *string `json:"resolvedAt,omitempty"`
}

// toReviewTaskDto converts a row for the response.
func toReviewTaskDto(task models.ReviewTask) reviewTaskDto {
	dto := reviewTaskDto{
		ID:           task.ID,
		TargetType:   task.TargetType.String(),
		TargetId:     task.TargetId,
		SubmitterID:  task.SubmitterID,
		Status:       task.Status.String(),
		Reason:       task.Reason,
		ReviewerNote: task.ReviewerNote,
		CreatedAt:    task.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}

	if task.Kind == models.KindEdit {
		dto.Kind = "Edit"
	} else {
		dto.Kind = "NewContent"
	}

	if task.ResolvedAt != nil {
		formatted := task.ResolvedAt.Format("2006-01-02T15:04:05.000Z07:00")
		dto.ResolvedAt = &formatted
	}
	return dto
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

	page, err := parsePagination(r)
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
	if raw := query.Get("type"); raw != "" {
		targetType, err := models.ParseReviewableType(raw)
		if err != nil {
			writeAPIError(w, r, apierror.BadRequest(err.Error()))
			return
		}
		filter.TargetType = &targetType
	}

	// Moderators see everything; everyone else sees only their own tasks.
	if !caller.CanModerate() {
		filter.SubmitterID = &caller.UserID
	}

	tasks, total, err := s.repo.ListReviewTasks(r.Context(), filter)
	if err != nil {
		s.logError(r, "list review tasks", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]reviewTaskDto, 0, len(tasks))
	for _, task := range tasks {
		dtos = append(dtos, toReviewTaskDto(task))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
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
		writeRepoError(w, r, "get review task", err, "Review task not found.")
		return
	}

	// A non-moderator may only read their own task, and is told it does not
	// exist rather than that it is forbidden.
	if !caller.CanModerate() && (task.SubmitterID == nil || *task.SubmitterID != caller.UserID) {
		apierror.NotFound("Review task not found.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, toReviewTaskDto(task))
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

	if err := s.repo.ApproveReviewTask(r.Context(), tx, id, reviewer); err != nil {
		writeRepoError(w, r, "approve review task", err, "Review task not found.")
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

	if err := s.repo.RejectReviewTask(r.Context(), tx, id, reviewer, body.Reason); err != nil {
		writeRepoError(w, r, "reject review task", err, "Review task not found.")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		s.logError(r, "commit reject", err)
		internalServerError(w, r)
		return
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

	if caller.CanModerate() {
		// Moderators cancel through the shared path with their own ID; the
		// ownership clause is relaxed by using the reviewer identity check in
		// the handler below.
		task, err := s.repo.GetReviewTask(r.Context(), id)
		if err != nil {
			writeRepoError(w, r, "cancel review task", err, "Review task not found.")
			return
		}
		if task.Status != models.TaskPending {
			apierror.Conflict("This review task has already been resolved.").Write(w, r.URL.Path)
			return
		}
		if err := s.repo.CancelReviewTaskAsModerator(r.Context(), id, caller.UserID); err != nil {
			writeRepoError(w, r, "cancel review task", err, "Review task not found.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := s.repo.CancelReviewTask(r.Context(), id, caller.UserID); err != nil {
		writeRepoError(w, r, "cancel review task", err,
			"Review task not found, not owned by you, or already resolved.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
