// Package background holds the periodic maintenance jobs the C# service runs as
// BackgroundService.
//
// # Why they exist
//
// Two tables grow without bound: DailyStat gains a row per day forever, and
// ReviewTask keeps a durable record of every decision forever. Both are
// retention problems rather than correctness problems, so both are handled here
// rather than on the request path.
//
// # Why they run on a timer rather than per request
//
// Neither is hot, and both delete rows. Doing them on a request would put an
// unbounded DELETE in the latency of an unlucky caller. A daily ticker means the
// worst case is one day's growth, which the database handles without noticing.
package background

import (
	"context"
	"fmt"
	"time"

	"waifuapi/internal/repo"
)

// Interval between cleanup runs.
//
// The C# service uses 24 hours for both jobs and the retention windows are
// measured in months, so an hourly tick would delete nothing extra while costing
// a database round trip every hour forever.
const cleanupInterval = 24 * time.Hour

// Logger is the small logging surface these jobs need.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Job is one periodic maintenance task.
type Job struct {
	// Name appears in the log line, so an operator reading logs can tell which
	// job failed without cross-referencing the stack.
	Name string

	// Run performs one pass. Returning an error is logged and ignored; the next
	// tick tries again.
	Run func(ctx context.Context) error
}

// Start runs every job on a daily ticker until ctx is cancelled.
//
// It returns immediately: the caller is a server that must keep serving, and a
// failing cleanup must never prevent startup or shutdown. Every error is logged
// and the loop continues, because a transient database problem at 3am must not
// permanently disable cleanup.
func Start(ctx context.Context, log Logger, jobs ...Job) {
	for _, job := range jobs {
		go runJob(ctx, log, job)
	}
}

// runJob drives one job: once at startup, then on a daily ticker.
//
// The first pass happens before the ticker starts because the C# services
// execute their body first and delay afterwards, so a freshly deployed instance
// prunes immediately. Ticking first would leave a day of growth sitting there
// after every restart, which is the kind of difference nobody notices until a
// retention window has been missed twice.
func runJob(ctx context.Context, log Logger, job Job) {
	runSafely(ctx, log, job)

	// A timer rather than a sleep-in-a-loop: this makes the interval explicit
	// and lets the select notice cancellation without waiting out the tick.
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("background job stopped", "job", job.Name)
			return
		case <-ticker.C:
			runSafely(ctx, log, job)
		}
	}
}

// runSafely performs one pass, turning both errors and panics into log lines.
//
// The recovery is the important part. Each C# job wraps its body in
// `catch (Exception ex)`, so a failure there costs one log line. In Go the
// situation is different: net/http recovers panics per handler goroutine, but
// these jobs are bare goroutines with nothing above them, so an unrecovered
// panic terminates the entire process.
//
// The blast radius is what makes this worth the four lines. A nightly cleanup
// that trips over an unexpected row would otherwise take the API down with it,
// and a monitoring check on "is the site up" would be reporting on a database
// maintenance task.
func runSafely(ctx context.Context, log Logger, job Job) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error("background job panicked", "job", job.Name,
				"panic", fmt.Sprint(recovered))
		}
	}()

	if err := job.Run(ctx); err != nil {
		log.Error("background job failed", "job", job.Name, "error", err.Error())
	}
}

// StatsCleanup prunes daily request counts older than a year.
//
// The table is "DailyStats", plural. That is not a typo in the oracle: the
// InitialCreate migration creates the table under that name
// (name: "DailyStats", PK_DailyStats) and the C# raw SQL in
// StatsCleanupService matches it. Referring to it in the singular compiles
// perfectly in Go and then fails against the production database on every tick,
// which is the kind of mistake that only shows up after deploy.
func StatsCleanup(data *repo.Repo, log Logger) Job {
	return Job{
		Name: "stats-cleanup",
		Run: func(ctx context.Context) error {
			cutoff := time.Now().UTC().AddDate(-1, 0, 0)

			deleted, err := data.DeleteDailyStatsBefore(ctx, cutoff)
			if err != nil {
				return err
			}
			if deleted > 0 {
				log.Info("pruned old daily stats", "count", deleted)
			}
			return nil
		},
	}
}

// ReviewTaskCleanup prunes resolved review tasks past the retention window.
//
// A retention of zero or less disables the job entirely rather than deleting
// everything: "unlimited" must not be spelled "delete all".
func ReviewTaskCleanup(data *repo.Repo, retentionDays int, log Logger) (Job, bool) {
	if retentionDays <= 0 {
		return Job{}, false
	}

	return Job{
		Name: "review-task-cleanup",
		Run: func(ctx context.Context) error {
			cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

			deleted, err := data.DeleteResolvedTasksBefore(ctx, cutoff)
			if err != nil {
				return err
			}
			if deleted > 0 {
				log.Info("pruned resolved review tasks",
					"count", deleted, "retention_days", retentionDays)
			}
			return nil
		},
	}, true
}
