package httpapi

import (
	"net/http"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// registerReportRoutes mounts the report endpoints.
func (s *Server) registerReportRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("POST "+base+"reports", s.handleCreateReport)
	mux.HandleFunc("GET "+base+"reports", s.handleListReports)
	mux.HandleFunc("GET "+base+"reports/{id}", s.handleGetReport)
	mux.HandleFunc("PATCH "+base+"reports/{id}", s.handleUpdateReport)
	mux.HandleFunc("DELETE "+base+"reports/{id}", s.handleDeleteReport)
}

// registerUserRoutes mounts the user administration endpoints.
func (s *Server) registerUserRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"users", s.handleListUsers)
	mux.HandleFunc("PATCH "+base+"users/{id}/role", s.handleUpdateUserRole)
}

// reportDto is the wire form of a report.
type reportDto struct {
	ID           int64   `json:"id"`
	UserID       int64   `json:"userId"`
	ImageID      *int64  `json:"imageId,omitempty"`
	Description  string  `json:"description"`
	Status       string  `json:"status"`
	ReviewerNote *string `json:"reviewerNote,omitempty"`
	CreatedAt    string  `json:"createdAt"`
}

// createReportRequest is the POST body.
type createReportRequest struct {
	ImageID     int64  `json:"imageId"`
	Description string `json:"description"`
}

// handleCreateReport serves POST /reports.
//
// Anyone authenticated may report. The C# service does not restrict this beyond
// requiring a user, and a report is a safety signal, so gating it more tightly
// would discourage exactly the reports that matter.
func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	var body createReportRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	description := strings.TrimSpace(body.Description)
	if description == "" {
		writeAPIError(w, r, apierror.UnprocessableEntity("A description is required.").
			WithExtra("errors", map[string][]string{
				"Description": {"A description is required."}}))
		return
	}
	if body.ImageID == 0 {
		writeAPIError(w, r, apierror.UnprocessableEntity("An imageId is required.").
			WithExtra("errors", map[string][]string{"ImageId": {"An imageId is required."}}))
		return
	}

	report, err := s.repo.CreateReport(r.Context(), caller.UserID, body.ImageID, description)
	if err != nil {
		writeRepoError(w, r, "create report", err, "Image not found.")
		return
	}

	writeJSON(w, http.StatusOK, toReportDto(report))
}

// handleListReports serves GET /reports.
//
// Ordinary users see only their own reports; moderators see everything, which is
// what makes the queue usable.
func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter := repo.ReportFilter{
		Page:     page.offset(),
		PageSize: page.PageSize,
	}
	if !caller.CanModerate() {
		filter.UserID = &caller.UserID
	}

	if raw := r.URL.Query().Get("status"); raw != "" {
		status, err := parseReportStatus(raw)
		if err != nil {
			writeAPIError(w, r, err)
			return
		}
		filter.Status = &status
	}

	reports, total, err := s.repo.ListReports(r.Context(), filter)
	if err != nil {
		s.logError(r, "list reports", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]reportDto, 0, len(reports))
	for _, report := range reports {
		dtos = append(dtos, toReportDto(report))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
}

// handleGetReport serves GET /reports/{id}.
func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	report, err := s.repo.GetReport(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get report", err, "Report not found.")
		return
	}

	if !caller.CanModerate() && report.UserID != caller.UserID {
		apierror.NotFound("Report not found.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, toReportDto(report))
}

// updateReportRequest is the PATCH body.
type updateReportRequest struct {
	Status       string `json:"status"`
	ReviewerNote string `json:"reviewerNote"`
}

// handleUpdateReport serves PATCH /reports/{id}.
//
// Resolving a report is a moderation action.
func (s *Server) handleUpdateReport(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionModerate) {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateReportRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	status, err := parseReportStatus(body.Status)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	note := optionalString(body.ReviewerNote)

	report, err := s.repo.UpdateReport(r.Context(), id, status, note)
	if err != nil {
		writeRepoError(w, r, "update report", err, "Report not found.")
		return
	}

	writeJSON(w, http.StatusOK, toReportDto(report))
}

// handleDeleteReport serves DELETE /reports/{id}.
//
// A reporter may withdraw their own report; a moderator may remove any.
func (s *Server) handleDeleteReport(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	tag, err := s.repo.DeleteReport(r.Context(), id, caller.UserID, caller.CanModerate())
	if err != nil {
		writeRepoError(w, r, "delete report", err, "Report not found.")
		return
	}
	if tag == 0 {
		apierror.NotFound("Report not found.").Write(w, r.URL.Path)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// toReportDto converts a report for the response.
func toReportDto(report models.Report) reportDto {
	return reportDto{
		ID:           report.ID,
		UserID:       report.UserID,
		ImageID:      report.ImageID,
		Description:  report.Description,
		Status:       report.Status.String(),
		ReviewerNote: report.ReviewerNote,
		CreatedAt:    report.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

// parseReportStatus converts the wire status name.
func parseReportStatus(value string) (models.ReportStatus, error) {
	switch value {
	case "Pending":
		return models.ReportPending, nil
	case "Accepted":
		return models.ReportAccepted, nil
	case "Rejected":
		return models.ReportRejected, nil
	default:
		return 0, apierror.BadRequest("'status' must be Pending, Accepted or Rejected.")
	}
}

// handleListUsers serves GET /users.
//
// Administrator only: the endpoint exposes role and blacklist state, which is
// not something ordinary users should enumerate.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}
	if !caller.IsAdmin() {
		writeForbidden(w, r, &authz.ErrForbidden{Action: "list users", RequiredRole: config.RoleAdmin})
		return
	}

	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	users, total, err := s.repo.ListUsers(r.Context(), page.offset(), page.PageSize)
	if err != nil {
		s.logError(r, "list users", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]models.UserMinimal, 0, len(users))
	for _, user := range users {
		dtos = append(dtos, models.UserMinimal{
			ID:        user.ID,
			Name:      user.Name,
			DiscordID: user.DiscordID,
			AvatarURL: user.AvatarURL,
			Role:      roleNameFor(user.Role),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result":     dtos,
		"pagination": buildPagination(page, int(total)),
	})
}

// updateUserRoleRequest is the body for a role change.
type updateUserRoleRequest struct {
	Role string `json:"role"`
}

// handleUpdateUserRole serves PATCH /users/{id}/role.
//
// This is the endpoint a maintainer uses to promote themselves to Admin, which
// the README describes as a manual SQL step. Doing it over the API means the
// change is attributable rather than being an unexplained row update.
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}
	if !caller.IsAdmin() {
		writeForbidden(w, r, &authz.ErrForbidden{
			Action: "change user roles", RequiredRole: config.RoleAdmin})
		return
	}

	targetID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateUserRoleRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	role, err := config.ParseRole(body.Role)
	if err != nil {
		writeAPIError(w, r, apierror.BadRequest(err.Error()))
		return
	}

	updated, err := s.repo.UpdateUserRole(r.Context(), targetID, int(role))
	if err != nil {
		writeRepoError(w, r, "update user role", err, "User not found.")
		return
	}

	writeJSON(w, http.StatusOK, models.UserMinimal{
		ID:        updated.ID,
		Name:      updated.Name,
		DiscordID: updated.DiscordID,
		AvatarURL: updated.AvatarURL,
		Role:      roleNameFor(updated.Role),
	})
}
