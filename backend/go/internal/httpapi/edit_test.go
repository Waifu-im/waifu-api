package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
	"waifuapi/internal/store/models"
)

// This file covers the edit pipeline: POST /review/tasks (submit a proposed
// change) and PATCH /review/tasks/{id} (edit a pending task's content).
//
// It is the most intricate logic in the C# service — EditPreparer and EditApplier
// are 335 lines between them. It is the thing most likely to be misunderstood
// rather than mistranslated. Two of its properties are easy to lose in a port
// and expensive when lost:
//
//   - The stored payload is a DELTA. Only the fields that actually differ from
//     the current value are written, so two people proposing different changes to
//     the same image do not overwrite each other.
//   - Unknown members are REJECTED. The payload comes from a browser and is
//     written straight to published content, so accepting an unrecognised field
//     would let a client smuggle reviewStatus or uploaderId through a tag rename.

// submitEdit posts an edit proposal and returns the response.
func (h *harness) submitEdit(t *testing.T, token, body string) *http.Response {
	t.Helper()
	return h.request("POST", "/review/tasks", token, bytesReader(body), "application/json")
}

// taskStatus reads the status of a review task by id.
func (h *harness) taskStatus(t *testing.T, id int64) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var status int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "Status" FROM "ReviewTasks" WHERE "Id" = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read task %d: %v", id, err)
	}
	switch status {
	case 0:
		return "Pending"
	case 1:
		return "Approved"
	case 2:
		return "Rejected"
	case 3:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// tagField reads one column of a tag.
func (h *harness) tagField(t *testing.T, id int64, column string) string {
	t.Helper()

	if !strings.HasPrefix(column, `"`) {
		t.Fatalf("column must be quoted, got %q", column)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var value string
	query := `SELECT ` + column + `::text FROM "Tags" WHERE "Id" = $1`
	if err := h.db.Pool().QueryRow(ctx, query, id).Scan(&value); err != nil {
		t.Fatalf("read %s of tag %d: %v", column, id, err)
	}
	return value
}

// artistField reads one column of an artist.
func (h *harness) artistField(t *testing.T, id int64, column string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var value *string
	query := `SELECT ` + column + ` FROM "Artists" WHERE "Id" = $1`
	if err := h.db.Pool().QueryRow(ctx, query, id).Scan(&value); err != nil {
		t.Fatalf("read %s of artist %d: %v", column, id, err)
	}
	if value == nil {
		return ""
	}
	return *value
}

// TestSubmitTagEditQueuesForReview is the basic happy path.
func TestSubmitTagEditQueuesForReview(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "waifu")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := h.db.Pool().Exec(ctx,
		`UPDATE "Tags" SET "Description" = 'original' WHERE "Id" = $1`, tagID); err != nil {
		t.Fatalf("set description: %v", err)
	}

	token := h.issueToken(31, config.RoleUser)

	// A plain User does not bypass edit review, so this queues.
	resp := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"reason":"better name","payload":{"name":"waifu girl"}}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if got := jsonString(t, body, "kind"); got != "Edit" {
		t.Errorf("kind = %q, want Edit", got)
	}
	if got := jsonString(t, body, "status"); got != "Pending" {
		t.Errorf("status = %q, want Pending", got)
	}
	if got := jsonString(t, body, "reason"); got != "better name" {
		t.Errorf("reason = %q, want the submitted reason", got)
	}

	// The proposal must NOT be applied yet. An edit that lands before approval
	// would let any user rewrite published content.
	if got := h.tagField(t, tagID, `"Name"`); got != "waifu" {
		t.Errorf("tag name = %q before approval, want the original", got)
	}
}

// TestSubmitEditIsADelta is the property most easily lost.
//
// Only the changed field is stored. If the whole payload were stored, a second
// proposal on the same tag would carry the first proposal's value and silently
// revert it on approval.
func TestSubmitEditIsADelta(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := h.db.Pool().Exec(ctx,
		`UPDATE "Tags" SET "Description" = 'unchanged' WHERE "Id" = $1`, tagID); err != nil {
		t.Fatalf("set description: %v", err)
	}

	token := h.issueToken(31, config.RoleUser)
	resp := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"maid outfit"}}`)
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit = %d: %s", resp.StatusCode, body)
	}

	payload := payloadOf(t, body)
	if strings.Contains(payload, "description") {
		t.Errorf("stored payload should be a delta but carries description: %s", payload)
	}
	if !strings.Contains(payload, "maid outfit") {
		t.Errorf("stored payload should carry the new name: %s", payload)
	}
}

// payloadOf re-marshals just the payload object from a task response.
//
// The whole body cannot be searched for a field name, because the embedded
// target carries fields of its own (a tag has a description) that have nothing
// to do with the proposal.
func payloadOf(t *testing.T, body []byte) string {
	t.Helper()

	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode task: %v (%s)", err, body)
	}
	return string(envelope.Payload)
}

// TestSubmitEditRejectsNoOpChange pins the "identical to current values" rule.
func TestSubmitEditRejectsNoOpChange(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	token := h.issueToken(31, config.RoleUser)

	// Proposing the name it already has is not a change.
	resp := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"maid"}}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no-op edit = %d, want 400: %s", resp.StatusCode, readAll(t, resp))
	}

	// An empty payload is likewise refused.
	resp = h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+`,"payload":{}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty payload = %d, want 400", resp.StatusCode)
	}
}

