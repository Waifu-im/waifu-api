package repo

import (
	"context"
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
func (r *Repo) CreateImage(ctx context.Context, tx Tx, input CreateImageInput) (models.Image, error) {
	var image models.Image
	var hashBits string

	err := tx.QueryRow(ctx, `
        INSERT INTO "Images"
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
			`INSERT INTO "ImageTag" ("ImagesId", "TagsId") VALUES ($1, $2)
             ON CONFLICT DO NOTHING`,
			image.ID, tagID); err != nil {
			return models.Image{}, fmt.Errorf("link tag %d: %w", tagID, err)
		}
	}

	for _, artistID := range input.ArtistIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO "ArtistImage" ("ImagesId", "ArtistsId") VALUES ($1, $2)
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
func (r *Repo) CreateReviewTask(ctx context.Context, tx Tx, kind models.ReviewTaskKind, targetType models.ReviewableType, targetID int64, submitterID *int64, payload []byte, requiresReview bool) (int64, error) {
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
        INSERT INTO "ReviewTasks"
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
	Kind        *models.ReviewTaskKind
	TargetType  *models.ReviewableType
	TargetID    *int64
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
	if filter.Kind != nil {
		args = append(args, int(*filter.Kind))
		conditions = append(conditions, fmt.Sprintf(`"Kind" = $%d`, len(args)))
	}
	if filter.TargetType != nil {
		args = append(args, int(*filter.TargetType))
		conditions = append(conditions, fmt.Sprintf(`"TargetType" = $%d`, len(args)))
	}
	if filter.TargetID != nil {
		args = append(args, *filter.TargetID)
		conditions = append(conditions, fmt.Sprintf(`"TargetId" = $%d`, len(args)))
	}
	if filter.SubmitterID != nil {
		args = append(args, *filter.SubmitterID)
		conditions = append(conditions, fmt.Sprintf(`"SubmitterId" = $%d`, len(args)))
	}

	where := joinConditions(conditions)

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "ReviewTasks" WHERE %s`, where), args...).
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
        FROM "ReviewTasks"
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
        FROM "ReviewTasks" WHERE "Id" = $1`, id))
}

// ApproveReviewTask publishes the target and marks the task approved.
//
// The two task kinds do different work here, and conflating them is a silent
// failure rather than a loud one:
//
//   - NewContent publishes the submitted entity by flipping its ReviewStatus.
//   - Edit applies the stored payload over the *already accepted* target. The
//     target's ReviewStatus is already Accepted, so flipping it would be a
//     no-op: the queue would say Approved while the proposed change silently
//     never landed.
//
// Both writes share the caller's transaction, so a failure cannot leave a task
// marked Approved while the content is unchanged.
func (r *Repo) ApproveReviewTask(ctx context.Context, tx Tx, taskID, reviewerID int64, apply EditApplier) error {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskPending {
		return fmt.Errorf("%w: this review task has already been resolved", ErrConflict)
	}

	switch task.Kind {
	case models.KindEdit:
		if err := apply(ctx, tx, task); err != nil {
			return err
		}
	case models.KindNewContent:
		if err := acceptTarget(ctx, tx, task); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown review task kind %d", int(task.Kind))
	}

	_, err = tx.Exec(ctx, `
        UPDATE "ReviewTasks"
        SET "Status" = $1, "ReviewerId" = $2, "ResolvedAt" = now()
        WHERE "Id" = $3`,
		int(models.TaskApproved), reviewerID, taskID)
	if err != nil {
		return fmt.Errorf("mark task approved: %w", err)
	}
	return nil
}

// RejectReviewTask discards the submission and marks the task rejected.
//
// Only a NewContent task deletes anything. An Edit task is a *proposal* about
// content that is already published and in use, so rejecting it closes the task
// and discards the proposal; deleting the target would destroy live content
// because somebody disagreed with a suggested edit.
//
// The task row survives either way: it is the durable record that a submission
// was made and refused.
func (r *Repo) RejectReviewTask(ctx context.Context, tx Tx, taskID, reviewerID int64, reason *string) error {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return err
	}

	if task.Status != models.TaskPending {
		return fmt.Errorf("%w: this review task has already been resolved", ErrConflict)
	}

	if task.Kind == models.KindNewContent {
		if err := discardTarget(ctx, tx, task); err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
        UPDATE "ReviewTasks"
        SET "Status" = $1, "ReviewerId" = $2, "ReviewerNote" = $3, "ResolvedAt" = now()
        WHERE "Id" = $4`,
		int(models.TaskRejected), reviewerID, reason, taskID)
	if err != nil {
		return fmt.Errorf("mark task rejected: %w", err)
	}
	return nil
}

