package httpapi_test

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// Storage diff: reconciling S3 against the database.
//
// The endpoint exists because the two can drift, and the whole point is that it
// reports drift in BOTH directions without conflating them:
//
//   - OrphansInS3: a file with no database row. The file is unreferenced and
//     can be deleted from S3.
//   - MissingFromS3: a row with no file. The row points at nothing and can be
//     deleted from the database.
//
// Both cleanups are destructive and irreversible, which is why the controller is
// Admin-only at the class level and why the delete routes are tested for that
// gate rather than assumed from the read route.

// putObject writes a file into the fake store.
func (h *harness) putObject(t *testing.T, key string) {
	t.Helper()

	if err := h.store.Put(context.Background(), key, bytesReader("data"), ""); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
}

// objectKey is the storage key for an image: its id and extension, which is how
// the storage-diff query pairs files with rows.
func objectKey(id int64) string {
	return itoa64(id) + ".png"
}

// objectKeys lists everything currently in the fake store.
func (h *harness) objectKeys(t *testing.T) []string {
	t.Helper()

	keys, err := h.store.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sort.Strings(keys)
	return keys
}

// hasObject reports whether a key is present.
func (h *harness) hasObject(key string) bool {
	_, ok := h.store.objects[key]
	return ok
}

// TestStorageDiffIsAdminOnly covers the class-level gate.
func TestStorageDiffIsAdminOnly(t *testing.T) {
	h := newHarness(t)

	cases := []struct{ method, path string }{
		{"GET", "/storage-diff"},
		{"DELETE", "/storage-diff/s3"},
		{"DELETE", "/storage-diff/db"},
	}

	for _, tc := range cases {
		resp := h.request(tc.method, tc.path, "", bytesReader(`{}`), "application/json")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
		resp.Body.Close()

		// A moderator is not enough either.
		resp = h.request(tc.method, tc.path, h.issueToken(61, config.RoleModerator),
			bytesReader(`{}`), "application/json")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("moderator %s %s = %d, want 403", tc.method, tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestStorageDiffRequiresAMode pins the [Required] on Mode.
//
// Mode is an enum, so its zero value is a real member (OrphansInS3) rather than
// "absent". Without an explicit guard the endpoint would silently report one
// direction when the caller simply forgot the parameter, which reads as "no
// drift found" rather than "you did not ask a question".
func TestStorageDiffRequiresAMode(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/storage-diff", h.issueToken(71, config.RoleAdmin), nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("storage diff with no mode = %d, want 400", resp.StatusCode)
	}
}

// TestStorageDiffBothDirections is the core behaviour.
func TestStorageDiffBothDirections(t *testing.T) {
	h := newHarness(t)

	// Two rows that reference files, one of which is absent from storage.
	backed := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	backfilled := seededImage{Width: 11, Height: 11, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	// Only the first row's file is actually present in storage.
	h.putObject(t, objectKey(backed))

	// One file in storage with no row behind it.
	h.putObject(t, "orphan.png")

	admin := h.issueToken(71, config.RoleAdmin)

	// OrphansInS3 lists the unreferenced file.
	body := readAll(t, h.request("GET", "/storage-diff?mode=OrphansInS3", admin, nil, "")) //nolint:bodyclose
	if !strings.Contains(string(body), "orphan.png") {
		t.Errorf("OrphansInS3 should list the unreferenced file: %s", body)
	}
	if strings.Contains(string(body), objectKey(backed)) {
		t.Errorf("OrphansInS3 should not list a file a row references: %s", body)
	}

	// MissingFromS3 lists the row with no file.
	body = readAll(t, h.request("GET", "/storage-diff?mode=MissingFromS3", admin, nil, "")) //nolint:bodyclose
	if !strings.Contains(string(body), `"id":`+itoa64(backfilled)) {
		t.Errorf("MissingFromS3 should list the row whose file is gone: %s", body)
	}
	if strings.Contains(string(body), `"id":`+itoa64(backed)) {
		t.Errorf("MissingFromS3 should not list a row whose file exists: %s", body)
	}

}

// TestStorageDiffCountsPendingImages pins a behaviour that looks like a bug.
//
// GetStorageDiffQuery loads Images with no review-status predicate, so a pending
// upload with no file yet is reported as MissingFromS3. That reads as a flaw but
// is the contract: the rewrite has to reproduce the original, not improve on it,
// and "fixing" it would make the endpoint disagree with whatever an operator
// already learned to expect from it.
func TestStorageDiffCountsPendingImages(t *testing.T) {
	h := newHarness(t)

	pending := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 0}.insert(t, h)

	admin := h.issueToken(71, config.RoleAdmin)
	body := readAll(t, h.request("GET", "/storage-diff?mode=MissingFromS3", admin, nil, "")) //nolint:bodyclose

	if got := jsonIntOrZero(t, body, "totalCount"); got != 1 {
		t.Errorf("totalCount = %d, want 1; the C# query does not filter on review status", got)
	}
	if !strings.Contains(string(body), `"id":`+itoa64(pending)) {
		t.Errorf("the pending image should be the one reported: %s", body)
	}
}

// TestStorageDiffSkipsHealthcheckObjects covers the one key filter that does
// exist. The bucket holds a connectivity probe under "healthcheck/", which is
// deliberately not a catalogue entry, so reporting it as an orphan every run
// would be pure noise.
func TestStorageDiffSkipsHealthcheckObjects(t *testing.T) {
	h := newHarness(t)

	h.putObject(t, "healthcheck/ping")
	h.putObject(t, "healthcheck/deep/nested")

	admin := h.issueToken(71, config.RoleAdmin)
	body := readAll(t, h.request("GET", "/storage-diff?mode=OrphansInS3", admin, nil, "")) //nolint:bodyclose

	if strings.Contains(string(body), "healthcheck") {
		t.Errorf("healthcheck objects must not be reported as orphans: %s", body)
	}
	if got := jsonIntOrZero(t, body, "totalCount"); got != 0 {
		t.Errorf("totalCount = %d, want 0", got)
	}

	// A file that merely starts with the same letters is still a real orphan.
	h.putObject(t, "healthchecked.png")
	body = readAll(t, h.request("GET", "/storage-diff?mode=OrphansInS3", admin, nil, "")) //nolint:bodyclose
	if !strings.Contains(string(body), "healthchecked.png") {
		t.Errorf("only the healthcheck/ prefix is excluded: %s", body)
	}
}

// TestDeleteS3Objects covers the S3 cleanup route.
func TestDeleteS3Objects(t *testing.T) {
	h := newHarness(t)

	h.putObject(t, "orphan-one.png")
	h.putObject(t, "orphan-two.png")
	h.putObject(t, "keep.png")

	admin := h.issueToken(71, config.RoleAdmin)

	resp := h.request("DELETE", "/storage-diff/s3", admin,
		bytesReader(`{"fileNames":["orphan-one.png","orphan-two.png"]}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Fatalf("delete s3 objects = %d: %s", status, body)
	}

	remaining := h.objectKeys(t)
	if len(remaining) != 1 || remaining[0] != "keep.png" {
		t.Errorf("after deleting two orphans, store holds %v, want only keep.png", remaining)
	}

	// Deleting a key that is already gone is not an error: the caller's intent
	// is satisfied either way, and failing would make cleanup awkward to retry.
	resp = h.request("DELETE", "/storage-diff/s3", admin,
		bytesReader(`{"fileNames":["orphan-one.png"]}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Errorf("deleting an absent key = %d: %s", status, body)
	}
}

// TestDeleteDbRecords covers the database cleanup route.
func TestDeleteDbRecords(t *testing.T) {
	h := newHarness(t)

	gone := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	survivor := seededImage{Width: 11, Height: 11, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	admin := h.issueToken(71, config.RoleAdmin)

	resp := h.request("DELETE", "/storage-diff/db", admin,
		bytesReader(`{"imageIds":[`+itoa64(gone)+`]}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
		t.Fatalf("delete db records = %d: %s", status, body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var remaining int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT count(*) FROM "Images" WHERE "Id" = $1`, survivor).Scan(&remaining); err != nil {
		t.Fatalf("count survivor: %v", err)
	}
	if remaining != 1 {
		t.Errorf("the surviving image was also deleted")
	}

	var deleted int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT count(*) FROM "Images" WHERE "Id" = $1`, gone).Scan(&deleted); err != nil {
		t.Fatalf("count deleted: %v", err)
	}
	if deleted != 0 {
		t.Errorf("image %d still present after DELETE /storage-diff/db", gone)
	}
}

// TestStorageDiffPaginate covers the envelope on a paginated endpoint.
func TestStorageDiffPaginate(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < 3; i++ {
		h.putObject(t, "orphan-"+itoa(i)+".png")
	}

	admin := h.issueToken(71, config.RoleAdmin)
	body := readAll(t, h.request("GET", "/storage-diff?mode=OrphansInS3&pageSize=2", admin, nil, "")) //nolint:bodyclose

	if got := jsonIntOrZero(t, body, "totalCount"); got != 3 {
		t.Errorf("totalCount = %d, want 3 regardless of page size", got)
	}
	if !strings.Contains(string(body), `"hasNextPage":true`) {
		t.Errorf("hasNextPage should be true with 3 orphans and a page size of 2: %s", body)
	}
}
