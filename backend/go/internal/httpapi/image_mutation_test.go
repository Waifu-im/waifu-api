package httpapi_test

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// PATCH /images/{id} and DELETE /images/{id}.
//
// These are the two endpoints where the previous port was wrong in a way that
// mattered, so the tests here are mostly about authorisation and about the
// three-state fields rather than about the happy path.
//
// Both take a multipart body, because ImagesController binds them with
// [FromForm]. A JSON decoder "works" right up until a real client sends a form.

// imagePatch builds the multipart body PATCH /images/{id} binds from.
//
// Repeated keys are how ASP.NET collects a List<T> from a form, so tags and
// artists are sent that way rather than comma-joined.
func imagePatch(t *testing.T, fields map[string][]string, file []byte, filename, declaredType string) (string, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for name, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}

	if file != nil {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
		if declaredType != "" {
			header.Set("Content-Type", declaredType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := part.Write(file); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return writer.FormDataContentType(), buf.String()
}

// patchImage posts a patch and returns the status and body.
func (h *harness) patchImage(t *testing.T, token string, id int64,
	fields map[string][]string) (int, []byte) {
	t.Helper()

	ctype, body := imagePatch(t, fields, nil, "", "")
	resp := h.request("PATCH", "/images/"+itoa64(id), token, bytesReader(body), ctype)
	return resp.StatusCode, readAll(t, resp)
}

// imageState reads a column from an image row.
func (h *harness) imageState(t *testing.T, id int64, column string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var value *string
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT `+column+`::text FROM "Images" WHERE "Id" = $1`, id).Scan(&value); err != nil {
		t.Fatalf("read %s of image %d: %v", column, id, err)
	}
	if value == nil {
		return ""
	}
	return *value
}

func (h *harness) imageTags(t *testing.T, id int64) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := h.db.Pool().Query(ctx, `
        SELECT t."Slug" FROM "ImageTag" it
        JOIN "Tags" t ON t."Id" = it."TagsId"
        WHERE it."ImagesId" = $1 ORDER BY t."Slug"`, id)
	if err != nil {
		t.Fatalf("read tags: %v", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatalf("scan slug: %v", err)
		}
		out = append(out, slug)
	}
	return out
}

func (h *harness) imageArtists(t *testing.T, id int64) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var n int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT count(*) FROM "ArtistImage" WHERE "ImagesId" = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count artists: %v", err)
	}
	return n
}

// TestImagePatchIsModeratorOnly is the regression test for the privilege leak.
//
// ImagesController.Update is [Authorize(Policy = Moderator)] and
// UpdateImageCommand contains no ownership check, so an ordinary user cannot
// edit an image at all. The previous handler let the uploader in, which read the
// command and missed the policy on the controller.
func TestImagePatchIsModeratorOnly(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, UploaderID: ptr(int64(51))}.insert(t, h)

	for _, role := range []config.Role{config.RoleUser, config.RoleTrustedUser} {
		token := h.issueToken(51, role)
		status, body := h.patchImage(t, token, imageID,
			map[string][]string{"source": {"https://example.com/hijacked"}})

		if status != http.StatusForbidden {
			t.Errorf("role %d patching their own image = %d, want 403: %s", role, status, body)
		}
	}

	// The content was not touched by the refused attempts.
	if got := h.imageState(t, imageID, `"Source"`); got != "" {
		t.Errorf("a refused patch wrote source %q", got)
	}

	moderator := h.issueToken(61, config.RoleModerator)
	status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"source": {"https://example.com/ok"}})
	if status != http.StatusOK {
		t.Fatalf("moderator patch = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"Source"`); got != "https://example.com/ok" {
		t.Errorf("source = %q, want the patch applied", got)
	}
}

// TestImageDeleteIsAdminOnly covers DELETE /images/{id}.
//
// DeleteImageCommand has an ownership allowance, but the controller's Admin
// policy short-circuits before the command runs, so the allowance is
// unreachable over HTTP. A user withdrawing their own upload is what
// DELETE /review/tasks/{id}/cancel is for.
func TestImageDeleteIsAdminOnly(t *testing.T) {
	h := newHarness(t)

	mine := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, UploaderID: ptr(int64(51))}.insert(t, h)

	for _, role := range []config.Role{config.RoleUser, config.RoleTrustedUser, config.RoleModerator} {
		resp := h.request("DELETE", "/images/"+itoa64(mine), h.issueToken(51, role), nil, "")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("role %d deleting their own image = %d, want 403", role, resp.StatusCode)
		}
		resp.Body.Close()
	}

	if got := h.imageState(t, mine, `"Id"`); got == "" {
		t.Error("the image was deleted by a non-admin")
	}

	admin := h.issueToken(71, config.RoleAdmin)
	resp := h.request("DELETE", "/images/"+itoa64(mine), admin, nil, "")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Fatalf("admin delete = %d: %s", status, body)
	}

	resp = h.request("GET", "/images/"+itoa64(mine), "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("deleted image = %d, want 404", resp.StatusCode)
	}
}

