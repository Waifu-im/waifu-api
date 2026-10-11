package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
)

// ReportFilter carries the optional parameters of GET /reports.
type ReportFilter struct {
	// UserID scopes the result to one reporter. Set for ordinary users, and for
	// a moderator who passed ?mine=true.
	UserID *int64

	Status   *models.ReportStatus
	Page     int
	PageSize int
}

// ErrReportHandled reports that a report has already reached a terminal status.
//
// It is separate from ErrNotFound because the two produce different responses:
// a missing report is 404, a resolved one is 409.
var ErrReportHandled = errors.New("report has already been handled")

// CreateReport files a report against an image.
//
// The image must exist; reporting a missing image would create a queue entry a
// moderator cannot act on.
//
// A second pending report from the same user for the same image is refused, so
// the moderation queue cannot be flooded by one person.
func (r *Repo) CreateReport(ctx context.Context, userID, imageID int64, description string) (models.Report, error) {
	if _, err := r.GetImageByID(ctx, imageID, false); err != nil {
		return models.Report{}, err
	}

	var open int
	err := r.db.QueryRow(ctx, `
        SELECT COUNT(*)
        FROM "Reports"
        WHERE "UserId" = $1 AND "ImageId" = $2 AND "Status" = $3`,
		userID, imageID, int(models.ReportPending)).Scan(&open)
	if err != nil {
		return models.Report{}, fmt.Errorf("check open report: %w", err)
	}
	if open > 0 {
		return models.Report{}, ErrConflict
	}

	return scanReport(r.db.QueryRow(ctx, `
        INSERT INTO "Reports" ("UserId", "ImageId", "Description", "Status", "CreatedAt")
        VALUES ($1, $2, $3, $4, now())
        RETURNING "Id", "UserId", "ImageId", "Description", "Status",
                  "ReviewerNote", "CreatedAt"`,
		userID, imageID, description, int(models.ReportPending)))
}

// ListReports returns a page of reports.
func (r *Repo) ListReports(ctx context.Context, filter ReportFilter) ([]models.Report, int64, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 4)

	if filter.UserID != nil {
		args = append(args, *filter.UserID)
		conditions = append(conditions, fmt.Sprintf(`"UserId" = $%d`, len(args)))
	}
	if filter.Status != nil {
		args = append(args, int(*filter.Status))
		conditions = append(conditions, fmt.Sprintf(`"Status" = $%d`, len(args)))
	}

	where := joinConditions(conditions)

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "Reports" WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	args = append(args, filter.PageSize)
	limit := len(args)
	args = append(args, filter.Page)
	offset := len(args)

	query := fmt.Sprintf(`
        SELECT "Id", "UserId", "ImageId", "Description", "Status", "ReviewerNote", "CreatedAt"
        FROM "Reports"
        WHERE %s
        ORDER BY "CreatedAt" DESC, "Id" DESC
        LIMIT $%d OFFSET $%d`, where, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	reports := make([]models.Report, 0, filter.PageSize)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}
	return reports, total, rows.Err()
}

func scanReport(row rowScanner) (models.Report, error) {
	var report models.Report
	err := row.Scan(&report.ID, &report.UserID, &report.ImageID, &report.Description,
		&report.Status, &report.ReviewerNote, &report.CreatedAt)
	if err != nil {
		return models.Report{}, fmt.Errorf("scan report: %w", err)
	}
	return report, nil
}

// GetReport fetches one report.
func (r *Repo) GetReport(ctx context.Context, id int64) (models.Report, error) {
	return scanReport(r.db.QueryRow(ctx, `
        SELECT "Id", "UserId", "ImageId", "Description", "Status", "ReviewerNote", "CreatedAt"
        FROM "Reports" WHERE "Id" = $1`, id))
}

// UpdateReportDescription edits the description of a report that is still open.
//
// The pending check is part of the WHERE clause rather than a separate read, so a
// report resolved concurrently by a moderator cannot be edited in the gap
// between the two statements.
func (r *Repo) UpdateReportDescription(ctx context.Context, id int64, description string) (models.Report, error) {
	report, err := scanReport(r.db.QueryRow(ctx, `
        UPDATE "Reports" SET "Description" = $1
        WHERE "Id" = $2 AND "Status" = $3
        RETURNING "Id", "UserId", "ImageId", "Description", "Status",
                  "ReviewerNote", "CreatedAt"`,
		description, id, int(models.ReportPending)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Report{}, ErrNotFound
		}
		return models.Report{}, fmt.Errorf("update report description: %w", err)
	}
	return report, nil
}

