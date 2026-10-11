package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"

	"waifuapi/internal/config"
)

// Coverage for two single-item read endpoints that had none: GET /users/{id}
// and GET /review/tasks/{id}.
//
// Both are the kind of handler that returns 200 with a plausible body while
// quietly omitting fields, which is exactly what the tag lookups were doing.
// Neither is reached by the listing tests, because those hit different routes.

// TestGetUserByIDReturnsTheMinimalDto pins the narrower shape on purpose.
//
// GET /users/{id} returns UserMinimalDto, not the administrative UserDto that
// the listing returns: id, name and avatar only. Emitting the counters and the
// blacklist reason here would hand per-user activity data to any moderator,
// which is more than that endpoint is scoped for.
func TestGetUserByIDReturnsTheMinimalDto(t *testing.T) {
	h := newHarness(t)

	moderatorToken := h.issueToken(2, config.RoleModerator)
	resp := h.request(http.MethodGet, "/users/2", moderatorToken, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)

	for _, field := range []string{"id", "name", "avatarUrl"} {
		if !jsonHas(payload, field) {
			t.Errorf("minimal user dto is missing %q", field)
		}
	}
	for _, field := range []string{
		"discordId", "role", "uploadedImageCount", "albumImageCount",
		"jwtRequestCount", "apiKeyRequestCount", "requestCount",
		"isBlacklisted", "blacklistReason",
	} {
		if jsonHas(payload, field) {
			t.Errorf("minimal user dto unexpectedly carries %q", field)
		}
	}

	if got := jsonIntOrZero(t, payload, "id"); got != 2 {
		t.Errorf("id = %d, want 2", got)
	}
}

// TestGetUserByIDRequiresModerator distinguishes this endpoint from the listing.
//
// The listing is Admin-only; the single lookup is Moderator, because
// UsersController guards GET {id} with the Moderator policy and the listing
// with Admin. An ordinary contributor must still be refused.
func TestGetUserByIDRequiresModerator(t *testing.T) {
	h := newHarness(t)

	for _, role := range []config.Role{config.RoleUser, config.RoleTrustedUser} {
		t.Run(role.String(), func(t *testing.T) {
			token := h.issueToken(3, role)
			resp := h.request(http.MethodGet, "/users/2", token, nil, "")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want 403 for %s", resp.StatusCode, role)
			}
		})
	}

	t.Run("moderator is allowed", func(t *testing.T) {
		resp := h.request(http.MethodGet, "/users/2", h.issueToken(2, config.RoleModerator), nil, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200 for a Moderator", resp.StatusCode)
		}
	})

	t.Run("anonymous", func(t *testing.T) {
		resp := h.request(http.MethodGet, "/users/2", "", nil, "")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 without a token", resp.StatusCode)
		}
	})
}

func TestGetUserByIDNotFound(t *testing.T) {
	h := newHarness(t)

	adminToken := h.issueToken(2, config.RoleModerator)
	resp := h.request(http.MethodGet, "/users/999999", adminToken, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for an unknown user", resp.StatusCode)
	}
}

func TestGetUserByIDRejectsNonNumericID(t *testing.T) {
	h := newHarness(t)

	moderatorToken := h.issueToken(2, config.RoleModerator)
	resp := h.request(http.MethodGet, "/users/not-a-number", moderatorToken, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a non-numeric id", resp.StatusCode)
	}
}

// TestGetReviewTaskReturnsTargetAndSubmitter covers the single-task read.
func TestGetReviewTaskReturnsTargetAndSubmitter(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	tagID := ensureTag(t, h, "reviewable")

	resp := h.submitEdit(t, userToken, fmt.Sprintf(
		`{"targetType":"Tag","targetId":%d,"reason":"nicer name","payload":{"name":"better"}}`, tagID))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit: status %d: %s", resp.StatusCode, readAll(t, resp))
	}
	taskID := jsonIntOrZero(t, readAll(t, resp), "id")

	// The submitter can read their own task.
	ownerResp := h.request(http.MethodGet, fmt.Sprintf("/review/tasks/%d", taskID), userToken, nil, "")
	defer ownerResp.Body.Close()

	if ownerResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", ownerResp.StatusCode, readAll(t, ownerResp))
	}
	payload := readAll(t, ownerResp)

	for _, field := range []string{"id", "kind", "targetType", "targetId", "status", "reason", "target"} {
		if !jsonHas(payload, field) {
			t.Errorf("review task dto is missing %q", field)
		}
	}

	// The proposed payload must be echoed back: that is what the moderator sees
	// in the queue before deciding.
	if !containsBytes(payload, "better") {
		t.Error("the stored payload is not echoed in the response")
	}

	// Another ordinary user must not read it.
	otherResp := h.request(http.MethodGet, fmt.Sprintf("/review/tasks/%d", taskID),
		h.issueToken(4, config.RoleUser), nil, "")
	defer otherResp.Body.Close()

	if otherResp.StatusCode == http.StatusOK {
		t.Error("an unrelated user could read someone else's review task")
	}
}

func TestGetReviewTaskNotFound(t *testing.T) {
	h := newHarness(t)

	resp := h.request(http.MethodGet, "/review/tasks/999999",
		h.issueToken(2, config.RoleModerator), nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for an unknown task", resp.StatusCode)
	}
}

func TestGetReviewTaskRequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	resp := h.request(http.MethodGet, "/review/tasks/1", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 without a token", resp.StatusCode)
	}
}
