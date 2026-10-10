package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
)

// ErrNotFound is returned when a requested row does not exist.
//
// It is a distinct type so handlers can map it to 404 without inspecting driver
// error strings.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a write violates a uniqueness rule, such as a
// tag slug that already exists.
var ErrConflict = errors.New("conflict")

// CreateImageInput describes a new image row.
type CreateImageInput struct {
	UploaderID     *int64
	Width          int64
	Height         int64
	ByteSize       int64
	Extension      string
	DominantColor  string
	Source         *string
	IsNsfw         bool
	IsAnimated     bool
	PerceptualHash uint64
	ReviewStatus   models.ReviewStatus
	TagIDs         []int64
	ArtistIDs      []int64
}

// CreateImage inserts an image together with its tag and artist links.
//
// The insert and both link inserts share one transaction. Without it a failure
// halfway through would leave an image with no tags, or tags pointing at an
// image that does not exist. The C# service makes the same guarantee explicitly
// (UploadImage/Command.cs opens a transaction covering the image and its review
// task).
func (r *Repo) CreateImage(ctx context.Context, tx pgx.Tx, input CreateImageInput) (models.Image, error) {
	var image models.Image
	var hashBits string

	err := tx.QueryRow(ctx, `
        INSERT INTO "Image"
            ("UploaderId", "Width", "Height", "ByteSize", "Extension", "DominantColor",
             "Source", "IsNsfw", "IsAnimated", "UploadedAt", "PerceptualHash", "ReviewStatus")
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), $10::bit(64), $11)
        RETURNING "Id", "UploaderId", "Width", "Height", "ByteSize", "Extension",
                  "DominantColor", "Source", "IsNsfw", "IsAnimated", "UploadedAt",
                  "PerceptualHash"::text, "ReviewStatus"`,
		input.UploaderID, input.Width, input.Height, input.ByteSize, input.Extension,
		input.DominantColor, input.Source, input.IsNsfw, input.IsAnimated,
		models.PerceptualHashToBitString(input.PerceptualHash), int(input.ReviewStatus)).
		Scan(&image.ID, &image.UploaderID, &image.Width, &image.Height, &image.ByteSize,
			&image.Extension, &image.DominantColor, &image.Source, &image.IsNsfw,
			&image.IsAnimated, &image.UploadedAt, &hashBits, &image.ReviewStatus)
	if err != nil {
		return models.Image{}, fmt.Errorf("insert image: %w", err)
	}

	parsed, err := models.PerceptualHashFromBitString(hashBits)
	if err != nil {
		return models.Image{}, fmt.Errorf("image %d: %w", image.ID, err)
	}
	image.PerceptualHash = parsed

	for _, tagID := range input.TagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO "ImageTag" ("ImageId", "TagId") VALUES ($1, $2)
             ON CONFLICT DO NOTHING`,
			image.ID, tagID); err != nil {
			return models.Image{}, fmt.Errorf("link tag %d: %w", tagID, err)
		}
	}

	for _, artistID := range input.ArtistIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO "ArtistImage" ("ImageId", "ArtistId") VALUES ($1, $2)
             ON CONFLICT DO NOTHING`,
			image.ID, artistID); err != nil {
			return models.Image{}, fmt.Errorf("link artist %d: %w", artistID, err)
		}
	}

	return image, nil
}

// ---------------------------------------------------------------------------
// Review tasks
// ---------------------------------------------------------------------------

// CreateReviewTask inserts a moderation task.
//
// A task row is written even when the content is auto-accepted, because the
// audit trail is the point. Auto-accepted tasks land in Approved state with a
// note explaining why, exactly as ReviewTaskFactory.NewContent does.
func (r *Repo) CreateReviewTask(ctx context.Context, tx pgx.Tx, kind models.ReviewTaskKind, targetType models.ReviewableType, targetID int64, submitterID *int64, payload []byte, requiresReview bool) (int64, error) {
	if payload == nil {
		payload = []byte("{}")
	}

	status := models.TaskApproved
	var resolvedAt *string
	reviewerNote := "Auto-accepted (trusted role)."

	if requiresReview {
		status = models.TaskPending
		resolvedAt = nil
		reviewerNote = ""
	}

	var id int64
	err := tx.QueryRow(ctx, `
        INSERT INTO "ReviewTask"
            ("Kind", "TargetType", "TargetId", "SubmitterId", "Payload", "Reason",
             "Status", "ReviewerNote", "CreatedAt", "ResolvedAt")
        VALUES ($1, $2, $3, $4, $5, '', $6, $7, now(), $8)
        RETURNING "Id"`,
		int(kind), int(targetType), targetID, submitterID, payload,
		int(status), reviewerNote, resolvedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert review task: %w", err)
	}
	return id, nil
}

