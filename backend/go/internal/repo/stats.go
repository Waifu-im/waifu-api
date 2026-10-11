package repo

import (
	"context"
	"fmt"
	"time"

	"waifuapi/internal/store/models"
)

// This file implements request counting.
//
// # Why it is not optional
//
// /stats/public and /stats/admin both read counters that nothing else writes.
// Without this, those endpoints report zero forever: the home page shows "0
// images" and the moderation dashboard's request graph is a flat line, and
// there is no signal anywhere that counting is broken rather than genuinely
// idle.
//
// # Why the writes are deferred
//
// Counting is telemetry and must never affect the response. The C# middleware
// runs it in a fire-and-forget task after the response is written; this does the
// same, on a detached context, and swallows every error. A counter that cannot
// be written costs a lost data point, not a failed request.

// TotalRequestsKey is the GlobalStat row holding the lifetime request count.
const TotalRequestsKey = "TotalRequests"

// RecordDailyRequest increments today's DailyStat row, creating it if absent.
//
// The upsert is a single statement rather than select-then-insert. The C# version
// does the latter and catches the resulting unique-violation to recover, which
// is two round trips on the common path and racy by construction; ON CONFLICT
// does it in one and is atomic.
func (r *Repo) RecordDailyRequest(ctx context.Context, day time.Time) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO "DailyStats" ("Date", "RequestCount")
        VALUES ($1, 1)
        ON CONFLICT ("Date") DO UPDATE
            SET "RequestCount" = "DailyStats"."RequestCount" + 1`,
		day.UTC().Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("record daily request: %w", err)
	}
	return nil
}

// RecordTotalRequest increments the lifetime counter.
func (r *Repo) RecordTotalRequest(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO "GlobalStats" ("Key", "Value")
        VALUES ($1, 1)
        ON CONFLICT ("Key") DO UPDATE
            SET "Value" = "GlobalStats"."Value" + 1`, TotalRequestsKey)
	if err != nil {
		return fmt.Errorf("record total request: %w", err)
	}
	return nil
}

// UserRequestKinds says which per-user counters to advance.
//
// The two are tracked separately because the admin user list shows them, and the
// split is the only evidence of how much of the traffic is machine-to-machine
// through API keys versus humans through Discord.
type UserRequestKinds struct {
	// JWT is true when the caller authenticated with a Discord-issued token.
	JWT bool

	// APIKey is true when the caller presented an API key.
	APIKey bool
}

// RecordUserRequest increments an authenticated caller's counters.
//
// The three increments are one statement so a partial write cannot leave the
// total out of step with the per-kind counts.
func (r *Repo) RecordUserRequest(ctx context.Context, userID int64, kinds UserRequestKinds) error {
	jwtIncrement := 0
	if kinds.JWT {
		jwtIncrement = 1
	}
	apiKeyIncrement := 0
	if kinds.APIKey {
		apiKeyIncrement = 1
	}

	_, err := r.db.Exec(ctx, `
        UPDATE "Users"
        SET "RequestCount" = "RequestCount" + 1,
            "JwtRequestCount" = "JwtRequestCount" + $2,
            "ApiKeyRequestCount" = "ApiKeyRequestCount" + $3
        WHERE "Id" = $1`, userID, jwtIncrement, apiKeyIncrement)
	if err != nil {
		return fmt.Errorf("record user request: %w", err)
	}
	return nil
}

// DeleteDailyStatsBefore drops daily rows older than the cutoff.
//
// Retention is bounded rather than open-ended: the admin dashboard only charts
// the last 30 days, so keeping years of rows costs storage and buys nothing.
// A cutoff of zero or less keeps everything.
func (r *Repo) DeleteDailyStatsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "DailyStats" WHERE "Date" < $1`, cutoff.UTC().Format("2006-01-02"))
	if err != nil {
		return 0, fmt.Errorf("delete daily stats: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteResolvedTasksBefore prunes the review queue's audit history.
//
// Only resolved tasks are eligible: a pending task is live work, and deleting
// one would lose a submission nobody has looked at yet.
func (r *Repo) DeleteResolvedTasksBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `
        DELETE FROM "ReviewTasks"
        WHERE "Status" <> $1 AND "ResolvedAt" < $2`,
		int(models.TaskPending), cutoff.UTC())
	if err != nil {
		return 0, fmt.Errorf("delete resolved tasks: %w", err)
	}
	return tag.RowsAffected(), nil
}
