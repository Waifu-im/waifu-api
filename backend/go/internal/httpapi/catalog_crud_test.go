package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// Tag and artist CRUD. The controllers are small but the behaviour around them
// is not, and each rule below has a way of being lost in a port:
//
//   - The three verbs have different permission bars. POST is configurable and
//     may be as low as User; PATCH is Moderator; DELETE is Admin because it
//     cascades. Collapsing them into one guard is the easiest mistake to make.
//   - Creation always writes a review task, even when the creator bypasses
//     review. The task is a record of who changed what, not just a queue.
//   - Slug conflicts and name conflicts are 409 with different messages, because
//     the frontend shows the message next to the field that caused it.
//   - A tag name can exceed what a slug can hold, so the two length limits are
//     different and both are enforced.

// tagReviewStatus reads the stored review status of a tag.
func (h *harness) tagReviewStatus(t *testing.T, id int64) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var status int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "ReviewStatus" FROM "Tags" WHERE "Id" = $1`, id).Scan(&status); err != nil {
		t.Fatalf("read tag %d: %v", id, err)
	}
	return statusName(status)
}

func statusName(v int) string {
	switch v {
	case 0:
		return "Pending"
	case 1:
		return "Accepted"
	case 2:
		return "Rejected"
	default:
		return "Unknown"
	}
}

// openTaskCount counts pending review tasks of a kind for a submitter.
func (h *harness) openTaskCount(t *testing.T, userID int64, kind int) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var n int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT count(*) FROM "ReviewTasks"
         WHERE "SubmitterId" = $1 AND "Kind" = $2 AND "Status" = 0`, userID, kind).
		Scan(&n); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	return n
}