// TestSubmitEditRejectsUnknownMembers is the security boundary.
//
// The payload is written straight to published content. If unrecognised members
// were ignored, a client could smuggle reviewStatus, uploaderId or slug through
// a tag rename and escalate.
func TestSubmitEditRejectsUnknownMembers(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	token := h.issueToken(31, config.RoleUser)

	for _, payload := range []string{
		`{"reviewStatus":"Accepted"}`,
		`{"creatorId":1}`,
		`{"slug":"hijacked"}`,
		`{"name":"fine","reviewStatus":"Pending"}`,
	} {
		resp := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
			`,"payload":`+payload+`}`)
		status := resp.StatusCode
		body := readAll(t, resp)

		if status != http.StatusBadRequest {
			t.Errorf("payload %s = %d, want 400: %s", payload, status, body)
		}
		if strings.Contains(string(body), "not a recognised field") == false {
			t.Logf("payload %s rejected with: %s", payload, body)
		}
	}
}

// TestSubmitEditOneOpenPerTarget prevents queue spam.
func TestSubmitEditOneOpenPerTarget(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	token := h.issueToken(31, config.RoleUser)

	first := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"first"}}`)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first submit = %d: %s", first.StatusCode, readAll(t, first))
	}
	first.Body.Close()

	second := h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"second"}}`)
	defer second.Body.Close()

	if second.StatusCode != http.StatusConflict {
		t.Errorf("second open edit on the same target = %d, want 409: %s",
			second.StatusCode, readAll(t, second))
	}
}

// TestSubmitEditRejectsUnknownTarget distinguishes 404 from 400.
func TestSubmitEditRejectsUnknownTarget(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(31, config.RoleUser)

	resp := h.submitEdit(t, token, `{"targetType":"Tag","targetId":999999,"payload":{"name":"x"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown target = %d, want 404: %s", resp.StatusCode, readAll(t, resp))
	}

	// A pending target is not editable: the C# query filters on Accepted.
	pendingTag := ensureTag(t, h, "pending-tag")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := h.db.Pool().Exec(ctx,
		`UPDATE "Tags" SET "ReviewStatus" = 0 WHERE "Id" = $1`, pendingTag); err != nil {
		t.Fatalf("make tag pending: %v", err)
	}

	resp = h.submitEdit(t, token, `{"targetType":"Tag","targetId":`+itoa64(pendingTag)+
		`,"payload":{"name":"x"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("pending target = %d, want 404: %s", resp.StatusCode, readAll(t, resp))
	}
}

// TestSubmitEditImageTagDelta covers the image variant of the delta rules.
func TestSubmitEditImageTagDelta(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "waifu")
	ensureTag(t, h, "maid")

	imageID := seededImage{Width: 100, Height: 100, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"waifu"}}.insert(t, h)

	user := int64(31)
	token := h.issueToken(user, config.RoleUser)

	// Only one edit may be open per (user, target), so each case cancels the
	// previous one. Without that the second submit is a 409 and the test would
	// be measuring the queue rule instead of the payload rules.
	submit := func(t *testing.T, payload string) *http.Response {
		t.Helper()

		if open := openEditTaskFor(t, h, user, models.TargetImage, imageID); open != 0 {
			resp := h.request("PATCH", "/review/tasks/"+itoa64(open)+"/cancel", token, nil, "")
			status, body := resp.StatusCode, readAll(t, resp)
			if status != http.StatusNoContent {
				t.Fatalf("cancel of task %d = %d: %s", open, status, body)
			}
		}

		return h.submitEdit(t, token, `{"targetType":"Image","targetId":`+itoa64(imageID)+
			`,"payload":`+payload+`}`)
	}

	// Adding a tag the image does not have is a real change.
	resp := submit(t, `{"addTagSlugs":["maid"]}`)
	if body := readAll(t, resp); resp.StatusCode != http.StatusCreated {
		t.Fatalf("adding a new tag = %d: %s", resp.StatusCode, body)
	}

	// Adding one it already has is not a change, so the request is refused.
	resp = submit(t, `{"addTagSlugs":["waifu"]}`)
	if body := readAll(t, resp); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("adding an existing tag = %d, want 400: %s", resp.StatusCode, body)
	}

	// An unknown slug is a 404, not a silently skipped entry.
	resp = submit(t, `{"addTagSlugs":["does-not-exist"]}`)
	if body := readAll(t, resp); resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown tag slug = %d, want 404: %s", resp.StatusCode, body)
	}
}

