package httpapi_test

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"testing"

	"waifuapi/internal/config"
)

// Coverage for approving new content, which is the other half of the review
// pipeline.
//
// applyNewContent was at zero: nothing exercised the path where a moderator
// approves a pending upload and the content is actually published. The submit
// side is covered elsewhere, so this is specifically the apply side.

func TestApproveNewImagePublishesIt(t *testing.T) {
	h := newHarness(t)

	// A plain User's upload is queued rather than published.
	userToken := h.issueToken(1, config.RoleUser)
	body, contentType := multipartImage(t, "file", "queued.png",
		testImage(t, 120, 160, color.RGBA{70, 120, 200, 255}), map[string]string{"isNsfw": "false"})

	uploadResp := h.request(http.MethodPost, "/images/upload", userToken, body, contentType)
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d: %s", uploadResp.StatusCode, readAll(t, uploadResp))
	}
	uploadPayload := readAll(t, uploadResp)
	uploadResp.Body.Close()

	if got := jsonString(t, uploadPayload, "reviewStatus"); got != "Pending" {
		t.Fatalf("reviewStatus = %q, want Pending for a plain User's upload", got)
	}

	imageID := jsonIntOrZero(t, uploadPayload, "id")

	// A pending image is not publicly visible yet.
	publicResp := h.request(http.MethodGet, fmt.Sprintf("/images/%d", imageID), "", nil, "")
	publicResp.Body.Close()
	if publicResp.StatusCode == http.StatusOK {
		t.Error("a pending image is visible before approval")
	}

	// Approve it.
	moderatorToken := h.issueToken(2, config.RoleModerator)
	tasksResp := h.request(http.MethodGet, "/review/tasks?kind=NewContent", moderatorToken, nil, "")
	if tasksResp.StatusCode != http.StatusOK {
		t.Fatalf("list tasks: status %d: %s", tasksResp.StatusCode, readAll(t, tasksResp))
	}
	taskID := findNewContentTaskFor(t, readAll(t, tasksResp), imageID)
	if taskID == 0 {
		t.Fatal("no pending NewContent task for the uploaded image")
	}

	approveResp := h.request(http.MethodPatch,
		fmt.Sprintf("/review/tasks/%d/approve", taskID), moderatorToken, nil, "")
	defer approveResp.Body.Close()

	if approveResp.StatusCode < 200 || approveResp.StatusCode >= 300 {
		t.Fatalf("approve: status %d: %s", approveResp.StatusCode, readAll(t, approveResp))
	}

	// Now it is public, and the stored object must still match the row.
	afterResp := h.request(http.MethodGet, fmt.Sprintf("/images/%d", imageID), "", nil, "")
	defer afterResp.Body.Close()
	if afterResp.StatusCode != http.StatusOK {
		t.Fatalf("approved image is still not visible: status %d", afterResp.StatusCode)
	}
	if got := jsonString(t, readAll(t, afterResp), "reviewStatus"); got != "Accepted" {
		t.Errorf("reviewStatus = %q, want Accepted after approval", got)
	}
}

func TestApproveNewTagPublishesIt(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	resp := h.request(http.MethodPost, "/tags", userToken,
		bytesReader(`{"name":"queued-tag","description":"waiting"}`), "application/json")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create tag: status %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)
	resp.Body.Close()

	tagID := jsonIntOrZero(t, payload, "id")
	if got := jsonString(t, payload, "reviewStatus"); got != "Pending" {
		t.Fatalf("reviewStatus = %q, want Pending for a plain User's tag", got)
	}

	moderatorToken := h.issueToken(2, config.RoleModerator)
	tasksResp := h.request(http.MethodGet, "/review/tasks?kind=NewContent", moderatorToken, nil, "")
	tasks := readAll(t, tasksResp)
	tasksResp.Body.Close()

	taskID := findNewContentTaskFor(t, tasks, tagID)
	if taskID == 0 {
		t.Fatal("no pending NewContent task for the created tag")
	}

	approveResp := h.request(http.MethodPatch,
		fmt.Sprintf("/review/tasks/%d/approve", taskID), moderatorToken, nil, "")
	defer approveResp.Body.Close()

	if approveResp.StatusCode < 200 || approveResp.StatusCode >= 300 {
		t.Fatalf("approve: status %d: %s", approveResp.StatusCode, readAll(t, approveResp))
	}

	getResp := h.request(http.MethodGet, fmt.Sprintf("/tags/%d", tagID), "", nil, "")
	defer getResp.Body.Close()
	body := readAll(t, getResp)
	if got := jsonString(t, body, "reviewStatus"); got != "Accepted" {
		t.Errorf("reviewStatus = %q, want Accepted after approval", got)
	}
}