// RejectedImageTarget reports the image a rejection deleted, so the caller can
// clean up the stored object.
//
// The storage write cannot happen inside the transaction — it is a network call
// and rolling it back is impossible — so it is deferred until after the commit,
// exactly as the C# service does by deleting from S3 before SaveChanges.
func (r *Repo) PendingDeletionFor(task models.ReviewTask) (int64, string, bool) {
	if task.Kind != models.KindNewContent || task.TargetType != models.TargetImage {
		return 0, "", false
	}
	return task.TargetId, "", true
}

// acceptTarget flips the target row to Accepted.
func acceptTarget(ctx context.Context, tx Tx, task models.ReviewTask) error {
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
func discardTarget(ctx context.Context, tx Tx, task models.ReviewTask) error {
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
		return `"Images"`, nil
	case models.TargetTag:
		return `"Tags"`, nil
	case models.TargetArtist:
		return `"Artists"`, nil
	default:
		return "", fmt.Errorf("unknown reviewable type %d", int(target))
	}
}

func getReviewTaskTx(ctx context.Context, tx Tx, id int64) (models.ReviewTask, error) {
	return scanReviewTask(tx.QueryRow(ctx, `
        SELECT "Id", "Kind", "TargetType", "TargetId", "SubmitterId", "Payload",
               "Reason", "Status", "ReviewerNote", "ReviewerId", "ModeratorEditedAt",
               "CreatedAt", "ResolvedAt"
        FROM "ReviewTasks" WHERE "Id" = $1`, id))
}

// EditApplier writes a stored edit payload to its target.
//
// It is passed into ApproveReviewTask rather than called from inside the
// repository because applying an edit is domain logic that lives in the HTTP
// layer's edit pipeline, while the repository owns the task row. Passing it in
// keeps the approve/reject decision — the part that is easy to get wrong — here,
// while the payload semantics stay where they are written.
type EditApplier func(ctx context.Context, tx Tx, task models.ReviewTask) error

// CancelReviewTask lets the submitter withdraw a pending task.
//
// As with rejection, a NewContent cancellation deletes the not-yet-public entity
// and its file, while an Edit cancellation only discards the proposal.
func (r *Repo) CancelReviewTask(ctx context.Context, tx Tx, taskID, submitterID int64) (models.ReviewTask, error) {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return models.ReviewTask{}, err
	}

	if task.Kind == models.KindNewContent {
		if err := discardTarget(ctx, tx, task); err != nil {
			return models.ReviewTask{}, err
		}
	}

	tag, err := tx.Exec(ctx, `
        UPDATE "ReviewTasks"
        SET "Status" = $1, "ResolvedAt" = now()
        WHERE "Id" = $2 AND "SubmitterId" = $3 AND "Status" = $4`,
		int(models.TaskCancelled), taskID, submitterID, int(models.TaskPending))
	if err != nil {
		return models.ReviewTask{}, fmt.Errorf("cancel review task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ReviewTask{}, fmt.Errorf(
			"%w: task not found, not owned by you, or already resolved", ErrConflict)
	}
	return task, nil
}

// SetImageEditableFields writes the two fields a metadata edit may change.
//
// Tag and artist membership is deliberately not here: changing what an image
// depicts goes through the review queue as an edit request, and the C# service
// keeps the two concerns in separate commands.
func (r *Repo) SetImageEditableFields(ctx context.Context, id int64, source *string, isNsfw bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE "Images" SET "Source" = $1, "IsNsfw" = $2 WHERE "Id" = $3`,
		source, isNsfw, id)
	if err != nil {
		return fmt.Errorf("update image: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResolvePendingTask approves an open NewContent task directly.
//
// This is what a moderator accepting from the catalogue rather than the queue
// does: the content becomes visible and the queue entry is closed in the same
// step, so the same submission does not appear twice for the next moderator.
func (r *Repo) ResolvePendingTask(ctx context.Context, kind models.ReviewTaskKind, target models.ReviewableType, targetID int64) error {
	_, err := r.db.Exec(ctx, `
        UPDATE "ReviewTasks"
        SET "Status" = $1, "ResolvedAt" = now()
        WHERE "Kind" = $2 AND "TargetType" = $3 AND "TargetId" = $4 AND "Status" = $5`,
		int(models.TaskApproved), int(kind), int(target), targetID, int(models.TaskPending))
	if err != nil {
		return fmt.Errorf("resolve pending task: %w", err)
	}
	return nil
}

// HasOpenEditForTarget reports whether the submitter already has a pending edit
// on this target.
//
// One open edit per (user, target) stops a single user queueing the same change
// repeatedly, which would otherwise show up in the queue several times.
func (r *Repo) HasOpenEditForTarget(ctx context.Context, submitterID int64, target models.ReviewableType, targetID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1 FROM "ReviewTasks"
            WHERE "Kind" = $1 AND "SubmitterId" = $2 AND "TargetType" = $3
              AND "TargetId" = $4 AND "Status" = $5)`,
		int(models.KindEdit), submitterID, int(target), targetID, int(models.TaskPending)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check open edit: %w", err)
	}
	return exists, nil
}