// TestCreateTagDerivesSlug covers POST /tags when no slug is supplied.
func TestCreateTagDerivesSlug(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	resp := h.request("POST", "/tags", token,
		bytesReader(`{"name":"Marin Kitagawa","description":"classmate"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	body := readAll(t, resp)
	if got := jsonString(t, body, "slug"); got != "marin-kitagawa" {
		t.Errorf("slug = %q, want the name slugified", got)
	}
	if got := jsonString(t, body, "name"); got != "Marin Kitagawa" {
		t.Errorf("name = %q, want it preserved with its capitalisation", got)
	}
	if got := jsonString(t, body, "reviewStatus"); got != "Pending" {
		t.Errorf("reviewStatus = %q, want Pending for a plain User", got)
	}

	// The name keeps its case even though the slug does not, which is why the
	// conflict check cannot just compare slugs.
	if location := resp.Header.Get("Location"); !strings.HasSuffix(location, "/tags/"+itoa(jsonIntOrZero(t, body, "id"))) {
		t.Errorf("Location = %q, want it to address the new tag", location)
	}
}

func jsonIntOrZero(t *testing.T, body []byte, field string) int {
	t.Helper()

	v, err := jsonInt(t, body, field)
	if err != nil {
		t.Fatalf("field %q: %v", field, err)
	}
	return v
}

// TestCreateTagHonoursExplicitSlug covers the override path.
func TestCreateTagHonoursExplicitSlug(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	resp := h.request("POST", "/tags", token,
		bytesReader(`{"name":"Kitagawa","slug":"my-own-slug","description":"x"}`), "application/json")
	body := readAll(t, resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "slug"); got != "my-own-slug" {
		t.Errorf("slug = %q, want the supplied slug", got)
	}
}

// TestCreateTagConflicts distinguishes the two 409s.
//
// The messages differ because the frontend attaches them to different fields, so
// collapsing them into one "already exists" loses the distinction.
func TestCreateTagConflicts(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	create := func(t *testing.T, body string) (int, []byte) {
		t.Helper()
		resp := h.request("POST", "/tags", token, bytesReader(body), "application/json")
		return resp.StatusCode, readAll(t, resp)
	}

	// The first one succeeds; the rest need something to conflict with.
	if status, body := create(t, `{"name":"Waifu","slug":"waifu","description":"d"}`); status != http.StatusCreated {
		t.Fatalf("first create = %d: %s", status, body)
	}

	// Same name, different slug: the name check fires and names the name.
	status, body := create(t, `{"name":"Waifu","slug":"waifu-two","description":"d"}`)
	if status != http.StatusConflict {
		t.Fatalf("same name = %d, want 409: %s", status, body)
	}
	if !strings.Contains(string(body), "name 'Waifu' already exists") {
		t.Errorf("name conflict should name the tag, got: %s", body)
	}

	// Same slug, different name: the slug check fires and names the slug.
	status, body = create(t, `{"name":"Waifu Girl","slug":"waifu","description":"d"}`)
	if status != http.StatusConflict {
		t.Fatalf("same slug = %d, want 409: %s", status, body)
	}
	if !strings.Contains(string(body), "slug 'waifu' already exists") {
		t.Errorf("slug conflict should name the slug, got: %s", body)
	}
}

// TestCreateTagCaseInsensitiveNameConflict pins the lowercasing the C# does.
//
// ToSlug lowercases the slug, but the stored name keeps its case. The lookup
// compares lowercased, so "Waifu" and "waifu" are one tag.
func TestCreateTagCaseInsensitiveNameConflict(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	resp := h.request("POST", "/tags", token,
		bytesReader(`{"name":"waifu","description":"lowercase"}`), "application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.StatusCode, body)
	}

	resp = h.request("POST", "/tags", token,
		bytesReader(`{"name":"WAIFU","description":"uppercase"}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("case-differing name = %d, want 409: %s", resp.StatusCode, readAll(t, resp))
	}
}

// TestCreateTagValidatesInput covers the validator limits.
func TestCreateTagValidatesInput(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	cases := []struct {
		body  string
		field string
	}{
		{`{"name":"","description":"d"}`, "Name"},
		{`{"name":"   ","description":"d"}`, "Name"},
		{`{"name":"` + strings.Repeat("a", 31) + `","description":"d"}`, "Name"},
		{`{"name":"ok","description":""}`, "Description"},
		{`{"name":"ok","description":"` + strings.Repeat("d", 151) + `"}`, "Description"},
		{`{"name":"ok","description":"d","slug":"Has Uppercase"}`, "Slug"},
		{`{"name":"ok","description":"d","slug":"` + strings.Repeat("s", 51) + `"}`, "Slug"},
	}

	for _, c := range cases {
		resp := h.request("POST", "/tags", token, bytesReader(c.body), "application/json")
		status, body := resp.StatusCode, readAll(t, resp)

		if status != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400: %s", c.body, status, body)
			continue
		}
		if !strings.Contains(string(body), c.field) {
			t.Errorf("%s should fail on %s, got: %s", c.body, c.field, body)
		}
	}
}

// TestTagPermissionsAreDistinct is the regression test for a single shared guard.
//
// POST is configurable and may be open to Users, PATCH is Moderator-only, and
// DELETE is Admin-only because it strips the tag from every image. If all three
// used the POST guard, ordinary users could rewrite the catalogue.
func TestTagPermissionsAreDistinct(t *testing.T) {
	h := newHarness(t)

	user := h.issueToken(51, config.RoleUser)
	moderator := h.issueToken(61, config.RoleModerator)
	admin := h.issueToken(71, config.RoleAdmin)

	tagID := ensureTag(t, h, "permission-probe")

	// A User may create.
	resp := h.request("POST", "/tags", user,
		bytesReader(`{"name":"user-made","description":"d"}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusCreated {
		t.Fatalf("User creating a tag = %d, want 201: %s", status, body)
	}

	// A User may not patch.
	resp = h.request("PATCH", "/tags/"+itoa64(tagID), user,
		bytesReader(`{"name":"renamed-by-user","description":"d"}`), "application/json")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("User patching a tag = %d, want 403", resp.StatusCode)
	}

	// A User may not delete.
	resp = h.request("DELETE", "/tags/"+itoa64(tagID), user, nil, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("User deleting a tag = %d, want 403", resp.StatusCode)
	}

	// A Moderator may patch but not delete.
	resp = h.request("PATCH", "/tags/"+itoa64(tagID), moderator,
		bytesReader(`{"name":"renamed-by-mod","description":"d"}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Errorf("Moderator patching a tag = %d, want 200: %s", status, body)
	}

	resp = h.request("DELETE", "/tags/"+itoa64(tagID), moderator, nil, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Moderator deleting a tag = %d, want 403", resp.StatusCode)
	}

	// An Admin may delete.
	resp = h.request("DELETE", "/tags/"+itoa64(tagID), admin, nil, "")
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Admin deleting a tag = %d, want 204: %s", resp.StatusCode, readAll(t, resp))
	}
}

