package background

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// entry is one captured log line: the message plus its structured key/values.
//
// The attributes are kept rather than discarded because they carry the job name
// and the panic value. Asserting only on the message would pass even if every
// line were stripped of its context, which is precisely the failure mode worth
// guarding: a log full of "background job panicked" and no indication of which
// job is useless at 3am.
type entry struct {
	msg  string
	args []any
}

func (e entry) String() string {
	var b strings.Builder
	b.WriteString(e.msg)
	for i := 0; i+1 < len(e.args); i += 2 {
		b.WriteString(" ")
		b.WriteString(stringify(e.args[i]))
		b.WriteString("=")
		b.WriteString(stringify(e.args[i+1]))
	}
	return b.String()
}

func stringify(v any) string {
	return fmt.Sprint(v)
}

// recordingLog captures what the jobs logged.
type recordingLog struct {
	mu    sync.Mutex
	infos []entry
	errs  []entry
}

func (l *recordingLog) Info(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, entry{msg: msg, args: args})
}

func (l *recordingLog) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, entry{msg: msg, args: args})
}

func (l *recordingLog) errors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.errs))
	for _, e := range l.errs {
		out = append(out, e.String())
	}
	return out
}

// TestRunSafelySurvivesPanic is the regression test for the process-exit bug.
//
// These jobs are the only bare goroutines in the service. net/http recovers
// panics in handler goroutines, so a panic on the request path costs one
// request; a panic here with nothing above it terminates the whole binary.
//
// Both C# cleanup services wrap their body in `catch (Exception ex)`, so a
// failure there costs a log line. Without the recovery this Go port turned the
// same failure into an outage.
func TestRunSafelySurvivesPanic(t *testing.T) {
	log := &recordingLog{}

	runSafely(context.Background(), log, Job{
		Name: "panicking-job",
		Run:  func(context.Context) error { panic("boom") },
	})

	errs := log.errors()
	if len(errs) != 1 {
		t.Fatalf("expected exactly one logged error, got %v", errs)
	}
	if !strings.Contains(errs[0], "panicked") {
		t.Errorf("want the panic to be logged as a panic, got %q", errs[0])
	}
	if !strings.Contains(errs[0], "panicking-job") {
		t.Errorf("the log must name the job, or an operator cannot find it: %q", errs[0])
	}
}

// TestRunSafelyLogsErrors confirms a returned error is still handled and does
// not escalate to a panic report.
func TestRunSafelyLogsErrors(t *testing.T) {
	log := &recordingLog{}

	runSafely(context.Background(), log, Job{
		Name: "failing-job",
		Run:  func(context.Context) error { return errors.New("db down") },
	})

	errs := log.errors()
	if len(errs) != 1 {
		t.Fatalf("expected one logged error, got %v", errs)
	}
	if !strings.Contains(errs[0], "failed") {
		t.Errorf("want a failure log, got %q", errs[0])
	}
	if strings.Contains(errs[0], "panicked") {
		t.Errorf("an error is not a panic: %q", errs[0])
	}
}

// TestRunSafelyQuietOnSuccess keeps a healthy job from writing noise.
func TestRunSafelyQuietOnSuccess(t *testing.T) {
	log := &recordingLog{}

	runSafely(context.Background(), log, Job{
		Name: "healthy-job",
		Run:  func(context.Context) error { return nil },
	})

	if errs := log.errors(); len(errs) != 0 {
		t.Errorf("a successful pass should log nothing, got %v", errs)
	}
}

// TestRunJobRunsImmediatelyBeforeTicking pins the startup ordering.
//
// Both C# services execute their body first and delay afterwards. If the ticker
// ran first, a fresh deployment would not prune for 24 hours, which is
// invisible until a retention window gets missed twice.
func TestRunJobRunsImmediatelyBeforeTicking(t *testing.T) {
	runs := make(chan struct{}, 4)
	log := &recordingLog{}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runJob(ctx, log, Job{
			Name: "startup-job",
			Run: func(context.Context) error {
				runs <- struct{}{}
				return nil
			},
		})
	}()

	select {
	case <-runs:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatal("the job did not run at startup; cleanupInterval would delay it by 24h")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runJob did not stop when its context was cancelled")
	}

	// Cancellation is logged so a shutdown is distinguishable from a crash.
	log.mu.Lock()
	defer log.mu.Unlock()
	stopped := false
	for _, e := range log.infos {
		if strings.Contains(e.String(), "background job stopped") {
			stopped = true
		}
	}
	if !stopped {
		t.Errorf("want a stop line on shutdown, got %v", log.infos)
	}
}

// TestStartIsNonBlocking confirms Start returns immediately.
//
// Start launches goroutines; if it blocked, the caller could never finish
// booting and the server would never start listening.
func TestStartIsNonBlocking(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		Start(context.Background(), &recordingLog{}, Job{
			Name: "blocking-job",
			Run: func(context.Context) error {
				close(started)
				<-release
				return nil
			},
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("Start blocked; it must return so the server can keep booting")
	}

	<-started
	close(release)
}
