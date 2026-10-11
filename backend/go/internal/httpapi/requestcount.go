package httpapi

import (
	"context"
	"net/http"
	"time"

	"waifuapi/internal/repo"
)

// requestCounter records the per-request telemetry that /stats/public and
// /stats/admin read.
//
// It is an interface so the middleware can be exercised without a database, and
// so the counting policy — which counters exist and who increments them — is
// stated once, here, rather than implied by SQL scattered through handlers.
type requestCounter interface {
	RecordDailyRequest(ctx context.Context, day time.Time) error
	RecordTotalRequest(ctx context.Context) error
	RecordUserRequest(ctx context.Context, userID int64, kinds repo.UserRequestKinds) error
}

// withRequestCounting increments the daily, lifetime and per-user counters.
//
// # Why it runs after the response
//
// The counter writes are three statements. Doing them inline would add their
// latency to every request and, worse, make an unrelated request fail because
// a counter row could not be written. So the work is deferred onto a detached
// context after the handler returns, exactly as the C# middleware's Task.Run
// does, and every error is swallowed.
//
// The consequence to accept: a process killed between the response and the
// write loses that request's count. That is the right trade — undercounting by
// a handful during a deploy is invisible, and failing requests to keep a
// counter exact would not be.
func (s *Server) withRequestCounting(counter requestCounter, log Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The caller is read before the handler runs because the principal
			// is populated by authentication middleware, which sits outside
			// this one, and the request context is not readable afterwards.
			userID, kinds := s.callerIdentity(r)

			next.ServeHTTP(w, r)

			if counter == nil {
				return
			}
			go recordRequest(r.Context(), counter, userID, kinds, log)
		})
	}
}

// recordRequest writes the three counters, ignoring every failure.
func recordRequest(parent context.Context, counter requestCounter, userID int64,
	kinds repo.UserRequestKinds, log Logger) {

	// WithoutCancel detaches from the request context so the writes survive the
	// client disconnecting. Cancel is what would otherwise abandon them
	// mid-flight on every slow mobile connection.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()

	if err := counter.RecordDailyRequest(ctx, now); err != nil {
		log.Error("record daily request", "error", err.Error())
	}
	if err := counter.RecordTotalRequest(ctx); err != nil {
		log.Error("record total request", "error", err.Error())
	}
	if userID != 0 {
		if err := counter.RecordUserRequest(ctx, userID, kinds); err != nil {
			log.Error("record user request", "error", err.Error())
		}
	}
}

// callerIdentity extracts the user id and authentication kind for counting.
//
// The kind split matters: it is the only way the admin user list can show how
// much traffic is machine-to-machine through API keys versus humans through
// Discord. An API key call is not also a JWT call, so the two are exclusive.
func (s *Server) callerIdentity(r *http.Request) (int64, repo.UserRequestKinds) {
	caller := s.principal(r)
	if caller == nil {
		return 0, repo.UserRequestKinds{}
	}

	if caller.ViaAPIKey {
		return caller.UserID, repo.UserRequestKinds{APIKey: true}
	}
	return caller.UserID, repo.UserRequestKinds{JWT: true}
}
