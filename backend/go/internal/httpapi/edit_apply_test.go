package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"waifuapi/internal/config"
)

// Coverage for the edit pipeline's apply step, which writes an approved edit's
// payload onto its target.
//
// This is the code that mutates live content on a moderator's approval, and it
// sat at 27% with applyNewContent at zero. The rule it has to honour is the one
// its own comment states: a field the payload does not mention is left alone,
// because the underlying update treats an absent value as "clear it".

// submitEdit posts an edit request and returns the new task id.
func submitEdit(t *testing.T, h *harness, token string, body string) int {
	t.Helper()

	resp := h.submitEdit(t, token, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit edit: status %d: %s", resp.StatusCode, readAll(t, resp))
	}
	return jsonIntOrZero(t, readAll(t, resp), "id")
}

// approveEdit approves a review task as a moderator.
func approveEdit(t *testing.T, h *harness, taskID int) {
	t.Helper()

	token := h.issueToken(2, config.RoleModerator)
	resp := h.request(http.MethodPatch, fmt.Sprintf("/review/tasks/%d/approve", taskID), token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("approve edit %d: status %d: %s", taskID, resp.StatusCode, readAll(t, resp))
	}
}

// TestApprovedTagEditReplacesOnlyTheNamedFields is the core "leave the rest
// alone" rule. Both fields are set, so a payload that cleared the unmentioned
// one would be visible.
func TestApprovedTagEditReplacesOnlyTheNamedFields(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	tagID := ensureTag(t, h, "original-name")
	setTagDescription(t, h, tagID, "original description")

	// Change only the name; description is not mentioned.
	taskID := submitEdit(t, h, userToken, fmt.Sprintf(
		`{"targetType":"Tag","targetId":%d,"payload":{"name":"renamed"}}`, tagID))
	approveEdit(t, h, taskID)

	resp := h.request(http.MethodGet, fmt.Sprintf("/tags/%d", tagID), "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)

	if got := jsonString(t, payload, "name"); got != "renamed" {
		t.Errorf("name = %q, want renamed", got)
	}
	if got := jsonString(t, payload, "description"); got != "original description" {
		t.Errorf("description = %q, want it unchanged at %q", got, "original description")
	}
}

// TestApprovedTagEditTreatsEmptyStringAsNoChange pins a subtlety that is easy
// to get wrong in either direction.
//
// EditPreparer.cs computes hasDescription as !string.IsNullOrEmpty, so an empty
// string means "no change" rather than "clear it". The Go validator matches
// that exactly. An edit whose only content is an empty description is therefore
// rejected as identical rather than clearing the field, and the frontend has to
// use null for "leave alone" the same way.
func TestApprovedTagEditTreatsEmptyStringAsNoChange(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	tagID := ensureTag(t, h, "clearable")
	setTagDescription(t, h, tagID, "will be removed")

	body := fmt.Sprintf(
		`{"targetType":"Tag","targetId":%d,"payload":{"description":""}}`, tagID)

	resp := h.submitEdit(t, userToken, body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an edit that changes nothing: %s",
			resp.StatusCode, readAll(t, resp))
	}

	// And the field is genuinely untouched.
	getResp := h.request(http.MethodGet, fmt.Sprintf("/tags/%d", tagID), "", nil, "")
	defer getResp.Body.Close()
	if got := jsonString(t, readAll(t, getResp), "description"); got != "will be removed" {
		t.Errorf("description = %q, want it unchanged because an empty string is not a change", got)
	}
}

// TestUnreadableStoredPayloadIsRejected covers the defensive branch: a payload
// that cannot be decoded must not half-apply.
func TestUnreadableStoredPayloadIsRejected(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	tagID := ensureTag(t, h, "safe")

	// Corrupt the stored payload directly, which is the only way to reach the
	// branch: the submit endpoint validates before it writes.
	stored := fmt.Sprintf(
		`{"targetType":"Tag","targetId":%d,"payload":{"name":"renamed"}}`, tagID)
	taskID := submitEdit(t, h, userToken, stored)
	corruptReviewPayload(t, h, taskID)

	moderatorToken := h.issueToken(2, config.RoleModerator)
	resp := h.request(http.MethodPatch, fmt.Sprintf("/review/tasks/%d/approve", taskID), moderatorToken, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		t.Fatalf("status = %d, want a rejection for an unreadable payload", resp.StatusCode)
	}

	// The target must be unchanged.
	getResp := h.request(http.MethodGet, fmt.Sprintf("/tags/%d", tagID), "", nil, "")
	defer getResp.Body.Close()
	if got := jsonString(t, readAll(t, getResp), "name"); got != "safe" {
		t.Errorf("name = %q, want it untouched after a failed apply", got)
	}
}

// corruptReviewPayload replaces a task's stored payload with unparseable JSON.
//
// The submit endpoint refuses a malformed payload, so the only way to exercise
// the defensive branch in applyEdit is to corrupt the row directly. That models
// a truncated write or a migration from an older shape, which is exactly when
// the branch would run in production.
func corruptReviewPayload(t *testing.T, h *harness, taskID int) {
	t.Helper()

	// The column is jsonb, so Postgres rejects malformed JSON itself. A type
	// mismatch is valid JSON that the target struct cannot decode, which is what
	// the branch actually guards against: a payload written by an older shape.
	_, err := h.db.Pool().Exec(context.Background(),
		`UPDATE "ReviewTasks" SET "Payload" = $1 WHERE "Id" = $2`,
		[]byte(`{"name": 12345}`), taskID)
	if err != nil {
		t.Fatalf("corrupt payload: %v", err)
	}
}

// setTagDescription writes a tag description directly, so the edit under test
// has a non-empty value that must survive.
func setTagDescription(t *testing.T, h *harness, tagID int64, description string) {
	t.Helper()
	if _, err := h.db.Pool().Exec(context.Background(),
		`UPDATE "Tags" SET "Description" = $1 WHERE "Id" = $2`, description, tagID); err != nil {
		t.Fatalf("set description: %v", err)
	}
}