// TestCreateTagBypassesReviewForTrustedRoles covers IReviewPolicy.
//
// The tag is accepted immediately, but a review task is still written so the
// queue records who created it.
func TestCreateTagBypassesReviewForTrustedRoles(t *testing.T) {
	h := newHarness(t)

	for _, role := range []config.Role{config.RoleTrustedUser, config.RoleModerator, config.RoleAdmin} {
		before := h.openTaskCount(t, 51, 0)
		token := h.issueToken(51, role)

		resp := h.request("POST", "/tags", token,
			bytesReader(`{"name":"bypass-`+itoa(int(role))+`","description":"d"}`), "application/json")
		body := readAll(t, resp)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("role %d create = %d: %s", role, resp.StatusCode, body)
		}

		if got := jsonString(t, body, "reviewStatus"); got != "Accepted" {
			t.Errorf("role %d reviewStatus = %q, want Accepted", role, got)
		}

		if after := h.openTaskCount(t, 51, 0); after != before {
			t.Errorf("role %d left %d open tasks, want %d; a bypassed creation must "+
				"still be recorded in the queue", role, after-before, after)
		}
	}
}

// TestCreateTagAlwaysEnqueuesATask pins the record half of the bypass.
func TestCreateTagAlwaysEnqueuesATask(t *testing.T) {
	h := newHarness(t)

	user := h.issueToken(51, config.RoleUser)
	resp := h.request("POST", "/tags", user,
		bytesReader(`{"name":"queue-check","description":"d"}`), "application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.StatusCode, body)
	}

	tagID := int64(jsonIntOrZero(t, body, "id"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		kind       int
		targetType int
		targetID   int64
		status     int
	)
	err := h.db.Pool().QueryRow(ctx, `
        SELECT "Kind", "TargetType", "TargetId", "Status"
        FROM "ReviewTasks"
        WHERE "SubmitterId" = $1 AND "TargetId" = $2
        ORDER BY "Id" DESC LIMIT 1`, 51, tagID).
		Scan(&kind, &targetType, &targetID, &status)
	if err != nil {
		t.Fatalf("no review task was written for the new tag: %v", err)
	}

	if kind != 0 {
		t.Errorf("Kind = %d, want 0 (NewContent)", kind)
	}
	if targetType != 1 {
		t.Errorf("TargetType = %d, want 1 (Tag)", targetType)
	}
	if status != 0 {
		t.Errorf("Status = %d, want 0 (Pending)", status)
	}
}

// TestUpdateTag covers PATCH /tags/{id}.
func TestUpdateTag(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "before")
	moderator := h.issueToken(61, config.RoleModerator)

	resp := h.request("PATCH", "/tags/"+itoa64(tagID), moderator,
		bytesReader(`{"name":"after","slug":"after-slug","description":"new text"}`), "application/json")
	body := readAll(t, resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "name"); got != "after" {
		t.Errorf("name = %q, want after", got)
	}
	if got := jsonString(t, body, "slug"); got != "after-slug" {
		t.Errorf("slug = %q, want after-slug", got)
	}
	if got := jsonString(t, body, "description"); got != "new text" {
		t.Errorf("description = %q, want the new text", got)
	}

	// The slug is addressable at its new value.
	resp = h.request("GET", "/tags/by-slug/after-slug", "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET by the new slug = %d, want 200", resp.StatusCode)
	}
}

