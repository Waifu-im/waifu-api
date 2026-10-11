package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// registerReportRoutes mounts the report endpoints.
//
// The path shape is ReportsController.cs exactly: create, list, edit, and three
// status transitions. There is deliberately no GET or DELETE on an individual
// report, because the C# service exposes neither and adding one would widen the
// contract this rewrite exists to freeze.
func (s *Server) registerReportRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("POST "+base+"reports", s.handleCreateReport)
	mux.HandleFunc("GET "+base+"reports", s.handleListReports)
	mux.HandleFunc("PATCH "+base+"reports/{id}", s.handleUpdateReport)

	// The three transitions answer 204 with no body, matching the C# controller.
	mux.HandleFunc("PATCH "+base+"reports/{id}/validate", s.handleValidateReport)
	mux.HandleFunc("PATCH "+base+"reports/{id}/reject", s.handleRejectReport)
	mux.HandleFunc("PATCH "+base+"reports/{id}/cancel", s.handleCancelReport)
}

// reportDto is the wire form of a report.
//
// It is ReportDto.cs, which embeds the reporter and the reported image. Both
// matter: the moderation queue shows who reported and what was reported, and
// MyReports shows the reporter's own avatar.
type reportDto struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"userId"`
	User         *userMinimalDto `json:"user"`
	ImageID      *int64          `json:"imageId"`
	Image        *ImageDto       `json:"image"`
	Description  *string         `json:"description"`
	Status       string          `json:"status"`
	ReviewerNote *string         `json:"reviewerNote"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// userMinimalDto is UserMinimalDto: no role, no discord id.
//
// The report screen renders an avatar and a name; exposing the role there would
// leak moderation state into every viewer's queue.
type userMinimalDto struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl"`
}

// createReportRequest is the body of POST /reports.
type createReportRequest struct {
	ImageID     int64   `json:"imageId"`
	Description *string `json:"description"`
}

// maxReportDescription is the FluentValidation limit on the description.
const maxReportDescription = 500

// handleCreateReport serves POST /reports.
//
// A second pending report from the same user for the same image is refused with
// 409, so one person cannot flood the queue with copies of the same complaint.
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

	failures := map[string]string{}
	description := ""
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	if description == "" {
		failures["Description"] = "Description is required."
	} else if len([]rune(description)) > maxReportDescription {
		failures["Description"] = "Description must not exceed 500 characters."
	}
	if body.ImageID <= 0 {
		failures["ImageId"] = "Invalid Image ID."
	}
	if len(failures) > 0 {
		writeAPIError(w, r, validationErrors(failures))
		return
	}

	report, err := s.repo.CreateReport(r.Context(), caller.UserID, body.ImageID, description)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrConflict):
			apierror.Conflict("You already have a pending report for this image.").Write(w, r.URL.Path)
		case isNoRows(err):
			apierror.NotFound("Image not found.").Write(w, r.URL.Path)
		default:
			s.logError(r, "create report", err)
			internalServerError(w, r)
		}
		return
	}

	// The C# controller returns the Report entity rather than the DTO, so the
	// created response has no user or image block. Clients that need them
	// re-fetch the list.
	location := s.baseLocationPrefix() + "reports/" + itoa64(report.ID)
	s.writeCreated(w, r, reportEntityDto{
		ID:           report.ID,
		UserID:       report.UserID,
		ImageID:      report.ImageID,
		Description:  report.Description,
		CreatedAt:    report.CreatedAt,
		Status:       report.Status.String(),
		ReviewerNote: report.ReviewerNote,
	}, location)
}

// reportEntityDto is the bare Report entity as POST /reports returns it.
//
// It differs from reportDto in that the navigation properties are absent, which
// is what serialising the entity directly produces.
type reportEntityDto struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	ImageID      *int64    `json:"imageId"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"createdAt"`
	Status       string    `json:"status"`
	ReviewerNote *string   `json:"reviewerNote"`
}