// CountOpenEdits reports how many pending edits the user has in total.
func (r *Repo) CountOpenEdits(ctx context.Context, submitterID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM "ReviewTasks"
        WHERE "Kind" = $1 AND "SubmitterId" = $2 AND "Status" = $3`,
		int(models.KindEdit), submitterID, int(models.TaskPending)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count open edits: %w", err)
	}
	return count, nil
}

// CreateEditTask inserts an Edit task and returns its id.
//
// status is passed rather than derived because a caller whose role bypasses edit
// review writes the task already approved, in the same transaction as the apply.
func (r *Repo) CreateEditTask(ctx context.Context, tx Tx, target models.ReviewableType, targetID, submitterID int64, payload []byte, reason *string, status models.ReviewTaskStatus, reviewerNote string, resolvedAt *string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
        INSERT INTO "ReviewTasks"
            ("Kind", "TargetType", "TargetId", "SubmitterId", "Payload", "Reason",
             "Status", "ReviewerNote", "CreatedAt", "ResolvedAt")
        VALUES ($1, $2, $3, $4, $5, COALESCE($6, ''), $7, $8, now(), $9)
        RETURNING "Id"`,
		int(models.KindEdit), int(target), targetID, submitterID, payload, reason,
		int(status), reviewerNote, resolvedAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert edit task: %w", err)
	}
	return id, nil
}

// UpdateReviewTaskContent replaces a task's payload and optionally stamps the
// moderator-edit marker.
func (r *Repo) UpdateReviewTaskContent(ctx context.Context, id int64, payload []byte, moderatorEdited bool) error {
	var query string
	if moderatorEdited {
		query = `UPDATE "ReviewTasks" SET "Payload" = $1, "ModeratorEditedAt" = now() WHERE "Id" = $2`
	} else {
		query = `UPDATE "ReviewTasks" SET "Payload" = $1 WHERE "Id" = $2`
	}

	if _, err := r.db.Exec(ctx, query, payload, id); err != nil {
		return fmt.Errorf("update review task content: %w", err)
	}
	return nil
}