// TestImagePatchRequiresMultipart pins the binding.
//
// The C# takes [FromForm]. A JSON body is not a form, so it is a 400 rather
// than a silently-ignored patch that returns 200 having changed nothing.
func TestImagePatchRequiresMultipart(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	resp := h.request("PATCH", "/images/"+itoa64(imageID), moderator,
		bytesReader(`{"source":"https://example.com/json"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("JSON body on a [FromForm] endpoint = %d, want 400", resp.StatusCode)
	}
	if got := h.imageState(t, imageID, `"Source"`); got != "" {
		t.Errorf("a rejected body still wrote source %q", got)
	}
}

// TestImagePatchSourceIsThreeState covers the absent/empty/set distinction.
//
// UpdateImageCommand does `image.Source = request.Source.ToNullIfEmpty()?.Trim()`
// unconditionally, so there are only two outcomes: a trimmed value, or NULL.
// There is no "leave it alone" state, and a patch that omits the field clears
// the attribution.
func TestImagePatchSourceIsThreeState(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	h.setImageSource(t, imageID, "https://example.com/original")
	moderator := h.issueToken(61, config.RoleModerator)

	if got := h.imageState(t, imageID, `"Source"`); got != "https://example.com/original" {
		t.Fatalf("seeded source = %q", got)
	}

	// Set, with surrounding whitespace trimmed.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"source": {"  https://example.com/new  "}}); status != http.StatusOK {
		t.Fatalf("set source = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"Source"`); got != "https://example.com/new" {
		t.Errorf("source = %q, want it trimmed", got)
	}

	// Present but empty clears it.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"source": {""}}); status != http.StatusOK {
		t.Fatalf("clear source = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"Source"`); got != "" {
		t.Errorf("source = %q, want it cleared to NULL", got)
	}

	// Omitted clears it too, because the field is written unconditionally.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"isNsfw": {"false"}}); status != http.StatusOK {
		t.Fatalf("patch without source = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"Source"`); got != "" {
		t.Errorf("source = %q; omitting the field must clear it, not preserve it", got)
	}
}

func ptr[T any](v T) *T { return &v }

// setImageSource writes a source URL directly, for seeding state the helper
// does not model.
func (h *harness) setImageSource(t *testing.T, id int64, source string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := h.db.Pool().Exec(ctx,
		`UPDATE "Images" SET "Source" = $1 WHERE "Id" = $2`, source, id); err != nil {
		t.Fatalf("set source: %v", err)
	}
}

// TestImagePatchOmittedIsNsfwDefaultsFalse pins the non-nullable bool.
//
// C# binds IsNsfw as a plain bool, so a form without the field binds false and
// the assignment is unconditional. A patch that only renames tags therefore
// marks the image SFW.
func TestImagePatchOmittedIsNsfwDefaultsFalse(t *testing.T) {
	h := newHarness(t)

	nsfw := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, IsNsfw: true}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	// A patch with no isNsfw field writes false.
	if status, body := h.patchImage(t, moderator, nsfw,
		map[string][]string{"source": {"https://example.com/x"}}); status != http.StatusOK {
		t.Fatalf("patch = %d: %s", status, body)
	}
	if got := h.imageState(t, nsfw, `"IsNsfw"`); got != "false" {
		t.Errorf("IsNsfw = %q; an omitted non-nullable bool binds false in C#", got)
	}

	// An explicit true round-trips.
	if status, body := h.patchImage(t, moderator, nsfw,
		map[string][]string{"isNsfw": {"true"}}); status != http.StatusOK {
		t.Fatalf("patch = %d: %s", status, body)
	}
	if got := h.imageState(t, nsfw, `"IsNsfw"`); got != "true" {
		t.Errorf("IsNsfw = %q, want true", got)
	}

	// A nonsense value is a validation failure, not a silent false.
	status, body := h.patchImage(t, moderator, nsfw,
		map[string][]string{"isNsfw": {"maybe"}})
	if status != http.StatusBadRequest {
		t.Errorf("unparseable isNsfw = %d, want 400: %s", status, body)
	}
	if got := h.imageState(t, nsfw, `"IsNsfw"`); got != "true" {
		t.Errorf("IsNsfw = %q; a rejected patch must not write the field", got)
	}
}