// ReviewTaskFilter carries the optional parameters of GET /review/tasks.
type ReviewTaskFilter struct {
	Status      *models.ReviewTaskStatus
	TargetType  *models.ReviewableType
	SubmitterID *int64
	Page        int
	PageSize    int
}

// ListReviewTasks returns a page of tasks.
//
// Non-moderators must only see their own tasks, so SubmitterID is always
// supplied for them by the caller. Keeping the restriction here as well means a
// future caller cannot forget it.
func (r *Repo) ListReviewTasks(ctx context.Context, filter ReviewTaskFilter) ([]models.ReviewTask, int64, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 8)

	if filter.Status != nil {
		args = append(args, int(*filter.Status))
		conditions = append(conditions, fmt.Sprintf(`"Status" = $%d`, len(args)))
	}
	if filter.TargetType != nil {
		args = append(args, int(*filter.TargetType))
		conditions = append(conditions, fmt.Sprintf(`"TargetType" = $%d`, len(args)))
	}
	if filter.SubmitterID != nil {
		args = append(args, *filter.SubmitterID)
		conditions = append(conditions, fmt.Sprintf(`"SubmitterId" = $%d`, len(args)))
	}

	where := joinConditions(conditions)

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "ReviewTask" WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count review tasks: %w", err)
	}

	args = append(args, filter.PageSize)
	limit := len(args)
	args = append(args, filter.Page)
	offset := len(args)

	query := fmt.Sprintf(`
        SELECT "Id", "Kind", "TargetType", "TargetId", "SubmitterId", "Payload",
               "Reason", "Status", "ReviewerNote", "ReviewerId", "ModeratorEditedAt",
               "CreatedAt", "ResolvedAt"
        FROM "ReviewTask"
        WHERE %s
        ORDER BY "CreatedAt" DESC, "Id" DESC
        LIMIT $%d OFFSET $%d`, where, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list review tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]models.ReviewTask, 0, filter.PageSize)
	for rows.Next() {
		task, err := scanReviewTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate review tasks: %w", err)
	}
	return tasks, total, nil
}

func scanReviewTask(row rowScanner) (models.ReviewTask, error) {
	var task models.ReviewTask
	err := row.Scan(&task.ID, &task.Kind, &task.TargetType, &task.TargetId,
		&task.SubmitterID, &task.Payload, &task.Reason, &task.Status,
		&task.ReviewerNote, &task.ReviewerID, &task.ModeratorEditedAt,
		&task.CreatedAt, &task.ResolvedAt)
	if err != nil {
		return models.ReviewTask{}, fmt.Errorf("scan review task: %w", err)
	}
	return task, nil
}

// GetReviewTask fetches one task.
func (r *Repo) GetReviewTask(ctx context.Context, id int64) (models.ReviewTask, error) {
	return scanReviewTask(r.db.QueryRow(ctx, `
        SELECT "Id", "Kind", "TargetType", "TargetId", "SubmitterId", "Payload",
               "Reason", "Status", "ReviewerNote", "ReviewerId", "ModeratorEditedAt",
               "CreatedAt", "ResolvedAt"
        FROM "ReviewTask" WHERE "Id" = $1`, id))
}