// openEditTaskFor returns the id of a submitter's open pending edit on a
// target, or 0 when there is none.
//
// Only one edit may be open per (user, target), so a test that submits several
// proposals for the same target has to close each one before the next, or the
// second submit is refused with 409 and the test measures the wrong thing.
func openEditTaskFor(t *testing.T, h *harness, userID int64,
	targetType models.ReviewableType, targetID int64) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	err := h.db.Pool().QueryRow(ctx, `
        SELECT "Id"
        FROM "ReviewTasks"
        WHERE "Kind" = $4 AND "Status" = $5
          AND "SubmitterId" = $1 AND "TargetType" = $2 AND "TargetId" = $3`,
		userID, targetType, targetID, models.KindEdit, models.TaskPending).Scan(&id)
	if err != nil {
		// pgx.ErrNoRows simply means nothing is open.
		return 0
	}
	return id
}

// TestApproveEditAppliesThePayload is the regression test for the silent no-op.
//
// Approving an Edit used to flip ReviewStatus, which is a no-op on already
// accepted content: the queue said Approved while the change never landed.
func TestApproveEditAppliesThePayload(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	submitter := h.issueToken(31, config.RoleUser)
	moderator := h.issueToken(41, config.RoleModerator)

	resp := h.submitEdit(t, submitter, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"maid outfit"}}`)
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submit = %d: %s", resp.StatusCode, body)
	}
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	if got := h.tagField(t, tagID, `"Name"`); got != "maid" {
		t.Fatalf("tag should be unchanged before approval, got %q", got)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/approve", moderator, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("approve = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	if got := h.taskStatus(t, int64(taskID)); got != "Approved" {
		t.Errorf("task status = %q, want Approved", got)
	}
	if got := h.tagField(t, tagID, `"Name"`); got != "maid outfit" {
		t.Errorf("tag name = %q after approval, want the proposed change applied", got)
	}
}

// TestRejectEditLeavesLiveContent is the regression test for the data-loss bug.
//
// Rejecting a NewContent task deletes the submission. Rejecting an Edit must not:
// the target is already published and in use.
func TestRejectEditLeavesLiveContent(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	submitter := h.issueToken(31, config.RoleUser)
	moderator := h.issueToken(41, config.RoleModerator)

	resp := h.submitEdit(t, submitter, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"rejected idea"}}`)
	body := readAll(t, resp)
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/reject", moderator,
		bytesReader(`{"reason":"not a good idea"}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("reject = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	// The tag must still exist, with its original name.
	if got := h.tagField(t, tagID, `"Name"`); got != "maid" {
		t.Errorf("tag name = %q; rejecting an edit must not change the tag", got)
	}
	if got := h.taskStatus(t, int64(taskID)); got != "Rejected" {
		t.Errorf("task status = %q, want Rejected", got)
	}
}

// TestCancelEditLeavesLiveContent mirrors the reject rule for withdrawal.
func TestCancelEditLeavesLiveContent(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	submitter := h.issueToken(31, config.RoleUser)

	resp := h.submitEdit(t, submitter, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"withdrawn"}}`)
	body := readAll(t, resp)
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/cancel", submitter, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	if got := h.tagField(t, tagID, `"Name"`); got != "maid" {
		t.Errorf("tag name = %q; withdrawing an edit must not change the tag", got)
	}
	if got := h.taskStatus(t, int64(taskID)); got != "Cancelled" {
		t.Errorf("task status = %q, want Cancelled", got)
	}
}