// handleListReports serves GET /reports.
func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.Report)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	query := r.URL.Query()
	filter := repo.ReportFilter{Page: page.Page, PageSize: page.PageSize}

	if raw := query.Get("status"); raw != "" {
		status, ok := models.ParseReportStatus(raw)
		if !ok {
			writeAPIError(w, r, validationError("Status",
				"'status' must be Pending, Resolved, Rejected or Cancelled."))
			return
		}
		filter.Status = &status
	}

	// Ordinary users are always scoped to their own reports. A moderator may ask
	// for just their own with mine=true, which is the "My Reports" view.
	mine := truthy(query.Get("mine"))
	if !caller.CanModerate() || mine {
		filter.UserID = &caller.UserID
	}

	reports, total, err := s.repo.ListReports(r.Context(), filter)
	if err != nil {
		s.logError(r, "list reports", err)
		internalServerError(w, r)
		return
	}

	dtos, err := s.toReportDtos(r, reports, caller.CanModerate())
	if err != nil {
		s.logError(r, "resolve report relations", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// updateReportRequest is the body of PATCH /reports/{id}.
type updateReportRequest struct {
	Description *string `json:"description"`
}

// handleUpdateReport serves PATCH /reports/{id}.
//
// Only the description is editable, and only while the report is still open. The
// status is deliberately not settable here: it moves through the three dedicated
// transition routes, which each enforce their own authorisation and record a
// moderator note.
func (s *Server) handleUpdateReport(w http.ResponseWriter, r *http.Request) {
	caller := s.requireAuth(w, r)
	if caller == nil {
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

	description := ""
	if body.Description != nil {
		description = strings.TrimSpace(*body.Description)
	}
	switch {
	case description == "":
		writeAPIError(w, r, validationError("Description", "Description is required."))
		return
	case len([]rune(description)) > maxReportDescription:
		writeAPIError(w, r, validationError("Description",
			"Description must not exceed 500 characters."))
		return
	}

	report, err := s.repo.GetReport(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get report", err, "Report not found.")
		return
	}

	if !caller.CanModerate() && report.UserID != caller.UserID {
		writeAPIError(w, r, apierror.Forbidden("You can only edit your own reports."))
		return
	}

	updated, err := s.repo.UpdateReportDescription(r.Context(), id, description)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// The pending check lives in the UPDATE, so a concurrent moderator
			// resolving the report surfaces here as "already handled".
			apierror.Conflict("This report has already been handled and can no longer be edited.").
				Write(w, r.URL.Path)
			return
		}
		s.logError(r, "update report", err)
		internalServerError(w, r)
		return
	}

	dtos, err := s.toReportDtos(r, []models.Report{updated}, caller.CanModerate())
	if err != nil || len(dtos) == 0 {
		s.logError(r, "resolve report relations", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dtos[0])
}

// reportActionRequest is the body of validate and reject.
type reportActionRequest struct {
	Reason *string `json:"reason"`
}

// handleValidateReport serves PATCH /reports/{id}/validate.
//
// Validating a report does NOT delete the image. The C# handler only moves the
// report to Resolved; a moderator who also wants the image gone deletes it
// through DELETE /images/{id}, which the reports screen offers separately.
func (s *Server) handleValidateReport(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionModerate) {
		return
	}
	s.finishReportAction(w, r, models.ReportResolved)
}

// handleRejectReport serves PATCH /reports/{id}/reject.
func (s *Server) handleRejectReport(w http.ResponseWriter, r *http.Request) {
	if !s.requirePermission(w, r, authz.ActionModerate) {
		return
	}
	s.finishReportAction(w, r, models.ReportRejected)
}

// handleCancelReport serves PATCH /reports/{id}/cancel.
//
// Withdrawal is not a moderator action: the reporter may cancel their own open
// report, and a moderator may cancel any.
func (s *Server) handleCancelReport(w http.ResponseWriter, r *http.Request) {
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
		writeAPIError(w, r, apierror.Forbidden("You can only cancel your own reports."))
		return
	}

	if err := s.repo.SetReportStatus(r.Context(), id, models.ReportCancelled, nil); err != nil {
		s.writeReportTransitionError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// finishReportAction applies a moderator decision and answers 204.
func (s *Server) finishReportAction(w http.ResponseWriter, r *http.Request, target models.ReportStatus) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body reportActionRequest
	// The body is optional on these routes; an absent or malformed one simply
	// means no note is recorded.
	_ = decodeJSONBody(r, &body)

	var note *string
	if body.Reason != nil {
		note = optionalString(*body.Reason)
	}

	if err := s.repo.SetReportStatus(r.Context(), id, target, note); err != nil {
		s.writeReportTransitionError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeReportTransitionError maps the two failure modes of a status change.
func (s *Server) writeReportTransitionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		writeRepoError(w, r, "report transition", err, "Report not found.")
	case errors.Is(err, repo.ErrReportHandled):
		apierror.Conflict("This report has already been handled.").Write(w, r.URL.Path)
	default:
		s.logError(r, "report transition", err)
		internalServerError(w, r)
	}
}

// toReportDtos embeds the reporter and the reported image.
//
// The reporter and the image are loaded in batches rather than per report,
// because the moderation queue renders thirty of them at a time.
func (s *Server) toReportDtos(r *http.Request, reports []models.Report, includeUploaderID bool) ([]reportDto, error) {
	out := make([]reportDto, 0, len(reports))
	if len(reports) == 0 {
		return out, nil
	}

	userIDs := make([]int64, 0, len(reports))
	imageIDs := make([]int64, 0, len(reports))
	seenImages := map[int64]bool{}

	for _, report := range reports {
		userIDs = append(userIDs, report.UserID)
		if report.ImageID != nil && !seenImages[*report.ImageID] {
			seenImages[*report.ImageID] = true
			imageIDs = append(imageIDs, *report.ImageID)
		}
	}

	reporters, err := s.repo.GetUsersByIDs(r.Context(), userIDs)
	if err != nil {
		return nil, err
	}

	visibility := repo.ChildVisibility{Filter: models.FilterAll}
	images, err := s.repo.GetImagesByIDs(r.Context(), imageIDs, visibility, includeUploaderID)
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

	for _, report := range reports {
		dto := reportDto{
			ID:           report.ID,
			UserID:       report.UserID,
			ImageID:      report.ImageID,
			Description:  optionalString(report.Description),
			Status:       report.Status.String(),
			ReviewerNote: report.ReviewerNote,
			CreatedAt:    report.CreatedAt,
		}

		if user, ok := reporters[report.UserID]; ok {
			dto.User = &userMinimalDto{ID: user.ID, Name: user.Name, AvatarURL: user.AvatarURL}
		}
		if report.ImageID != nil {
			if image, ok := imagesByID[*report.ImageID]; ok {
				dto.Image = &image
			}
		}

		out = append(out, dto)
	}
	return out, nil
}

// registerUserRoutes mounts GET /users.
//
// The remaining user routes live with their own verbs: GET /users/{id} is the
// profile read, and the role and ban changes are PUT because that is what
// Users.tsx sends.
func (s *Server) registerUserRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"users", s.handleListUsers)
	mux.HandleFunc("GET "+base+"users/{id}", s.handleGetUserByID)
	mux.HandleFunc("PUT "+base+"users/{id}/role", s.handleUpdateUserRole)
	mux.HandleFunc("PUT "+base+"users/{id}/ban", s.handleUpdateUserBan)
}

