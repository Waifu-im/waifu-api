package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
)

// ReportFilter carries the optional parameters of GET /reports.
type ReportFilter struct {
	// UserID scopes the result to one reporter. Set for ordinary users.
	UserID *int64

	Status   *models.ReportStatus
	Page     int
	PageSize int
}

// CreateReport files a report against an image.
//
// The image must exist; reporting a missing image would create a queue entry a
// moderator cannot act on.
func (r *Repo) CreateReport(ctx context.Context, userID, imageID int64, description string) (models.Report, error) {
	if _, err := r.GetImageByID(ctx, imageID, false); err != nil {
		return models.Report{}, err
	}

	return scanReport(r.db.QueryRow(ctx, `
        INSERT INTO "Report" ("UserId", "ImageId", "Description", "Status", "CreatedAt")
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
		fmt.Sprintf(`SELECT COUNT(*) FROM "Report" WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	args = append(args, filter.PageSize)
	limit := len(args)
	args = append(args, filter.Page)
	offset := len(args)

	query := fmt.Sprintf(`
        SELECT "Id", "UserId", "ImageId", "Description", "Status", "ReviewerNote", "CreatedAt"
        FROM "Report"
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
        FROM "Report" WHERE "Id" = $1`, id))
}

// UpdateReport resolves a report.
func (r *Repo) UpdateReport(ctx context.Context, id int64, status models.ReportStatus, note *string) (models.Report, error) {
	report, err := scanReport(r.db.QueryRow(ctx, `
        UPDATE "Report"
        SET "Status" = $1, "ReviewerNote" = $2
        WHERE "Id" = $3
        RETURNING "Id", "UserId", "ImageId", "Description", "Status",
                  "ReviewerNote", "CreatedAt"`, int(status), note, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return models.Report{}, ErrNotFound
		}
		return models.Report{}, fmt.Errorf("update report: %w", err)
	}
	return report, nil
}

// DeleteReport removes a report and reports how many rows were affected.
//
// Ownership is checked in SQL. isModerator allows deletion of any report, which
// is how a moderator clears a bogus queue entry.
func (r *Repo) DeleteReport(ctx context.Context, id, userID int64, isModerator bool) (int64, error) {
	query := `DELETE FROM "Report" WHERE "Id" = $1 AND "UserId" = $2`
	if isModerator {
		query = `DELETE FROM "Report" WHERE "Id" = $1`
	}

	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return 0, fmt.Errorf("delete report: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ListUsers returns a page of users, for administration.
func (r *Repo) ListUsers(ctx context.Context, offset, limit int) ([]models.User, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM "User"`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "DiscordId", "Name", "AvatarUrl", "Role",
               "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
               "IsBlacklisted", "BlacklistReason"
        FROM "User"
        ORDER BY "Id" ASC
        LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]models.User, 0, limit)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL,
			&user.Role, &user.RequestCount, &user.JWTRequestCount,
			&user.APIKeyRequestCount, &user.IsBlacklisted, &user.BlacklistReason); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

// UpdateUserRole changes a user's role and returns the updated row.
func (r *Repo) UpdateUserRole(ctx context.Context, userID int64, role int) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        UPDATE "User" SET "Role" = $1 WHERE "Id" = $2
        RETURNING "Id", "DiscordId", "Name", "AvatarUrl", "Role",
                  "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
                  "IsBlacklisted", "BlacklistReason"`, role, userID).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		if err == pgx.ErrNoRows {
			return models.User{}, ErrNotFound
		}
		return models.User{}, fmt.Errorf("update user role: %w", err)
	}
	return user, nil
}