// CurrentTagFields reads a tag's name and description.
//
// The edit applier needs both because the underlying update treats an omitted
// field as "clear it", so anything the payload does not mention must be passed
// back explicitly.
func (r *Repo) CurrentTagFields(ctx context.Context, id int64) (name, description string, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT "Name", "Description" FROM "Tags" WHERE "Id" = $1`, id).
		Scan(&name, &description)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("read tag fields: %w", err)
	}
	return name, description, nil
}

// ApplyImageEdit writes an image edit payload to its target.
//
// Tag and artist membership is expressed as add/remove deltas, which are
// resolved against the current links. Passing the resulting full sets keeps the
// operation a single replace rather than a read-modify-write race.
func (r *Repo) ApplyImageEdit(ctx context.Context, tx Tx, id int64, payload ImageEditFields) error {
	image, err := r.GetImageByID(ctx, id, true)
	if err != nil {
		return err
	}

	source := image.Source
	if payload.Source != nil {
		source = payload.Source
	}
	isNsfw := image.IsNsfw
	if payload.IsNsfw != nil {
		isNsfw = *payload.IsNsfw
	}

	var replaceTags bool
	tagIDs := []int64{}
	if payload.AddTagSlugs != nil || payload.RemoveTagSlugs != nil {
		replaceTags = true
		current, err := r.tagIDsForImage(ctx, id)
		if err != nil {
			return err
		}
		set := toSet(current)
		for _, slug := range payload.AddTagSlugs {
			found, err := r.tagIDBySlug(ctx, slug)
			if err != nil {
				return err
			}
			set[found] = true
		}
		for _, slug := range payload.RemoveTagSlugs {
			found, err := r.tagIDBySlug(ctx, slug)
			if err != nil {
				return err
			}
			delete(set, found)
		}
		tagIDs = fromSet(set)
	}

	var replaceArtists bool
	artistIDs := []int64{}
	if payload.AddArtistIDs != nil || payload.RemoveArtistIDs != nil {
		replaceArtists = true
		current, err := r.artistIDsForImage(ctx, id)
		if err != nil {
			return err
		}
		set := toSet(current)
		for _, added := range payload.AddArtistIDs {
			set[added] = true
		}
		for _, removed := range payload.RemoveArtistIDs {
			delete(set, removed)
		}
		artistIDs = fromSet(set)
	}

	return replaceImageRelations(ctx, tx, id, source, isNsfw, tagIDs, artistIDs,
		replaceTags, replaceArtists)
}

func (r *Repo) tagIDsForImage(ctx context.Context, imageID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT "TagsId" FROM "ImageTag" WHERE "ImagesId" = $1`, imageID)
	if err != nil {
		return nil, fmt.Errorf("read image tags: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan image tag id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repo) artistIDsForImage(ctx context.Context, imageID int64) ([]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT "ArtistsId" FROM "ArtistImage" WHERE "ImagesId" = $1`, imageID)
	if err != nil {
		return nil, fmt.Errorf("read image artists: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan image artist id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// tagIDBySlug resolves a slug, failing when it does not exist.
//
// The edit path treats an unknown slug as a 404 rather than silently ignoring
// it, so a typo in a proposal is visible instead of producing a partial change.
func (r *Repo) tagIDBySlug(ctx context.Context, slug string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT "Id" FROM "Tags" WHERE "Slug" = $1`, slug).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("tag slug %q not found: %w", slug, err)
	}
	return id, nil
}

// replaceImageRelations writes the editable fields and, optionally, replaces the
// tag and artist link sets.
//
// The two "replace" flags exist because a payload may legitimately leave
// membership alone; without them an edit that only fixed a source URL would
// silently drop every tag. The writes join the caller's transaction so an edit
// and the task row that records it commit together.
func replaceImageRelations(ctx context.Context, tx Tx, id int64, source *string, isNsfw bool, tagIDs, artistIDs []int64, replaceTags, replaceArtists bool) error {
	tag, err := tx.Exec(ctx, `UPDATE "Images" SET "Source" = $1, "IsNsfw" = $2 WHERE "Id" = $3`,
		source, isNsfw, id)
	if err != nil {
		return fmt.Errorf("update image: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	if replaceTags {
		if _, err := tx.Exec(ctx, `DELETE FROM "ImageTag" WHERE "ImagesId" = $1`, id); err != nil {
			return fmt.Errorf("clear image tags: %w", err)
		}
		for _, tagID := range tagIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO "ImageTag" ("ImagesId", "TagsId") VALUES ($1, $2)`,
				id, tagID); err != nil {
				return fmt.Errorf("link tag %d: %w", tagID, err)
			}
		}
	}

	if replaceArtists {
		if _, err := tx.Exec(ctx, `DELETE FROM "ArtistImage" WHERE "ImagesId" = $1`, id); err != nil {
			return fmt.Errorf("clear image artists: %w", err)
		}
		for _, artistID := range artistIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO "ArtistImage" ("ImagesId", "ArtistsId") VALUES ($1, $2)`,
				id, artistID); err != nil {
				return fmt.Errorf("link artist %d: %w", artistID, err)
			}
		}
	}
	return nil
}

// toSet and fromSet convert between a slice and a set while keeping a stable
// order, so the SQL that replaces the links is deterministic in tests.
func toSet[T comparable](values []T) map[T]bool {
	set := make(map[T]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func fromSet[T comparable](set map[T]bool) []T {
	out := make([]T, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	return out
}

// ImageEditFields is a stored image edit payload.
//
// It mirrors ImageEditPayload in the C# service. Source is a pointer because nil
// means "leave unchanged" while an empty string means "clear it", and the tag and
// artist lists are nil when the payload does not touch membership at all.
type ImageEditFields struct {
	Source          *string
	IsNsfw          *bool
	AddTagSlugs     []string
	RemoveTagSlugs  []string
	AddArtistIDs    []int64
	RemoveArtistIDs []int64
}

// joinConditions combines WHERE fragments into one predicate.
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

// CancelReviewTaskAsModerator cancels any pending task, regardless of owner.
//
// A moderator withdrawing someone else's submission records themselves as the
// reviewer, so the queue shows who actually closed it.
func (r *Repo) CancelReviewTaskAsModerator(ctx context.Context, tx Tx, taskID, moderatorID int64) (models.ReviewTask, error) {
	task, err := getReviewTaskTx(ctx, tx, taskID)
	if err != nil {
		return models.ReviewTask{}, err
	}

	if task.Kind == models.KindNewContent {
		if err := discardTarget(ctx, tx, task); err != nil {
			return models.ReviewTask{}, err
		}
	}

	tag, err := tx.Exec(ctx, `
        UPDATE "ReviewTasks"
        SET "Status" = $1, "ReviewerId" = $2, "ResolvedAt" = now()
        WHERE "Id" = $3 AND "Status" = $4`,
		int(models.TaskCancelled), moderatorID, taskID, int(models.TaskPending))
	if err != nil {
		return models.ReviewTask{}, fmt.Errorf("cancel review task as moderator: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ReviewTask{}, ErrConflict
	}
	return task, nil
}