// handleListUsers serves GET /users.
//
// Admin only, because the listing exposes blacklist state and request counts.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "list users") {
		return
	}

	page, err := parsePagination(r, s.cfg.Pages.User)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("name"))

	// includedIds narrows to exact users, which is how the filter sidebar's user
	// picker resolves a numeric selection.
	var includedIDs []int64
	if raw := query.Get("includedIds"); raw != "" {
		includedIDs = parseIntList(r, "includedIds")
	}

	users, total, err := s.repo.ListUsers(r.Context(), repo.UserFilter{
		Name:       search,
		IncludedID: singleOrNil(includedIDs),
		Page:       page.Page,
		PageSize:   page.PageSize,
	})
	if err != nil {
		s.logError(r, "list users", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]userDto, 0, len(users))
	for _, user := range users {
		dtos = append(dtos, toUserDto(user))
	}

	// An includedIds lookup is not paginated: one page holds everything matched.
	if includedIDs != nil {
		page = exactLookupPage(page, len(dtos))
		total = int64(len(dtos))
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// userDto is UserDto.cs, the administrator's view of an account.
type userDto struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	DiscordID          string  `json:"discordId"`
	AvatarURL          *string `json:"avatarUrl"`
	Role               string  `json:"role"`
	IsBlacklisted      bool    `json:"isBlacklisted"`
	BlacklistReason    *string `json:"blacklistReason"`
	RequestCount       int64   `json:"requestCount"`
	APIKeyRequestCount int64   `json:"apiKeyRequestCount"`
	JWTRequestCount    int64   `json:"jwtRequestCount"`
	UploadedImageCount int64   `json:"uploadedImageCount"`
	AlbumImageCount    int64   `json:"albumImageCount"`
}

