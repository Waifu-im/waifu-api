package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// The last three routes that had no coverage at all: GET/HEAD /ping,
// PUT /users/{id}/ban and PATCH /auth/api-keys/{id}.
//
// GET /ping turned out to carry two real divergences rather than being a
// trivial endpoint, which is the argument for testing the boring ones.

// TestPingShape pins the body and content type.
//
// PingController returns `Content("pong", "text/plain")`. An earlier port
// answered with JSON, which still returns 200 — so every status-code check
// passed while anything grepping the body for "pong" reported the service down.
func TestPingShape(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/ping", "", nil, "")
	body := readAll(t, resp)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping = %d, want 200", resp.StatusCode)
	}
	if string(body) != "pong" {
		t.Errorf("body = %q, want the plain text %q", body, "pong")
	}
	if ctype := resp.Header.Get("Content-Type"); !strings.HasPrefix(ctype, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ctype)
	}
}

// TestPingAnswersHead covers the explicit HEAD mapping.
//
// PingController carries [HttpHead] alongside [HttpGet] and the C# comment says
// why: attribute routing does not derive HEAD from GET, so monitors defaulting
// to HEAD would otherwise get a 404 and read it as a dead service. Go's
// ServeMux does not synthesise HEAD either.
func TestPingAnswersHead(t *testing.T) {
	h := newHarness(t)

	for _, method := range []string{"GET", "HEAD"} {
		resp := h.request(method, "/ping", "", nil, "")
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s /ping = %d, want 200", method, resp.StatusCode)
		}
	}
}

// TestPingNeedsNoAuth confirms the probe is open.
func TestPingNeedsNoAuth(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{"/ping", "/healthz"} {
		resp := h.request("GET", path, "", nil, "")
		resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("%s requires auth; the probe must be reachable when the database is down", path)
		}
	}
}