// TestEditTaskContentReplacesTheProposal covers PATCH /review/tasks/{id}.
func TestEditTaskContentReplacesTheProposal(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	submitter := h.issueToken(31, config.RoleUser)

	resp := h.submitEdit(t, submitter, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"first idea"}}`)
	body := readAll(t, resp)
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID), submitter,
		bytesReader(`{"content":{"name":"revised idea"}}`), "application/json")
	edited := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("edit task = %d: %s", resp.StatusCode, edited)
	}

	if got := jsonString(t, edited, "status"); got != "Pending" {
		t.Errorf("status = %q; editing content must not change the review status", got)
	}
	if !strings.Contains(string(edited), "revised idea") {
		t.Errorf("edited payload should carry the revised proposal: %s", edited)
	}

	// Approving now applies the revision, not the original.
	moderator := h.issueToken(41, config.RoleModerator)
	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/approve", moderator, nil, "")
	resp.Body.Close()

	if got := h.tagField(t, tagID, `"Name"`); got != "revised idea" {
		t.Errorf("tag name = %q, want the revised proposal applied", got)
	}
}

// TestEditTaskIsOwnerOrModeratorScoped covers the authorisation.
func TestEditTaskIsOwnerOrModeratorScoped(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	owner := h.issueToken(31, config.RoleUser)
	intruder := h.issueToken(32, config.RoleUser)

	resp := h.submitEdit(t, owner, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"mine"}}`)
	body := readAll(t, resp)
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID), intruder,
		bytesReader(`{"content":{"name":"not yours"}}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("intruder editing a task = %d, want 403", resp.StatusCode)
	}

	// A moderator may.
	moderator := h.issueToken(41, config.RoleModerator)
	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID), moderator,
		bytesReader(`{"content":{"name":"moderator revision"}}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("moderator editing a task = %d, want 200: %s",
			resp.StatusCode, readAll(t, resp))
	}

	// A moderator correcting someone else's submission is flagged, so the
	// submitter sees a "modified" cue.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var flagged *time.Time
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "ModeratorEditedAt" FROM "ReviewTasks" WHERE "Id" = $1`, taskID).
		Scan(&flagged); err != nil {
		t.Fatalf("read moderator edit marker: %v", err)
	}
	if flagged == nil {
		t.Error("ModeratorEditedAt should be set when a moderator edits someone else's task")
	}
}

// TestEditResolvedTaskConflicts covers the state guard.
func TestEditResolvedTaskConflicts(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "maid")
	owner := h.issueToken(31, config.RoleUser)
	moderator := h.issueToken(41, config.RoleModerator)

	resp := h.submitEdit(t, owner, `{"targetType":"Tag","targetId":`+itoa64(tagID)+
		`,"payload":{"name":"idea"}}`)
	body := readAll(t, resp)
	taskID, err := jsonInt(t, body, "id")
	if err != nil {
		t.Fatalf("no task id: %v", err)
	}

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID)+"/approve", moderator, nil, "")
	resp.Body.Close()

	resp = h.request("PATCH", "/review/tasks/"+itoa(taskID), owner,
		bytesReader(`{"content":{"name":"too late"}}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("editing a resolved task = %d, want 409", resp.StatusCode)
	}
}

// TestSubmitEditValidatesSource covers the URL rule on an image source.
func TestSubmitEditValidatesSource(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	token := h.issueToken(31, config.RoleUser)

	resp := h.submitEdit(t, token, `{"targetType":"Image","targetId":`+itoa64(imageID)+
		`,"payload":{"source":"not-a-url"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("non-URL source = %d, want 400", resp.StatusCode)
	}

	long := strings.Repeat("a", 501)
	resp = h.submitEdit(t, token, `{"targetType":"Image","targetId":`+itoa64(imageID)+
		`,"payload":{"source":"https://example.com/`+long+`"}}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("over-long source = %d, want 400", resp.StatusCode)
	}
}

// TestArtistEditValidatesLinks covers the link rules on the artist payload.
func TestArtistEditValidatesLinks(t *testing.T) {
	h := newHarness(t)

	artistID := ensureArtist(t, h, "Miku")
	token := h.issueToken(31, config.RoleUser)

	for _, payload := range []string{
		`{"patreon":"not-a-url"}`,
		`{"pixiv":"ftp://example.com"}`,
		`{"twitter":"javascript:alert(1)"}`,
	} {
		resp := h.submitEdit(t, token, `{"targetType":"Artist","targetId":`+itoa64(artistID)+
			`,"payload":`+payload+`}`)
		status := resp.StatusCode
		body := readAll(t, resp)
		if status != http.StatusBadRequest {
			t.Errorf("payload %s = %d, want 400: %s", payload, status, body)
		}
	}

	// A valid link is accepted and queued.
	resp := h.submitEdit(t, token, `{"targetType":"Artist","targetId":`+itoa64(artistID)+
		`,"payload":{"patreon":"https://patreon.com/miku"}}`)
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("valid link = %d: %s", resp.StatusCode, body)
	}
}