// toUserDto converts a row.
//
// The two derived counts are already computed by the listing query; they are not
// re-derived here because the per-user subqueries are the expensive part of that
// statement and repeating them per row would undo the batching.
func toUserDto(user repo.UserEntry) userDto {
	return userDto{
		ID:                 user.ID,
		Name:               user.Name,
		DiscordID:          user.DiscordID,
		AvatarURL:          user.AvatarURL,
		Role:               roleNameFor(user.Role),
		IsBlacklisted:      user.IsBlacklisted,
		BlacklistReason:    user.BlacklistReason,
		RequestCount:       user.RequestCount,
		APIKeyRequestCount: user.APIKeyRequestCount,
		JWTRequestCount:    user.JWTRequestCount,
		UploadedImageCount: user.UploadedImageCount,
		AlbumImageCount:    user.AlbumImageCount,
	}
}

// handleGetUserByID serves GET /users/{id}.
//
// The response is UserMinimalDto — id, name and avatar only — and the route is
// moderator-only, because AlbumPage needs to show who owns an album but must not
// learn that user's role, blacklist state or request counts. Those are only on
// GET /users, which is admin-only.
func (s *Server) handleGetUserByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleModerator, "view other users") {
		return
	}

	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	user, err := s.repo.GetUserByID(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get user", err, "User with ID "+itoa64(id)+" not found.")
		return
	}

	writeJSON(w, http.StatusOK, userMinimalDto{ID: user.ID, Name: user.Name, AvatarURL: user.AvatarURL})
}

// updateUserRoleRequest is the body of PUT /users/{id}/role.
type updateUserRoleRequest struct {
	Role string `json:"role"`
}

// handleUpdateUserRole serves PUT /users/{id}/role.
//
// An admin changing their own role is refused: the C# validator rejects it
// because a self-demotion can lock the instance out of its own moderation tools
// with no way back except direct SQL.
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "change user roles") {
		return
	}
	caller := s.principal(r)

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
		writeAPIError(w, r, validationError("Role", err.Error()))
		return
	}

	if targetID == caller.UserID {
		writeAPIError(w, r, apierror.BadRequest("You cannot change your own role."))
		return
	}

	updated, err := s.repo.UpdateUserRole(r.Context(), targetID, int(role))
	if err != nil {
		writeRepoError(w, r, "update user role", err, "User with ID "+itoa64(targetID)+" not found.")
		return
	}

	writeJSON(w, http.StatusOK, toUserDto(repo.UserEntry{User: updated}))
}

// updateUserBanRequest is the body of PUT /users/{id}/ban.
type updateUserBanRequest struct {
	IsBlacklisted   *bool   `json:"isBlacklisted"`
	BlacklistReason *string `json:"blacklistReason"`
}

// handleUpdateUserBan serves PUT /users/{id}/ban.
func (s *Server) handleUpdateUserBan(w http.ResponseWriter, r *http.Request) {
	if !s.requireRole(w, r, config.RoleAdmin, "ban users") {
		return
	}
	caller := s.principal(r)

	targetID, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	var body updateUserBanRequest
	if err := decodeJSONBody(r, &body); err != nil {
		writeAPIError(w, r, err)
		return
	}

	if body.IsBlacklisted == nil {
		writeAPIError(w, r, validationError("IsBlacklisted", "'isBlacklisted' is required."))
		return
	}

	// A self-ban would also lock the instance out, so it is refused outright.
	if targetID == caller.UserID {
		writeAPIError(w, r, apierror.BadRequest("You cannot ban yourself."))
		return
	}

	var reason *string
	if *body.IsBlacklisted && body.BlacklistReason != nil {
		reason = optionalString(*body.BlacklistReason)
	}

	updated, err := s.repo.UpdateUserBan(r.Context(), targetID, *body.IsBlacklisted, reason)
	if err != nil {
		writeRepoError(w, r, "update user ban", err, "User with ID "+itoa64(targetID)+" not found.")
		return
	}

	writeJSON(w, http.StatusOK, toUserDto(repo.UserEntry{User: updated}))
}

// singleOrNil returns the first element, or nil for an empty slice.
func singleOrNil(values []int64) *int64 {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}
