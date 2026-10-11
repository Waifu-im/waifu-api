package repo

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"waifuapi/internal/store/models"
)

// These tests cover the review transition decisions, which are the easiest part
// of the queue to get wrong and the most expensive when wrong:
//
//   - Approving an Edit must apply the payload. Flipping ReviewStatus instead is
//     a no-op on an already-accepted target, so the queue would report Approved
//     while the change silently never landed.
//   - Rejecting or cancelling an Edit must NOT delete the target. An Edit is a
//     proposal about live, published content; deleting it because somebody
//     disagreed with a suggested change would destroy the content.
//
// Both were real bugs in the first port.

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// scriptedExec records the statements a test run issued.
type scriptedExec struct {
	queries []string
	args    [][]any
}

// execDB is a DBQuerier whose QueryRow returns whatever the test scripted.
//
// pgx.Row is an interface, so a fake is easy to write; pgx.Rows is harder
// because it exposes cursor internals. These tests only exercise the decision
// logic, which runs against Exec and QueryRow.
type execDB struct {
	*scriptedExec

	// row is returned by QueryRow, or pgx.ErrNoRows when the task is missing.
	row func(dest ...any) error
}

func (db *execDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.queries = append(db.queries, sql)
	db.args = append(db.args, args)
	// One row affected, so a guard written as "no rows updated" is not taken.
	// A test that wants the not-found path can script it explicitly.
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (db *execDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errors.New("not used by these tests")
}

func (db *execDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return fakeRow{scan: db.row}
}

// Begin returns the double itself, which already records Exec calls. No test in
// this file opens a real transaction.
func (db *execDB) Begin(context.Context) (pgx.Tx, error) { return nil, nil }

// fakeRow is a pgx.Row backed by a scan function.
type fakeRow struct{ scan func(dest ...any) error }

func (r fakeRow) Scan(dest ...any) error { return r.scan(dest...) }

// taskRow builds a scan function returning one review task.
func taskRow(task models.ReviewTask) func(...any) error {
	return func(dest ...any) error {
		values := []any{
			task.ID, int(task.Kind), int(task.TargetType), task.TargetId,
			task.SubmitterID, task.Payload, task.Reason, int(task.Status),
			task.ReviewerNote, task.ReviewerID, task.ModeratorEditedAt,
			task.CreatedAt, task.ResolvedAt,
		}
		for i, d := range dest {
			if i >= len(values) {
				return errors.New("scan target mismatch")
			}
			if err := assign(d, values[i]); err != nil {
				return err
			}
		}
		return nil
	}
}

// assign writes a scanned value into a destination pointer.
func assign(dest, value any) error {
	switch typed := dest.(type) {
	case *int64:
		*typed = value.(int64)
	case *int:
		*typed = value.(int)
	case **int64:
		*typed = value.(*int64)
	case **string:
		*typed = value.(*string)
	case *models.ReviewTaskKind:
		*typed = models.ReviewTaskKind(value.(int))
	case *models.ReviewableType:
		*typed = models.ReviewableType(value.(int))
	case *models.ReviewTaskStatus:
		*typed = models.ReviewTaskStatus(value.(int))
	case *[]byte:
		*typed = value.([]byte)
	case *string:
		*typed = value.(string)
	case **time.Time:
		*typed = value.(*time.Time)
	case *time.Time:
		*typed = value.(time.Time)
	default:
		return errors.New("unhandled destination type")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Approve
// ---------------------------------------------------------------------------

// TestApproveAppliesEditPayload is the regression test for the silent no-op.
func TestApproveAppliesEditPayload(t *testing.T) {
	task := models.ReviewTask{
		ID:         7,
		Kind:       models.KindEdit,
		TargetType: models.TargetTag,
		TargetId:   12,
		Payload:    []byte(`{"name":"renamed"}`),
		Status:     models.TaskPending,
	}

	db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
	applied := false

	err := New(db).ApproveReviewTask(context.Background(), db, 7, 99,
		func(context.Context, Tx, models.ReviewTask) error {
			applied = true
			return nil
		})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	if !applied {
		t.Error("approving an Edit task must apply the stored payload; " +
			"flipping ReviewStatus is a no-op on already-accepted content")
	}

	if joined := strings.Join(db.queries, "\n"); strings.Contains(joined, `UPDATE "Tags" SET "ReviewStatus"`) {
		t.Error("approving an Edit must not publish the target; the edit is applied instead")
	}
}

// TestApprovePublishesNewContent guards the other branch: a new submission is
// published, not edited.
func TestApprovePublishesNewContent(t *testing.T) {
	task := models.ReviewTask{
		ID:         8,
		Kind:       models.KindNewContent,
		TargetType: models.TargetImage,
		TargetId:   99,
		Status:     models.TaskPending,
	}

	db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
	applied := false

	err := New(db).ApproveReviewTask(context.Background(), db, 8, 99,
		func(context.Context, Tx, models.ReviewTask) error {
			applied = true
			return nil
		})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	if applied {
		t.Error("a NewContent task has no edit payload to apply")
	}
	if !containsStatement(db.queries, `UPDATE "Images" SET "ReviewStatus"`) {
		t.Errorf("a NewContent task must publish its target, got: %v", db.queries)
	}
}

// TestApproveRefusesResolvedTask proves the pending guard survives.
func TestApproveRefusesResolvedTask(t *testing.T) {
	task := models.ReviewTask{ID: 9, Kind: models.KindNewContent, Status: models.TaskApproved}

	db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
	err := New(db).ApproveReviewTask(context.Background(), db, 9, 1,
		func(context.Context, Tx, models.ReviewTask) error { return nil })

	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected a conflict for an already-resolved task, got %v", err)
	}
	if len(db.queries) != 0 {
		t.Errorf("a refused approval must issue no writes, got %v", db.queries)
	}
}

// ---------------------------------------------------------------------------
// Reject and cancel
// ---------------------------------------------------------------------------

// TestRejectDoesNotDeleteEditTarget is the regression test for the data loss.
func TestRejectDoesNotDeleteEditTarget(t *testing.T) {
	for _, kind := range []models.ReviewTaskKind{models.KindEdit, models.KindNewContent} {
		name := "Edit"
		if kind == models.KindNewContent {
			name = "NewContent"
		}

		t.Run(name, func(t *testing.T) {
			task := models.ReviewTask{
				ID: 10, Kind: kind, TargetType: models.TargetImage,
				TargetId: 55, Status: models.TaskPending,
			}

			db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
			if err := New(db).RejectReviewTask(context.Background(), db, 10, 1, nil); err != nil {
				t.Fatalf("reject: %v", err)
			}

			deleted := containsStatement(db.queries, `DELETE FROM "Images"`)
			if kind == models.KindEdit && deleted {
				t.Error("rejecting an Edit must not delete live published content; " +
					"the task is closed and the proposal discarded")
			}
			if kind == models.KindNewContent && !deleted {
				t.Error("rejecting a NewContent task must delete the submission")
			}
		})
	}
}

// TestCancelLeavesEditTargetAlone mirrors the reject rule for withdrawal.
func TestCancelLeavesEditTargetAlone(t *testing.T) {
	task := models.ReviewTask{
		ID: 11, Kind: models.KindEdit, TargetType: models.TargetArtist,
		TargetId: 3, Status: models.TaskPending,
	}

	db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
	if _, err := New(db).CancelReviewTask(context.Background(), db, 11, 42); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if containsStatement(db.queries, `DELETE FROM "Artists"`) {
		t.Error("withdrawing an edit suggestion must not delete the artist")
	}
	if !containsStatement(db.queries, `SET "Status" = `) {
		t.Errorf("the task must still be closed, got: %v", db.queries)
	}
}

// TestCancelWithdrawsNewContentUpload is the counterpart.
func TestCancelWithdrawsNewContentUpload(t *testing.T) {
	task := models.ReviewTask{
		ID: 12, Kind: models.KindNewContent, TargetType: models.TargetImage,
		TargetId: 77, Status: models.TaskPending,
	}

	db := &execDB{scriptedExec: &scriptedExec{}, row: taskRow(task)}
	if _, err := New(db).CancelReviewTask(context.Background(), db, 12, 42); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if !containsStatement(db.queries, `DELETE FROM "Images"`) {
		t.Errorf("withdrawing an upload must remove the row, got: %v", db.queries)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var whitespace = regexp.MustCompile(`\s+`)

// containsStatement reports whether any issued statement matches, ignoring
// whitespace differences.
func containsStatement(queries []string, fragment string) bool {
	wanted := whitespace.ReplaceAllString(fragment, " ")
	for _, query := range queries {
		if strings.Contains(whitespace.ReplaceAllString(query, " "), wanted) {
			return true
		}
	}
	return false
}