// userState reads one column of a user.
func (h *harness) userState(t *testing.T, id int64, column string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var value *string
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT `+column+`::text FROM "Users" WHERE "Id" = $1`, id).Scan(&value); err != nil {
		t.Fatalf("read %s of user %d: %v", column, id, err)
	}
	if value == nil {
		return ""
	}
	return *value
}

// banUser issues PUT /users/{id}/ban and returns status and body.
func (h *harness) banUser(t *testing.T, token string, id int64, body string) (int, []byte) {
	t.Helper()

	resp := h.request("PUT", "/users/"+itoa64(id)+"/ban", token, bytesReader(body), "application/json")
	return resp.StatusCode, readAll(t, resp)
}

// TestBanUser covers the happy path and the reason bookkeeping.
func TestBanUser(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(71, config.RoleAdmin)

	status, body := h.banUser(t, admin, 51,
		`{"isBlacklisted":true,"blacklistReason":"spamming reports"}`)
	if status != http.StatusOK {
		t.Fatalf("ban = %d: %s", status, body)
	}

	if got := h.userState(t, 51, `"IsBlacklisted"`); got != "true" {
		t.Errorf("IsBlacklisted = %q, want true", got)
	}
	if got := h.userState(t, 51, `"BlacklistReason"`); got != "spamming reports" {
		t.Errorf("BlacklistReason = %q, want the supplied reason", got)
	}
	if !jsonBool(t, body, "isBlacklisted") {
		t.Error("the response should reflect the new state")
	}
}

// TestUnbanClearsTheReason is the asymmetry in BanUserCommand.
//
// `user.BlacklistReason = request.IsBlacklisted ? request.Reason : null` means
// unbanning discards the reason even if the caller sent a new one. Leaving a
// stale reason on an unbanned user is the kind of thing that surfaces in a
// support ticket months later.
func TestUnbanClearsTheReason(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(71, config.RoleAdmin)

	if status, body := h.banUser(t, admin, 51,
		`{"isBlacklisted":true,"blacklistReason":"spamming"}`); status != http.StatusOK {
		t.Fatalf("ban = %d: %s", status, body)
	}

	// Unban, and deliberately send a reason that must be discarded.
	status, body := h.banUser(t, admin, 51,
		`{"isBlacklisted":false,"blacklistReason":"forgiven"}`)
	if status != http.StatusOK {
		t.Fatalf("unban = %d: %s", status, body)
	}

	if got := h.userState(t, 51, `"IsBlacklisted"`); got != "false" {
		t.Errorf("IsBlacklisted = %q, want false", got)
	}
	if got := h.userState(t, 51, `"BlacklistReason"`); got != "" {
		t.Errorf("BlacklistReason = %q, want NULL after unban", got)
	}
}

// TestBanUserIsAdminOnly covers the policy.
func TestBanUserIsAdminOnly(t *testing.T) {
	h := newHarness(t)

	body := `{"isBlacklisted":true,"blacklistReason":"x"}`

	resp := h.request("PUT", "/users/51/ban", "", bytesReader(body), "application/json")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous ban = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	for _, role := range []config.Role{config.RoleUser, config.RoleTrustedUser, config.RoleModerator} {
		resp = h.request("PUT", "/users/51/ban", h.issueToken(61, role), bytesReader(body), "application/json")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("role %d banning = %d, want 403", role, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestBanSelfIsRefused covers the guard in BanUserCommand.
//
// `if (request.UserId == _currentUserService.UserId) throw InvalidOperation`.
// An admin banning themselves would lock the instance out of its own
// moderation controls, so it is refused rather than allowed and regretted.
func TestBanSelfIsRefused(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(71, config.RoleAdmin)

	status, body := h.banUser(t, admin, 71,
		`{"isBlacklisted":true,"blacklistReason":"oops"}`)
	if status != http.StatusBadRequest {
		t.Errorf("self-ban = %d, want 400: %s", status, body)
	}
	if got := h.userState(t, 71, `"IsBlacklisted"`); got == "true" {
		t.Error("the refused self-ban still changed state")
	}
}

// TestBanUserValidation covers the required field and the 404.
func TestBanUserValidation(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(71, config.RoleAdmin)

	// IsBlacklisted is required; an absent pointer is a 400, not a default false.
	status, body := h.banUser(t, admin, 51, `{}`)
	if status != http.StatusBadRequest {
		t.Errorf("ban without isBlacklisted = %d, want 400: %s", status, body)
	}

	status, body = h.banUser(t, admin, 999999, `{"isBlacklisted":true}`)
	if status != http.StatusNotFound {
		t.Errorf("ban of a missing user = %d, want 404: %s", status, body)
	}
}

// updateAPIKey issues PATCH /auth/api-keys/{id}.
func (h *harness) updateAPIKey(t *testing.T, token string, id int64, body string) (int, []byte) {
	t.Helper()

	resp := h.request("PATCH", "/auth/api-keys/"+itoa64(id), token, bytesReader(body), "application/json")
	return resp.StatusCode, readAll(t, resp)
}

// keyID creates an API key and returns its id.
func (h *harness) keyID(t *testing.T, token string, description string) int64 {
	t.Helper()

	resp := h.request("POST", "/auth/api-keys", token,
		bytesReader(`{"description":"`+description+`"}`), "application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create api key = %d: %s", resp.StatusCode, body)
	}
	return int64(jsonIntOrZero(t, body, "id"))
}

// TestUpdateAPIKeyDescription covers the endpoint's one field.
func TestUpdateAPIKeyDescription(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(51, config.RoleUser)
	id := h.keyID(t, token, "before")

	status, body := h.updateAPIKey(t, token, id, `{"description":"after"}`)
	if status != http.StatusOK {
		t.Fatalf("update api key = %d: %s", status, body)
	}
	if got := jsonString(t, body, "description"); got != "after" {
		t.Errorf("description = %q, want after", got)
	}
}

// TestUpdateAPIKeyIsOwnerScoped is the security property.
//
// The query filters on both the key id and the caller, so another user's key
// reads as absent rather than as forbidden. Confirming it 404s rather than 403s
// matters: a 403 would confirm the key exists.
func TestUpdateAPIKeyIsOwnerScoped(t *testing.T) {
	h := newHarness(t)

	mine := h.keyID(t, h.issueToken(51, config.RoleUser), "mine")
	theirs := h.keyID(t, h.issueToken(52, config.RoleUser), "theirs")

	status, body := h.updateAPIKey(t, h.issueToken(52, config.RoleUser), mine, `{"description":"stolen"}`)
	if status != http.StatusNotFound {
		t.Errorf("updating someone else's key = %d, want 404: %s", status, body)
	}

	// The owner's key is untouched.
	listing := readAll(t, h.request("GET", "/auth/api-keys", h.issueToken(51, config.RoleUser), nil, "")) //nolint:bodyclose
	if !strings.Contains(string(listing), `"description":"mine"`) {
		t.Errorf("the key was renamed by another user: %s", listing)
	}

	_ = theirs
}

// TestUpdateAPIKeyValidatesDescription covers the empty case.
func TestUpdateAPIKeyValidatesDescription(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(51, config.RoleUser)
	id := h.keyID(t, token, "before")

	for _, body := range []string{`{"description":""}`, `{"description":"   "}`, `{}`} {
		status, payload := h.updateAPIKey(t, token, id, body)
		if status != http.StatusBadRequest {
			t.Errorf("update with %s = %d, want 400: %s", body, status, payload)
		}
	}
}

// TestUpdateAPIKeyRequiresAuth confirms the gate.
func TestUpdateAPIKeyRequiresAuth(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(51, config.RoleUser)
	id := h.keyID(t, token, "mine")

	resp := h.request("PATCH", "/auth/api-keys/"+itoa64(id), "",
		bytesReader(`{"description":"x"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous api key update = %d, want 401", resp.StatusCode)
	}
}