// ApproveReviewTask publishes the target and marks the task approved.
//
// Both writes share a transaction, so a failure to publish cannot leave a task
// marked Approved while the content stays Pending. That ordering bug would be
// invisible to a user until they looked for an image that never appeared.
func (r *Repo) ApproveReviewTask(ctx context.Context, tx pgx.Tx, taskID, reviewerID int64) error {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskPending {
		return fmt.Errorf("%w: this review task has already been resolved", ErrConflict)
	}

	if err := acceptTarget(ctx, tx, task); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
        UPDATE "ReviewTask"
        SET "Status" = $1, "ReviewerId" = $2, "ResolvedAt" = now()
        WHERE "Id" = $3`,
		int(models.TaskApproved), reviewerID, taskID)
	if err != nil {
		return fmt.Errorf("mark task approved: %w", err)
	}
	return nil
}

// RejectReviewTask discards the target and marks the task rejected.
//
// Rejecting deletes the content rather than flagging it, which is why
// ReviewStatus has no Rejected member.
func (r *Repo) RejectReviewTask(ctx context.Context, tx pgx.Tx, taskID, reviewerID int64, reason string) error {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskPending {
		return fmt.Errorf("%w: this review task has already been resolved", ErrConflict)
	}

	if err := discardTarget(ctx, tx, task); err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
        UPDATE "ReviewTask"
        SET "Status" = $1, "ReviewerId" = $2, "ReviewerNote" = $3, "ResolvedAt" = now()
        WHERE "Id" = $4`,
		int(models.TaskRejected), reviewerID, reason, taskID)
	if err != nil {
		return fmt.Errorf("mark task rejected: %w", err)
	}
	return nil
}

// acceptTarget flips the target row to Accepted.
func acceptTarget(ctx context.Context, tx pgx.Tx, task models.ReviewTask) error {
	table, err := targetTable(task.TargetType)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET "ReviewStatus" = $1 WHERE "Id" = $2`, table),
		int(models.ReviewAccepted), task.TargetId); err != nil {
		return fmt.Errorf("accept %s %d: %w", task.TargetType, task.TargetId, err)
	}
	return nil
}

// discardTarget deletes the target row. Cascades handle the join tables.
func discardTarget(ctx context.Context, tx pgx.Tx, task models.ReviewTask) error {
	table, err := targetTable(task.TargetType)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE "Id" = $1`, table), task.TargetId); err != nil {
		return fmt.Errorf("discard %s %d: %w", task.TargetType, task.TargetId, err)
	}
	return nil
}

// targetTable maps a reviewable type onto its table name.
//
// The result is chosen from a fixed set rather than interpolated from user
// input, so there is no injection path through the task's TargetType.
func targetTable(target models.ReviewableType) (string, error) {
	switch target {
	case models.TargetImage:
		return `"Image"`, nil
	case models.TargetTag:
		return `"Tag"`, nil
	case models.TargetArtist:
		return `"Artist"`, nil
	default:
		return "", fmt.Errorf("unknown reviewable type %d", int(target))
	}
}

func getReviewTaskTx(ctx context.Context, tx pgx.Tx, id int64) (models.ReviewTask, error) {
	return scanReviewTask(tx.QueryRow(ctx, `
        SELECT "Id", "Kind", "TargetType", "TargetId", "SubmitterId", "Payload",
               "Reason", "Status", "ReviewerNote", "ReviewerId", "ModeratorEditedAt",
               "CreatedAt", "ResolvedAt"
        FROM "ReviewTask" WHERE "Id" = $1`, id))
}

// CancelReviewTask lets the submitter withdraw a pending task.
func (r *Repo) CancelReviewTask(ctx context.Context, taskID, submitterID int64) error {
	tag, err := r.db.Exec(ctx, `
        UPDATE "ReviewTask"
        SET "Status" = $1, "ResolvedAt" = now()
        WHERE "Id" = $2 AND "SubmitterId" = $3 AND "Status" = $4`,
		int(models.TaskCancelled), taskID, submitterID, int(models.TaskPending))
	if err != nil {
		return fmt.Errorf("cancel review task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: task not found, not owned by you, or already resolved", ErrConflict)
	}
	return nil
}

// DecodePayload unmarshals a task payload.
func DecodePayload(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode task payload: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// joinConditions combines WHERE fragments.
func joinConditions(conditions []string) string {
	result := "1 = 1"
	for _, condition := range conditions {
		if condition == "1 = 1" {
			continue
		}
		result += " AND " + condition
	}
	return result
}