// TestOrdinaryUserCannotApprove is the authorisation half: publishing is a
// moderator action and an ordinary contributor must not be able to self-approve.
func TestOrdinaryUserCannotApprove(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	body, contentType := multipartImage(t, "file", "selfapprove.png",
		testImage(t, 100, 100, color.RGBA{10, 200, 90, 255}), nil)

	resp := h.request(http.MethodPost, "/images/upload", userToken, body, contentType)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d: %s", resp.StatusCode, readAll(t, resp))
	}
	imageID := jsonIntOrZero(t, readAll(t, resp), "id")
	resp.Body.Close()

	tasksResp := h.request(http.MethodGet, "/review/tasks", userToken, nil, "")
	taskID := findNewContentTaskFor(t, readAll(t, tasksResp), imageID)
	tasksResp.Body.Close()
	if taskID == 0 {
		t.Fatal("no pending task to attempt approval on")
	}

	approveResp := h.request(http.MethodPatch,
		fmt.Sprintf("/review/tasks/%d/approve", taskID), userToken, nil, "")
	defer approveResp.Body.Close()

	if approveResp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a non-moderator approval", approveResp.StatusCode)
	}
}

// findNewContentTaskFor returns the pending NewContent task whose targetId
// matches, or 0 when there is none.
//
// The envelope is walked by decoding into a generic shape rather than by
// substring matching, because the target id also appears in nested content.
func findNewContentTaskFor(t *testing.T, payload []byte, targetID int) int64 {
	t.Helper()

	var envelope struct {
		Items []struct {
			ID         int64  `json:"id"`
			Kind       string `json:"kind"`
			TargetType string `json:"targetType"`
			TargetID   int64  `json:"targetId"`
			Status     string `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode review task envelope: %v", err)
	}

	for _, item := range envelope.Items {
		if item.TargetID == int64(targetID) && item.Status == "Pending" {
			return item.ID
		}
	}
	return 0
}

// TestModeratorCorrectsPendingContentBeforeApproving covers the other apply
// path.
//
// PATCH /review/tasks/{id} is not an edit request in the review sense; it lets a
// moderator fix the proposed content on a still-pending task and writes it
// through immediately, which is what applyNewContent does. Approving goes
// through applyEdit instead, so neither test alone reaches this function.
func TestModeratorCorrectsPendingContentBeforeApproving(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(1, config.RoleUser)
	body, contentType := multipartImage(t, "file", "correctable.png",
		testImage(t, 110, 150, color.RGBA{30, 140, 90, 255}), nil)

	uploadResp := h.request(http.MethodPost, "/images/upload", userToken, body, contentType)
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: status %d: %s", uploadResp.StatusCode, readAll(t, uploadResp))
	}
	imageID := jsonIntOrZero(t, readAll(t, uploadResp), "id")
	uploadResp.Body.Close()

	moderatorToken := h.issueToken(2, config.RoleModerator)
	tasksResp := h.request(http.MethodGet, "/review/tasks", moderatorToken, nil, "")
	taskID := findNewContentTaskFor(t, readAll(t, tasksResp), imageID)
	tasksResp.Body.Close()
	if taskID == 0 {
		t.Fatal("no pending task for the uploaded image")
	}

	patchBody := `{"content":{"isNsfw":true,"source":"https://example.test/corrected"}}`
	patchResp := h.request(http.MethodPatch,
		fmt.Sprintf("/review/tasks/%d", taskID), moderatorToken, bytesReader(patchBody), "application/json")
	defer patchResp.Body.Close()

	if patchResp.StatusCode < 200 || patchResp.StatusCode >= 300 {
		t.Fatalf("patch task: status %d: %s", patchResp.StatusCode, readAll(t, patchResp))
	}

	// The correction is written through to the target, not just stored on the
	// task. The image is still pending, so it is read back as a moderator: an
	// anonymous caller correctly gets 404 for content that is not published.
	getResp := h.request(http.MethodGet, fmt.Sprintf("/images/%d", imageID), moderatorToken, nil, "")
	defer getResp.Body.Close()
	updated := readAll(t, getResp)

	if !jsonBool(t, updated, "isNsfw") {
		t.Error("isNsfw was not written through to the image")
	}
	if got := jsonString(t, updated, "source"); got != "https://example.test/corrected" {
		t.Errorf("source = %q, want the corrected value", got)
	}

	// And it is still pending: a moderator correction changes the proposal, it
	// does not publish it.
	if got := jsonString(t, updated, "reviewStatus"); got != "Pending" {
		t.Errorf("reviewStatus = %q, want Pending; correcting must not publish", got)
	}
}