// SetReportStatus moves a pending report to a terminal status.
//
// Only a pending report can be transitioned, which is enforced in SQL. The
// reviewer note is recorded for an answered report and cleared for a
// cancellation, matching SetReportStatus/Command.cs.
func (r *Repo) SetReportStatus(ctx context.Context, id int64, target models.ReportStatus, note *string) error {
	value := any(nil)
	if target != models.ReportCancelled {
		value = note
	}

	tag, err := r.db.Exec(ctx, `
        UPDATE "Reports"
        SET "Status" = $1, "ReviewerNote" = $2
        WHERE "Id" = $3 AND "Status" = $4`,
		int(target), value, id, int(models.ReportPending))
	if err != nil {
		return fmt.Errorf("set report status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		// Either the report does not exist or it is already resolved. The
		// handler distinguishes the two so the status code matches.
		if _, err := r.GetReport(ctx, id); err != nil {
			return ErrNotFound
		}
		return ErrReportHandled
	}
	return nil
}

// UserFilter carries the optional parameters of GET /users.
type UserFilter struct {
	// Name matches either the display name (case-insensitively) or the Discord
	// id, which is what the filter sidebar's user picker relies on when someone
	// pastes an id rather than typing a name.
	Name string

	// IncludedID restricts the result to one user and switches off pagination.
	IncludedID *int64

	Page     int
	PageSize int
}

// UserEntry is a user row plus the two derived counts the listing reports.
type UserEntry struct {
	models.User

	// UploadedImageCount counts accepted images, not pending ones, so a user
	// with a large backlog of unreviewed uploads does not outrank one with
	// published work.
	UploadedImageCount int64

	// AlbumImageCount counts entries across all of the user's albums.
	AlbumImageCount int64
}

// userSelect is the projection shared by the listing and the exact lookup.
//
// The two counts are correlated subqueries rather than joins so they cannot
// multiply the row count.
const userSelect = `SELECT u."Id", u."DiscordId", u."Name", u."AvatarUrl", u."Role",
       u."RequestCount", u."JwtRequestCount", u."ApiKeyRequestCount",
       u."IsBlacklisted", u."BlacklistReason",
       (SELECT COUNT(*) FROM "Images" i
         WHERE i."UploaderId" = u."Id" AND i."ReviewStatus" = 1),
       (SELECT COUNT(*) FROM "AlbumItems" ai
         WHERE EXISTS (SELECT 1 FROM "Albums" a WHERE a."Id" = ai."AlbumId" AND a."UserId" = u."Id"))`

// ListUsers returns a page of users for administration.
func (r *Repo) ListUsers(ctx context.Context, filter UserFilter) ([]UserEntry, int64, error) {
	where := "TRUE"
	var args []any

	if filter.IncludedID != nil {
		args = append(args, *filter.IncludedID)
		where = fmt.Sprintf(`u."Id" = $%d`, len(args))
	} else if filter.Name != "" {
		args = append(args, "%"+strings.ToLower(filter.Name)+"%")
		where = fmt.Sprintf(
			`(LOWER(u."Name") LIKE $%d OR u."DiscordId" LIKE $%d)`, len(args), len(args))
	}

	if filter.IncludedID == nil {
		var total int64
		if err := r.db.QueryRow(ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM "Users" u WHERE %s`, where), args...).
			Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count users: %w", err)
		}

		args = append(args, filter.PageSize, filter.Page*filter.PageSize)
		entries, err := r.queryUsers(ctx, fmt.Sprintf(userSelect+`
            FROM "Users" u
            WHERE %s
            ORDER BY "UploadedImageCount" DESC
            LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
		if err != nil {
			return nil, 0, err
		}
		return entries, total, nil
	}

	entries, err := r.queryUsers(ctx, userSelect+` FROM "Users" u WHERE `+where, args...)
	if err != nil {
		return nil, 0, err
	}
	return entries, int64(len(entries)), nil
}

func (r *Repo) queryUsers(ctx context.Context, statement string, args ...any) ([]UserEntry, error) {
	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	entries := make([]UserEntry, 0, 32)
	for rows.Next() {
		var entry UserEntry
		if err := rows.Scan(&entry.ID, &entry.DiscordID, &entry.Name, &entry.AvatarURL,
			&entry.Role, &entry.RequestCount, &entry.JWTRequestCount,
			&entry.APIKeyRequestCount, &entry.IsBlacklisted, &entry.BlacklistReason,
			&entry.UploadedImageCount, &entry.AlbumImageCount); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetUsersByIDs loads several users at once, keyed by id.
//
// Reports embed the reporter, so this avoids one lookup per row on the
// moderation queue.
func (r *Repo) GetUsersByIDs(ctx context.Context, ids []int64) (map[int64]models.User, error) {
	out := map[int64]models.User{}
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "DiscordId", "Name", "AvatarUrl", "Role",
               "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
               "IsBlacklisted", "BlacklistReason"
        FROM "Users" WHERE "Id" = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get users by id: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL,
			&user.Role, &user.RequestCount, &user.JWTRequestCount,
			&user.APIKeyRequestCount, &user.IsBlacklisted, &user.BlacklistReason); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		out[user.ID] = user
	}
	return out, rows.Err()
}

// GetImagesByIDs loads several images at once, keyed by id.
//
// Unlike the gallery path this does not apply the accepted-only filter: the
// moderation queue needs to render a pending image, and the report screen
// needs to render whatever the report refers to.
func (r *Repo) GetImagesByIDs(ctx context.Context, ids []int64, visibility ChildVisibility, includeUploader bool) ([]models.Image, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT i."Id", i."UploaderId",
               i."Width", i."Height", i."ByteSize", i."Extension", i."DominantColor",
               i."Source", i."IsNsfw", i."IsAnimated", i."UploadedAt",
               i."PerceptualHash"::text, i."ReviewStatus"
        FROM "Images" i
        WHERE i."Id" = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get images by id: %w", err)
	}
	defer rows.Close()

	images := make([]models.Image, 0, len(ids))
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan image: %w", err)
		}
		// The uploader is selected for the scanner's benefit and then cleared,
		// because the callers that must not see it pass false rather than
		// needing a second projection.
		if !includeUploader {
			image.UploaderID = nil
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

// UpdateUserRole changes a user's role and returns the updated row.
func (r *Repo) UpdateUserRole(ctx context.Context, userID int64, role int) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        UPDATE "Users" SET "Role" = $1 WHERE "Id" = $2
        RETURNING "Id", "DiscordId", "Name", "AvatarUrl", "Role",
                  "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
                  "IsBlacklisted", "BlacklistReason"`, role, userID).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, fmt.Errorf("update user role: %w", err)
	}
	return user, nil
}

// AdminStats is the moderation dashboard aggregate.
type AdminStats struct {
	TotalRequests int64
	TotalImages   int64
	TotalTags     int64
	TotalArtists  int64
	TotalUsers    int64
	PendingTasks  int64
	OpenReports   int64
}

// DailyStat is one day of request counts.
type DailyStat struct {
	Date         time.Time
	RequestCount int64
}

// GetAdminStats aggregates the dashboard counters in a single round trip.
//
// Several independent counts are gathered in one statement rather than seven,
// because the dashboard is polled and each extra round trip costs a request on a
// 0.1-CPU instance.
func (r *Repo) GetAdminStats(ctx context.Context) (AdminStats, error) {
	var stats AdminStats

	err := r.db.QueryRow(ctx, `
        SELECT
            (SELECT COALESCE(SUM("RequestCount"), 0) FROM "DailyStats"),
            (SELECT COUNT(*) FROM "Images"),
            (SELECT COUNT(*) FROM "Tags"),
            (SELECT COUNT(*) FROM "Artists"),
            (SELECT COUNT(*) FROM "Users"),
            (SELECT COUNT(*) FROM "ReviewTasks" WHERE "Status" = $1),
            (SELECT COUNT(*) FROM "Reports"   WHERE "Status" = $2)`,
		int(models.TaskPending), int(models.ReportPending)).
		Scan(&stats.TotalRequests, &stats.TotalImages, &stats.TotalTags,
			&stats.TotalArtists, &stats.TotalUsers, &stats.PendingTasks, &stats.OpenReports)
	if err != nil {
		return AdminStats{}, fmt.Errorf("get admin stats: %w", err)
	}
	return stats, nil
}

// GetDailyStats returns request counts for the most recent days.
func (r *Repo) GetDailyStats(ctx context.Context, days int) ([]DailyStat, error) {
	rows, err := r.db.Query(ctx, `
        SELECT "Date", "RequestCount"
        FROM "DailyStats"
        WHERE "Date" > CURRENT_DATE - $1::int
        ORDER BY "Date" ASC`, days)
	if err != nil {
		return nil, fmt.Errorf("get daily stats: %w", err)
	}
	defer rows.Close()

	var out []DailyStat
	for rows.Next() {
		var day DailyStat
		if err := rows.Scan(&day.Date, &day.RequestCount); err != nil {
			return nil, fmt.Errorf("scan daily stat: %w", err)
		}
		out = append(out, day)
	}
	return out, rows.Err()
}