// TestImagePatchOmittedUserIdClearsUploader covers the sharpest edge here.
//
// UpdateImageCommand has an else branch that sets UploaderId to null when no
// UserId is supplied. It looks like a bug and reads like one, but it is the
// contract, and a rewrite that "helpfully" preserves the uploader would leave
// the moderator screen quietly rewriting attribution history.
func TestImagePatchOmittedUserIdClearsUploader(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, UploaderID: ptr(int64(51))}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	if got := h.imageState(t, imageID, `"UploaderId"`); got != "51" {
		t.Fatalf("seeded uploader = %q", got)
	}

	// Omitting userId clears it.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"source": {"https://example.com/x"}}); status != http.StatusOK {
		t.Fatalf("patch = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"UploaderId"`); got == "51" {
		t.Error("uploader was preserved; UpdateImageCommand clears it when UserId is absent")
	}

	// An explicit id sets it.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"userId": {"52"}}); status != http.StatusOK {
		t.Fatalf("patch with userId = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"UploaderId"`); got != "52" {
		t.Errorf("uploader = %q, want 52", got)
	}

	// Zero clears it too.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"userId": {"0"}}); status != http.StatusOK {
		t.Fatalf("patch with userId=0 = %d: %s", status, body)
	}
	if got := h.imageState(t, imageID, `"UploaderId"`); got != "" {
		t.Errorf("uploader = %q, want NULL for userId=0", got)
	}

	// An unknown id is a 404, not a foreign key violation.
	status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"userId": {"999999"}})
	if status != http.StatusNotFound {
		t.Errorf("unknown uploader = %d, want 404: %s", status, body)
	}
}

// TestImagePatchReplacesTagsAndArtists covers the replace semantics.
func TestImagePatchReplacesTagsAndArtists(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "waifu")
	ensureTag(t, h, "maid")
	ensureTag(t, h, "catgirl")
	ensureArtist(t, h, "Miku")

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"waifu"}}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	// Replace with a different set: "waifu" goes, "maid" and "catgirl" arrive.
	status, body := h.patchImage(t, moderator, imageID, map[string][]string{
		"tags": {"maid", "catgirl"},
	})
	if status != http.StatusOK {
		t.Fatalf("replace tags = %d: %s", status, body)
	}
	assertSameIDs(t, slugsToIDsFor(t, h, h.imageTags(t, imageID)...),
		slugsToIDsFor(t, h, "maid", "catgirl"), "replaced tags")

	// Omitting the field leaves them alone.
	status, body = h.patchImage(t, moderator, imageID,
		map[string][]string{"source": {"https://example.com/keep"}})
	if status != http.StatusOK {
		t.Fatalf("patch = %d: %s", status, body)
	}
	if got := len(h.imageTags(t, imageID)); got != 2 {
		t.Errorf("tags after an unrelated patch = %d, want them untouched", got)
	}

	// Present but empty clears them, because C# runs Clear() before the Any()
	// check rather than skipping the block.
	status, body = h.patchImage(t, moderator, imageID, map[string][]string{"tags": {""}})
	if status != http.StatusOK {
		t.Fatalf("clear tags = %d: %s", status, body)
	}
	if got := len(h.imageTags(t, imageID)); got != 0 {
		t.Errorf("tags = %v, want an empty set", got)
	}

	// Artists replace the same way.
	artistID := ensureArtist(t, h, "Rin")
	status, body = h.patchImage(t, moderator, imageID, map[string][]string{
		"artists": {itoa64(artistID)},
	})
	if status != http.StatusOK {
		t.Fatalf("set artists = %d: %s", status, body)
	}
	if got := h.imageArtists(t, imageID); got != 1 {
		t.Errorf("artists = %d, want 1", got)
	}
}

func slugsToIDsFor(t *testing.T, h *harness, slugs ...string) []int64 {
	t.Helper()

	out := make([]int64, 0, len(slugs))
	for _, slug := range slugs {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var id int64
		err := h.db.Pool().QueryRow(ctx,
			`SELECT "Id" FROM "Tags" WHERE "Slug" = $1`, slug).Scan(&id)
		cancel()
		if err != nil {
			t.Fatalf("read tag %q: %v", slug, err)
		}
		out = append(out, id)
	}
	return out
}