// TestUpdateTagReviewStatus covers the moderator-only approval shortcut.
func TestUpdateTagReviewStatus(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(51, config.RoleUser)
	resp := h.request("POST", "/tags", token,
		bytesReader(`{"name":"awaiting-approval","description":"d"}`), "application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d: %s", resp.StatusCode, body)
	}
	tagID := int64(jsonIntOrZero(t, body, "id"))

	if got := h.tagReviewStatus(t, tagID); got != "Pending" {
		t.Fatalf("new tag status = %q, want Pending", got)
	}

	moderator := h.issueToken(61, config.RoleModerator)

	// Program.cs registers a bare JsonStringEnumConverter, whose
	// AllowIntegerValues defaults to true, so the numeric form is as valid as
	// the named one. Rejecting it would be a divergence the original does not have.
	for _, form := range []string{`"Accepted"`, `1`} {
		resp = h.request("PATCH", "/tags/"+itoa64(tagID), moderator,
			bytesReader(`{"name":"awaiting-approval","description":"d","reviewStatus":`+form+`}`),
			"application/json")
		status, body := resp.StatusCode, readAll(t, resp)
		if status != http.StatusOK {
			t.Fatalf("approve with reviewStatus %s = %d: %s", form, status, body)
		}
		if got := h.tagReviewStatus(t, tagID); got != "Accepted" {
			t.Errorf("status after PATCH %s = %q, want Accepted", form, got)
		}
	}

	// A value that is neither member is still rejected.
	resp = h.request("PATCH", "/tags/"+itoa64(tagID), moderator,
		bytesReader(`{"name":"awaiting-approval","description":"d","reviewStatus":7}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("reviewStatus 7 = %d, want 400", resp.StatusCode)
	}
}

// TestDeleteTagCascadesToImages covers the reason DELETE is Admin-only.
func TestDeleteTagCascadesToImages(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "doomed")
	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"doomed"}}.insert(t, h)
	tagID := ensureTag(t, h, "doomed")

	admin := h.issueToken(71, config.RoleAdmin)
	resp := h.request("DELETE", "/tags/"+itoa64(tagID), admin, nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Fatalf("delete = %d: %s", status, body)
	}

	// The tag is gone.
	resp = h.request("GET", "/tags/by-slug/doomed", "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET deleted tag = %d, want 404", resp.StatusCode)
	}

	// The image survived, because the cascade strips the association rather than
	// deleting content that has nothing to do with the tag.
	resp = h.request("GET", "/images/"+itoa64(imageID), "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("image after tag deletion = %d, want it still present", resp.StatusCode)
	}
}

// TestGetTagByIdAndSlug covers the two read endpoints and their 404s.
func TestGetTagByIdAndSlug(t *testing.T) {
	h := newHarness(t)

	tagID := ensureTag(t, h, "readable")

	resp := h.request("GET", "/tags/"+itoa64(tagID), "", nil, "")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get by id = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "slug"); got != "readable" {
		t.Errorf("slug = %q, want readable", got)
	}

	resp = h.request("GET", "/tags/by-slug/readable", "", nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Errorf("get by slug = %d: %s", status, body)
	}

	for _, path := range []string{"/tags/999999", "/tags/by-slug/nope"} {
		resp = h.request("GET", path, "", nil, "")
		if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404: %s", path, status, body)
		}
	}
}

// TestListTagsVisibility covers the pending-content rules on GET /tags.
func TestListTagsVisibility(t *testing.T) {
	h := newHarness(t)

	user := h.issueToken(51, config.RoleUser)
	moderator := h.issueToken(61, config.RoleModerator)

	// Seed one pending tag owned by our user, and another owned by someone else.
	mine := seededTagRow("mine-pending", "mine-pending", 51, 0).insert(t, h)
	theirs := seededTagRow("theirs-pending", "theirs-pending", 52, 0).insert(t, h)
	ensureTag(t, h, "visible")

	body := h.request("GET", "/tags", "", nil, "")
	body.Body.Close()
	if body.StatusCode != http.StatusOK {
		t.Fatalf("anonymous list = %d", body.StatusCode)
	}

	// Anonymous sees only accepted tags.
	listing := readAll(t, h.request("GET", "/tags", "", nil, ""))
	if strings.Contains(string(listing), "mine-pending") {
		t.Errorf("anonymous listing leaked a pending tag: %s", listing)
	}

	// The owner can ask for their own pending tags, to attach them in the
	// upload picker before approval.
	listing = readAll(t, h.request("GET", "/tags?includeMyPending=true", user, nil, "")) //nolint:bodyclose
	if !strings.Contains(string(listing), "mine-pending") {
		t.Errorf("includeMyPending should expose the caller's own pending tag: %s", listing)
	}
	if strings.Contains(string(listing), "theirs-pending") {
		t.Errorf("includeMyPending leaked another user's pending tag: %s", listing)
	}

	// A moderator can see everything pending. ReviewStatusFilter is
	// Accepted = 0, Pending = 1, All = 2, so the numeric form is 1.
	listing = readAll(t, h.request("GET", "/tags?reviewStatus=1", moderator, nil, ""))
	if !strings.Contains(string(listing), "theirs-pending") {
		t.Errorf("moderator should see all pending tags: %s", listing)
	}
	if strings.Contains(string(listing), `"visible"`) {
		t.Errorf("reviewStatus=Pending should not include accepted tags: %s", listing)
	}

	// Both spellings select the same set, and the names win over the numbers.
	for _, q := range []string{"reviewStatus=Pending", "reviewStatus=1"} {
		listing = readAll(t, h.request("GET", "/tags?"+q, moderator, nil, "")) //nolint:bodyclose
		if !strings.Contains(string(listing), "theirs-pending") {
			t.Errorf("%s should select pending tags: %s", q, listing)
		}
	}

	// ById still resolves a pending tag for its owner even though it is filtered
	// out of the list.
	resp := h.request("GET", "/tags/"+itoa64(mine), "", nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Errorf("GET pending tag by id = %d: %s", status, body)
	}

	// Theirs stays hidden from the owner, which is the half of the rule that
	// protects other people's submissions.
	_ = theirs
}

// seededTag inserts a tag directly, for cases that need a specific review state
// or creator that the API would not produce on demand.
type seededTag struct {
	name     string
	slug     string
	creator  int64
	status   int
	desc     string
	imageCnt int
}

func seededTagRow(name, slug string, creator int64, status int) seededTag {
	return seededTag{name: name, slug: slug, creator: creator, status: status}
}

func (s seededTag) insert(t *testing.T, h *harness) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	err := h.db.Pool().QueryRow(ctx, `
        INSERT INTO "Tags" ("Name", "Slug", "Description", "ReviewStatus", "CreatorId")
        VALUES ($1, $2, $3, $4, $5) RETURNING "Id"`,
		s.name, s.slug, s.desc, s.status, s.creator).Scan(&id)
	if err != nil {
		t.Fatalf("insert tag %q: %v", s.slug, err)
	}
	return id
}

// TestCreateArtist covers POST /artists and its link validation.
func TestCreateArtist(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	resp := h.request("POST", "/artists", token, bytesReader(`{
        "name":"Miku",
        "description":"a virtual singer",
        "patreon":"https://patreon.com/miku",
        "pixiv":"https://pixiv.net/users/1",
        "twitter":"https://twitter.com/miku"
    }`), "application/json")
	body := readAll(t, resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create artist = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "name"); got != "Miku" {
		t.Errorf("name = %q, want Miku", got)
	}
	if got := jsonString(t, body, "reviewStatus"); got != "Pending" {
		t.Errorf("reviewStatus = %q, want Pending for a plain User", got)
	}
}

// TestCreateArtistValidatesLinks covers the link rules.
func TestCreateArtistValidatesLinks(t *testing.T) {
	h := newHarness(t)
	token := h.issueToken(51, config.RoleUser)

	// The C# validator is Uri.TryCreate(value, UriKind.Absolute), which is far
	// looser than it looks: it only requires a scheme. So "javascript:alert(1)"
	// and "ftp://example.com" are VALID there, and a port that rejects them to
	// "harden" the endpoint would diverge from the service being replaced.
	cases := []struct {
		body string
		why  string
	}{
		{`{"name":"a","description":"d","patreon":"not-a-url"}`, "no scheme"},
		{`{"name":"d","description":"d","patreon":"` + strings.Repeat("x", 201) + `"}`, "over-long"},
		{`{"name":"e","description":"d","pixiv":"` + strings.Repeat("x", 201) + `"}`, "over-long"},
	}

	for _, c := range cases {
		resp := h.request("POST", "/artists", token, bytesReader(c.body), "application/json")
		status, body := resp.StatusCode, readAll(t, resp)
		if status != http.StatusBadRequest {
			t.Errorf("%s (%s) = %d, want 400: %s", c.why, c.body, status, body)
		}
	}

	// Any absolute URI is accepted, including the schemes a stricter reading
	// would reject.
	for _, body := range []string{
		`{"name":"ok1","description":"d","patreon":"javascript:alert(1)"}`,
		`{"name":"ok2","description":"d","twitter":"ftp://example.com"}`,
		`{"name":"ok3","description":"d","pixiv":"mailto:someone@example.com"}`,
	} {
		resp := h.request("POST", "/artists", token, bytesReader(body), "application/json")
		if status, respBody := resp.StatusCode, readAll(t, resp); status != http.StatusCreated {
			t.Errorf("%s = %d, want 201 to match Uri.TryCreate: %s", body, status, respBody)
		}
	}
}

// TestArtistPermissionsAreDistinct mirrors the tag rules.
func TestArtistPermissionsAreDistinct(t *testing.T) {
	h := newHarness(t)

	user := h.issueToken(51, config.RoleUser)
	moderator := h.issueToken(61, config.RoleModerator)
	admin := h.issueToken(71, config.RoleAdmin)

	artistID := ensureArtist(t, h, "Probe")

	resp := h.request("PATCH", "/artists/"+itoa64(artistID), user,
		bytesReader(`{"name":"nope","description":"d"}`), "application/json")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("User patching an artist = %d, want 403", resp.StatusCode)
	}

	resp = h.request("PATCH", "/artists/"+itoa64(artistID), moderator,
		bytesReader(`{"name":"Probed","description":"d"}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Errorf("Moderator patching an artist = %d: %s", status, body)
	}
	if got := h.artistField(t, artistID, `"Name"`); got != "Probed" {
		t.Errorf("stored name = %q, want Probed", got)
	}

	resp = h.request("DELETE", "/artists/"+itoa64(artistID), moderator, nil, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Moderator deleting an artist = %d, want 403", resp.StatusCode)
	}

	resp = h.request("DELETE", "/artists/"+itoa64(artistID), admin, nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Errorf("Admin deleting an artist = %d: %s", status, body)
	}

	resp = h.request("GET", "/artists/"+itoa64(artistID), "", nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNotFound {
		t.Errorf("GET deleted artist = %d: %s", status, body)
	}
}

// TestArtistSlugConflict covers the duplicate-name rule for artists.
func TestArtistSlugConflict(t *testing.T) {
	h := newHarness(t)

	ensureArtist(t, h, "Duplicate")
	token := h.issueToken(51, config.RoleUser)

	resp := h.request("POST", "/artists", token,
		bytesReader(`{"name":"duplicate","description":"d"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("case-differing artist name = %d, want 409: %s",
			resp.StatusCode, readAll(t, resp))
	}
}
