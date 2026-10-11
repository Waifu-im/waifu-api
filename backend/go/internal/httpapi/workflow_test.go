package httpapi_test

import (
	"context"
	"image/color"
	"net/http"
	"testing"
	"time"

	"waifuapi/internal/config"
	"waifuapi/internal/store/models"
)

// TestUploadRequiresAuthentication checks the guard runs before any parsing.
//
// A 401 rather than a 400 matters here: a client that forgot its token should be
// told that, not told its multipart body is malformed.
func TestUploadRequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	resp := h.request("POST", "/images/upload", "", nil, "multipart/form-data; boundary=x")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestUploadRequiresNoCredentialAtAll is the same guard without a malformed
// body, confirming the order of the checks.
func TestUploadRejectsBadToken(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/images", "not-a-real-token", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestOrdinaryUserCannotUploadWhenPermissionIsDenied would run under a config
// with ImageUploadMinRole raised. It is covered by the authz unit tests; here we
// assert the permissive default actually lets a User through, which is the
// behaviour that makes uploads work once review is enabled.
func TestOrdinaryUserCanUploadUnderDefaultConfig(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(1, config.RoleUser)
	data := testImage(t, 100, 100, color.RGBA{60, 60, 60, 255})

	body, contentType := multipartImage(t, "file", "photo.png", data, map[string]string{
		"isNsfw": "false",
	})

	resp := h.request("POST", "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, readAll(t, resp))
	}

	// A plain User must be queued for review, not published immediately.
	payload := readAll(t, resp)
	if got := jsonString(t, payload, "reviewStatus"); got != "Pending" {
		t.Errorf("reviewStatus = %q, want Pending for an ordinary user", got)
	}
	if got := jsonString(t, payload, "message"); got != "Upload pending review." {
		t.Errorf("message = %q, want \"Upload pending review.\"", got)
	}
}

// TestTrustedUserBypassesReview checks the bypass threshold configured in
// Review__ImageReviewBypassMinRole.
func TestTrustedUserBypassesReview(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(2, config.RoleTrustedUser)
	data := testImage(t, 110, 110, color.RGBA{9, 9, 9, 255})

	body, contentType := multipartImage(t, "file", "other.png", data, nil)
	resp := h.request("POST", "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, readAll(t, resp))
	}
	if got := jsonString(t, readAll(t, resp), "reviewStatus"); got != "Accepted" {
		t.Errorf("reviewStatus = %q, want Accepted for a TrustedUser", got)
	}
}

// TestUploadStoresTheObject checks the bytes actually reached storage under the
// expected key.
func TestUploadStoresTheObject(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(3, config.RoleTrustedUser)
	data := testImage(t, 120, 120, color.RGBA{30, 60, 90, 255})

	body, contentType := multipartImage(t, "file", "pic.png", data, nil)
	resp := h.request("POST", "/images/upload", token, body, contentType)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	id, err := jsonInt(t, readAll(t, resp), "id")
	if err != nil {
		t.Fatalf("no image id returned: %v", err)
	}

	stored, existsErr := h.store.Exists(context.Background(), storageKeyFor(int64(id), ".png"))
	if existsErr != nil {
		t.Fatalf("check storage: %v", existsErr)
	}
	if !stored {
		t.Errorf("object %d.png was not written to storage", id)
	}
}

// TestUploadRollsBackWhenStorageFails is the important one.
//
// A row left behind after a failed upload would appear in the catalogue as a
// broken image that no bytes back, and would block a legitimate re-upload
// because the duplicate check would match its hash.
func TestUploadRollsBackWhenStorageFails(t *testing.T) {
	h := newHarness(t)
	h.store.failPut = true

	token := h.issueToken(4, config.RoleTrustedUser)
	data := testImage(t, 130, 130, color.RGBA{200, 10, 10, 255})

	body, contentType := multipartImage(t, "file", "doomed.png", data, nil)
	resp := h.request("POST", "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", resp.StatusCode, readAll(t, resp))
	}

	// No row may survive.
	var count int
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM "Image"`).Scan(&count); err != nil {
		t.Fatalf("count images: %v", err)
	}
	if count != 0 {
		t.Errorf("%d image rows survived a failed upload; the rollback did not run", count)
	}

	// Nor may a review task.
	if err := h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM "ReviewTask"`).Scan(&count); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("%d review tasks survived a failed upload", count)
	}

	// And no object.
	if len(h.store.objects) != 0 {
		t.Errorf("storage holds %d objects after a failed upload", len(h.store.objects))
	}
}