// TestImagePatchRejectsUnknownTags is the all-or-nothing rule.
//
// C# compares foundTags.Count to the requested count and throws, so one unknown
// slug rejects the whole patch rather than silently applying the rest.
func TestImagePatchRejectsUnknownTags(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "waifu")

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"waifu"}}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	status, body := h.patchImage(t, moderator, imageID, map[string][]string{
		"tags": {"waifu", "does-not-exist"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("unknown slug = %d, want 400: %s", status, body)
	}
	if !strings.Contains(string(body), "does-not-exist") {
		t.Errorf("the error should name the offending slug: %s", body)
	}

	// The valid part of the list was not applied.
	if got := len(h.imageTags(t, imageID)); got != 1 {
		t.Errorf("tags = %d; a rejected patch must change nothing", got)
	}
}

// TestImagePatchAcceptingClosesTheReviewTask covers the queue sync.
//
// A moderator accepting from the moderation screen rather than the queue has to
// close the open NewContent entry, or the same upload stays queued and gets
// approved again by the next moderator.
func TestImagePatchAcceptingClosesTheReviewTask(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 0, UploaderID: ptr(int64(51))}.insert(t, h)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := h.db.Pool().Exec(ctx, `
        INSERT INTO "ReviewTasks" ("Kind", "TargetType", "TargetId", "SubmitterId")
        VALUES (0, 0, $1, 51)`, imageID); err != nil {
		t.Fatalf("seed review task: %v", err)
	}

	moderator := h.issueToken(61, config.RoleModerator)
	status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"reviewStatus": {"Accepted"}})
	if status != http.StatusOK {
		t.Fatalf("accept = %d: %s", status, body)
	}

	if got := h.imageState(t, imageID, `"ReviewStatus"`); got != "1" {
		t.Errorf("image ReviewStatus = %q, want 1", got)
	}

	var status2 int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "Status" FROM "ReviewTasks" WHERE "TargetId" = $1`, imageID).
		Scan(&status2); err != nil {
		t.Fatalf("read task: %v", err)
	}
	if status2 != 1 {
		t.Errorf("review task status = %d, want 1 (Approved); accepting must close the queue entry", status2)
	}
}

// TestImagePatchRejectsBadReviewStatus covers the enum parsing.
func TestImagePatchRejectsBadReviewStatus(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"reviewStatus": {"Banana"}})
	if status != http.StatusBadRequest {
		t.Errorf("bad review status = %d, want 400: %s", status, body)
	}

	// The numeric form is accepted, matching the JsonStringEnumConverter.
	if status, body := h.patchImage(t, moderator, imageID,
		map[string][]string{"reviewStatus": {"0"}}); status != http.StatusOK {
		t.Errorf("numeric review status = %d, want 200: %s", status, body)
	}
}

// TestImagePatchFileReplacement covers the optional new file.
func TestImagePatchFileReplacement(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	moderator := h.issueToken(61, config.RoleModerator)

	replacement := testImage(t, 120, 80, color.RGBA{200, 40, 40, 255})
	ctype, body := imagePatch(t,
		map[string][]string{"userId": {itoa64(51)}},
		replacement, "new.png", "image/png")

	resp := h.request("PATCH", "/images/"+itoa64(imageID), moderator, bytesReader(body), ctype)
	if status, payload := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Fatalf("replace file = %d: %s", status, payload)
	}

	// Dimensions are re-measured from the new bytes.
	if got := h.imageState(t, imageID, `"Width"`); got != "120" {
		t.Errorf("width = %q, want 120 after replacement", got)
	}
	if got := h.imageState(t, imageID, `"Height"`); got != "80" {
		t.Errorf("height = %q, want 80 after replacement", got)
	}

	// The new object is stored under the same id.
	if !h.hasObject(storageKeyFor(imageID, ".png")) {
		t.Errorf("the replacement was not written to storage; keys=%v", h.objectKeys(t))
	}

	// A replacement with a disallowed declared type is refused outright.
	ctype, body = imagePatch(t, nil, testImage(t, 90, 90, color.RGBA{1, 2, 3, 255}),
		"new.png", "application/octet-stream")
	resp = h.request("PATCH", "/images/"+itoa64(imageID), moderator, bytesReader(body), ctype)
	if status, payload := resp.StatusCode, readAll(t, resp); status != http.StatusBadRequest {
		t.Errorf("octet-stream replacement = %d, want 400: %s", status, payload)
	}

	// The rejected replacement did not alter the stored metadata.
	if got := h.imageState(t, imageID, `"Width"`); got != "120" {
		t.Errorf("width = %q; a rejected replacement must not change the image", got)
	}
}

// TestImagePatchUnknownImageIs404 covers the lookup.
func TestImagePatchUnknownImageIs404(t *testing.T) {
	h := newHarness(t)

	moderator := h.issueToken(61, config.RoleModerator)

	status, body := h.patchImage(t, moderator, 999999,
		map[string][]string{"source": {"https://example.com/x"}})
	if status != http.StatusNotFound {
		t.Errorf("patching a missing image = %d, want 404: %s", status, body)
	}

	resp := h.request("DELETE", "/images/999999", h.issueToken(71, config.RoleAdmin), nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("deleting a missing image = %d, want 404", resp.StatusCode)
	}
}