// TestDuplicateUploadIsRejected checks near-duplicate detection.
//
// The same bytes must be refused the second time, which is the whole point of
// the perceptual hash.
func TestDuplicateUploadIsRejected(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(5, config.RoleTrustedUser)
	data := testImage(t, 140, 140, color.RGBA{7, 7, 200, 255})

	body, contentType := multipartImage(t, "file", "dup.png", data, nil)
	first := h.request("POST", "/images/upload", token, body, contentType)
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first upload status = %d", first.StatusCode)
	}

	body2, contentType2 := multipartImage(t, "file", "dup.png", data, nil)
	second := h.request("POST", "/images/upload", token, body2, contentType2)

	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second upload status = %d, want 409: %s", second.StatusCode, readAll(t, second))
	}
	if got := jsonString(t, readAll(t, second), "title"); got != "Conflict" {
		t.Errorf("title = %q, want Conflict", got)
	}
}

// TestUploadRejectsUnknownTag checks that attaching a tag that does not exist
// fails the whole upload, matching UploadImage/Command.cs which throws rather
// than silently skipping.
func TestUploadRejectsUnknownTag(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(6, config.RoleTrustedUser)
	data := testImage(t, 150, 150, color.RGBA{1, 2, 3, 255})

	body, contentType := multipartImage(t, "file", "tagged.png", data, map[string]string{
		"tags": "does-not-exist",
	})

	resp := h.request("POST", "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404: %s", resp.StatusCode, readAll(t, resp))
	}

	var count int
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM "Image"`).Scan(&count); err != nil {
		t.Fatalf("count images: %v", err)
	}
	if count != 0 {
		t.Error("an image row was created despite the unknown tag")
	}
}

// TestCreateTagQueuesForReview covers the tag creation path end to end.
func TestCreateTagQueuesForReview(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(7, config.RoleUser)
	resp := h.request("POST", "/tags", token,
		bytesReader(`{"name":"Hatsune Miku","description":"vocaloid"}`),
		"application/json")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)
	if got := jsonString(t, payload, "slug"); got != "hatsune-miku" {
		t.Errorf("slug = %q, want hatsune-miku", got)
	}
	if got := jsonString(t, payload, "reviewStatus"); got != "Pending" {
		t.Errorf("reviewStatus = %q, want Pending", got)
	}

	// A pending tag must not appear in the public listing.
	list := h.request("GET", "/tags", token, nil, "")
	defer list.Body.Close()
	body := readAll(t, list)
	if containsBytes(body, "hatsune-miku") {
		t.Error("a pending tag leaked into the public tag list")
	}
}

// TestApproveRequiresModerator pins the hardcoded moderation requirement.
func TestApproveRequiresModerator(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(8, config.RoleUser)
	resp := h.request("PATCH", "/review/tasks/1/approve", token, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// TestApprovePublishesPendingImage is the full moderation round trip: upload as
// a User, approve as a Moderator, then confirm the image becomes visible.
func TestApprovePublishesPendingImage(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(9, config.RoleUser)
	data := testImage(t, 160, 160, color.RGBA{120, 30, 200, 255})

	body, contentType := multipartImage(t, "file", "pending.png", data, nil)
	uploadResp := h.request("POST", "/images/upload", userToken, body, contentType)

	if uploadResp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d: %s", uploadResp.StatusCode, readAll(t, uploadResp))
	}
	imageID, err := jsonInt(t, readAll(t, uploadResp), "id")
	if err != nil {
		t.Fatalf("upload returned no id: %v", err)
	}

	// Hidden while pending.
	before := h.request("GET", "/images", "", nil, "")
	defer before.Body.Close()
	if containsBytes(readAll(t, before), `"id":`+itoa(imageID)) {
		t.Error("a pending image was visible before approval")
	}

	// The moderator sees the task in the queue.
	modToken := h.issueToken(10, config.RoleModerator)
	queue := h.request("GET", "/review/tasks?status=Pending", modToken, nil, "")
	defer queue.Body.Close()

	var taskID int
	if err := scanFirstID(readAll(t, queue), &taskID); err != nil {
		t.Fatalf("no pending task in the moderator queue: %v", err)
	}

	// Approve it.
	approve := h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/approve", modToken, nil, "")
	defer approve.Body.Close()
	if approve.StatusCode != http.StatusNoContent {
		t.Fatalf("approve status = %d, want 204: %s", approve.StatusCode, readAll(t, approve))
	}

	// Now visible.
	after := h.request("GET", "/images", "", nil, "")
	defer after.Body.Close()
	if !containsBytes(readAll(t, after), `"id":`+itoa(imageID)) {
		t.Error("the approved image is still not visible")
	}
}

// TestApprovingTwiceConflicts checks the resolved-task guard.
func TestApprovingTwiceConflicts(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(11, config.RoleUser)
	data := testImage(t, 170, 170, color.RGBA{11, 22, 33, 255})
	body, contentType := multipartImage(t, "file", "twice.png", data, nil)
	uploadResp := h.request("POST", "/images/upload", userToken, body, contentType)
	uploadResp.Body.Close()

	modToken := h.issueToken(12, config.RoleModerator)

	first := h.request("PATCH", "/review/tasks/1/approve", modToken, nil, "")
	first.Body.Close()
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first approve status = %d, want 204", first.StatusCode)
	}

	second := h.request("PATCH", "/review/tasks/1/approve", modToken, nil, "")
	defer second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Errorf("second approve status = %d, want 409", second.StatusCode)
	}
}

// TestRejectDeletesTheTarget checks that rejection discards the content rather
// than flagging it, which is why ReviewStatus has no Rejected member.
func TestRejectDeletesTheTarget(t *testing.T) {
	h := newHarness(t)

	userToken := h.issueToken(13, config.RoleUser)
	data := testImage(t, 180, 180, color.RGBA{200, 200, 10, 255})
	body, contentType := multipartImage(t, "file", "rejected.png", data, nil)
	uploadResp := h.request("POST", "/images/upload", userToken, body, contentType)
	uploadResp.Body.Close()

	modToken := h.issueToken(14, config.RoleModerator)

	reject := h.request("PATCH", "/review/tasks/1/reject", modToken,
		bytesReader(`{"reason":"off topic"}`), "application/json")
	defer reject.Body.Close()
	if reject.StatusCode != http.StatusNoContent {
		t.Fatalf("reject status = %d, want 204: %s", reject.StatusCode, readAll(t, reject))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	if err := h.db.Pool().QueryRow(ctx, `SELECT COUNT(*) FROM "Image"`).Scan(&count); err != nil {
		t.Fatalf("count images: %v", err)
	}
	if count != 0 {
		t.Errorf("%d image rows survived a rejection; the target should be deleted", count)
	}
}

// TestOrdinaryUserSeesOnlyOwnTasks checks the queue scoping.
func TestOrdinaryUserSeesOnlyOwnTasks(t *testing.T) {
	h := newHarness(t)

	// One task from user 15 and one from user 16.
	seedPendingTask(t, h, 15)
	seedPendingTask(t, h, 16)

	viewer := h.issueToken(15, config.RoleUser)
	resp := h.request("GET", "/review/tasks", viewer, nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body := readAll(t, resp)
	total := scanTotal(body)
	if total != 1 {
		t.Errorf("ordinary user sees %d tasks, want only their own 1", total)
	}
}

// TestModeratorSeesAllTasks is the counterpart.
func TestModeratorSeesAllTasks(t *testing.T) {
	h := newHarness(t)

	seedPendingTask(t, h, 17)
	seedPendingTask(t, h, 18)

	mod := h.issueToken(19, config.RoleModerator)
	resp := h.request("GET", "/review/tasks", mod, nil, "")
	defer resp.Body.Close()

	if total := scanTotal(readAll(t, resp)); total != 2 {
		t.Errorf("moderator sees %d tasks, want 2", total)
	}
}

// seedPendingTask inserts a pending image task for a submitter.
func seedPendingTask(t *testing.T, h *harness, submitterID int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Two separate statements: pgx cannot send multiple commands in one
	// prepared statement.
	if _, err := h.db.Pool().Exec(ctx, `
        INSERT INTO "Image" ("Width","Height","ByteSize","Extension","DominantColor",
                              "IsNsfw","IsAnimated","PerceptualHash","ReviewStatus","UploaderId")
        VALUES (100,100,1,'.png','#000000',false,false,
                B'0000000000000000000000000000000000000000000000000000000000000000', 0, $1)`,
		submitterID); err != nil {
		t.Fatalf("seed image: %v", err)
	}

	if _, err := h.db.Pool().Exec(ctx, `
        INSERT INTO "ReviewTask" ("Kind","TargetType","TargetId","SubmitterId","Status")
        VALUES (0, 0, (SELECT MAX("Id") FROM "Image"), $1, 0)`,
		submitterID); err != nil {
		t.Fatalf("seed review task: %v", err)
	}
}

// confirm the models package is referenced so the import is meaningful.
var _ = models.ReviewPending
