package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// Album, report and user-administration coverage.

// TestAlbumLifecycle covers create, list, update and delete.
func TestAlbumLifecycle(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(30, config.RoleUser)

	created := h.request("POST", "/users/30/albums", token,
		bytesReader(`{"name":"Favourites","description":"the good ones"}`), "application/json")
	if created.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d: %s", created.StatusCode, readAll(t, created))
	}

	albumID, err := jsonInt(t, readAll(t, created), "id")
	if err != nil {
		t.Fatalf("no album id: %v", err)
	}

	list := h.request("GET", "/users/30/albums", token, nil, "")
	defer list.Body.Close()
	if !containsBytes(readAll(t, list), "Favourites") {
		t.Error("the album is missing from the listing")
	}

	updated := h.request("PUT", "/users/30/albums/"+itoa(albumID), token,
		bytesReader(`{"name":"Best","description":"updated"}`), "application/json")
	defer updated.Body.Close()
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d", updated.StatusCode)
	}
	if got := jsonString(t, readAll(t, updated), "name"); got != "Best" {
		t.Errorf("name = %q, want Best", got)
	}

	deleted := h.request("DELETE", "/users/30/albums/"+itoa(albumID), token, nil, "")
	defer deleted.Body.Close()
	if deleted.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", deleted.StatusCode)
	}
}

// TestAlbumOwnershipIsEnforced checks one user cannot touch another's album.
//
// The response must be 404 rather than 403 so the existence of another account's
// album is not disclosed.
func TestAlbumOwnershipIsEnforced(t *testing.T) {
	h := newHarness(t)

	owner := h.issueToken(31, config.RoleUser)
	created := h.request("POST", "/users/31/albums", owner,
		bytesReader(`{"name":"Private"}`), "application/json")
	albumID, err := jsonInt(t, readAll(t, created), "id")
	if err != nil {
		t.Fatalf("no album id: %v", err)
	}

	intruder := h.issueToken(32, config.RoleUser)

	read := h.request("GET", "/users/31/albums/"+itoa(albumID), intruder, nil, "")
	defer read.Body.Close()
	if read.StatusCode != http.StatusNotFound {
		t.Errorf("read status = %d, want 404", read.StatusCode)
	}

	remove := h.request("DELETE", "/users/31/albums/"+itoa(albumID), intruder, nil, "")
	defer remove.Body.Close()
	if remove.StatusCode != http.StatusNotFound {
		t.Errorf("delete status = %d, want 404", remove.StatusCode)
	}
}

// TestAlbumItems checks adding and removing images.
func TestAlbumItems(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(33, config.RoleTrustedUser)
	imageID := seedAcceptedImage(t, h)

	created := h.request("POST", "/users/33/albums", token,
		bytesReader(`{"name":"With image"}`), "application/json")
	albumID, err := jsonInt(t, readAll(t, created), "id")
	if err != nil {
		t.Fatalf("no album id: %v", err)
	}

	added := h.request("POST", "/users/33/albums/"+itoa(albumID)+"/items", token,
		bytesReader(`{"imageId":`+itoa(imageID)+`}`), "application/json")
	if added.StatusCode != http.StatusOK {
		t.Fatalf("add item status = %d: %s", added.StatusCode, readAll(t, added))
	}

	if got, err := jsonInt(t, readAll(t, added), "imageCount"); err != nil || got != 1 {
		t.Errorf("imageCount = %d (err %v), want 1", got, err)
	}

	// Adding the same image again is a no-op rather than an error, so a client
	// can retry safely.
	again := h.request("POST", "/users/33/albums/"+itoa(albumID)+"/items", token,
		bytesReader(`{"imageId":`+itoa(imageID)+`}`), "application/json")
	defer again.Body.Close()
	if again.StatusCode != http.StatusOK {
		t.Errorf("repeated add status = %d, want 200", again.StatusCode)
	}

	removed := h.request("DELETE",
		"/users/33/albums/"+itoa(albumID)+"/items/"+itoa(imageID), token, nil, "")
	defer removed.Body.Close()
	if removed.StatusCode != http.StatusNoContent {
		t.Errorf("remove status = %d, want 204", removed.StatusCode)
	}
}

// TestDefaultAlbumIsExclusive checks setting a default clears the previous one.
//
// Two defaults would make the "default album" the user lands on ambiguous.
func TestDefaultAlbumIsExclusive(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(34, config.RoleUser)

	first := h.request("POST", "/users/34/albums", token,
		bytesReader(`{"name":"First"}`), "application/json")
	firstID, err := jsonInt(t, readAll(t, first), "id")
	if err != nil {
		t.Fatalf("no first album id: %v", err)
	}

	second := h.request("POST", "/users/34/albums", token,
		bytesReader(`{"name":"Second"}`), "application/json")
	secondID, err := jsonInt(t, readAll(t, second), "id")
	if err != nil {
		t.Fatalf("no second album id: %v", err)
	}

	setFirst := h.request("PATCH", "/users/34/albums/default", token,
		bytesReader(`{"imageId":`+itoa(firstID)+`}`), "application/json")
	defer setFirst.Body.Close()
	if setFirst.StatusCode != http.StatusNoContent {
		t.Fatalf("set default status = %d: %s", setFirst.StatusCode, readAll(t, setFirst))
	}

	setSecond := h.request("PATCH", "/users/34/albums/default", token,
		bytesReader(`{"imageId":`+itoa(secondID)+`}`), "application/json")
	defer setSecond.Body.Close()
	if setSecond.StatusCode != http.StatusNoContent {
		t.Fatalf("set second default status = %d", setSecond.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var defaults int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT COUNT(*) FROM "Album" WHERE "UserId" = $1 AND "IsDefault" = true`, 34).
		Scan(&defaults); err != nil {
		t.Fatalf("count defaults: %v", err)
	}
	if defaults != 1 {
		t.Errorf("%d default albums, want exactly 1", defaults)
	}
}

// TestAlbumRequiresAuthentication guards the whole group.
func TestAlbumRequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/users/35/albums", "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestReportLifecycle covers filing and resolving a report.
func TestReportLifecycle(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(36, config.RoleUser)
	imageID := seedAcceptedImage(t, h)

	created := h.request("POST", "/reports", token,
		bytesReader(`{"imageId":`+itoa(imageID)+`,"description":"wrong attribution"}`),
		"application/json")
	if created.StatusCode != http.StatusOK {
		t.Fatalf("create status = %d: %s", created.StatusCode, readAll(t, created))
	}

	reportID, err := jsonInt(t, readAll(t, created), "id")
	if err != nil {
		t.Fatalf("no report id: %v", err)
	}
	if got := jsonString(t, readAll(t, h.request("GET", "/reports/"+itoa(reportID), token, nil, "")), "status"); got != "Pending" {
		t.Errorf("new report status = %q, want Pending", got)
	}

	// Resolving requires a moderator.
	forbidden := h.request("PATCH", "/reports/"+itoa(reportID), token,
		bytesReader(`{"status":"Accepted"}`), "application/json")
	forbidden.Body.Close()
	if forbidden.StatusCode != http.StatusForbidden {
		t.Errorf("user resolving a report: status = %d, want 403", forbidden.StatusCode)
	}

	mod := h.issueToken(37, config.RoleModerator)
	resolved := h.request("PATCH", "/reports/"+itoa(reportID), mod,
		bytesReader(`{"status":"Accepted","reviewerNote":"credit restored"}`), "application/json")
	defer resolved.Body.Close()

	if resolved.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d: %s", resolved.StatusCode, readAll(t, resolved))
	}

	body := readAll(t, resolved)
	if got := jsonString(t, body, "status"); got != "Accepted" {
		t.Errorf("status = %q, want Accepted", got)
	}
	if got := jsonString(t, body, "reviewerNote"); got != "credit restored" {
		t.Errorf("reviewerNote = %q, want the submitted note", got)
	}
}

// TestReportRequiresExistingImage checks validation.
func TestReportRequiresExistingImage(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(38, config.RoleUser)
	resp := h.request("POST", "/reports", token,
		bytesReader(`{"imageId":999999,"description":"nope"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a missing image", resp.StatusCode)
	}
}

// TestReportQueueScoping checks ordinary users see only their own reports.
func TestReportQueueScoping(t *testing.T) {
	h := newHarness(t)

	imageID := seedAcceptedImage(t, h)

	first := h.issueToken(39, config.RoleUser)
	second := h.issueToken(40, config.RoleUser)

	h.request("POST", "/reports", first,
		bytesReader(`{"imageId":`+itoa(imageID)+`,"description":"one"}`),
		"application/json").Body.Close()
	h.request("POST", "/reports", second,
		bytesReader(`{"imageId":`+itoa(imageID)+`,"description":"two"}`),
		"application/json").Body.Close()

	ownQueue := h.request("GET", "/reports", first, nil, "")
	defer ownQueue.Body.Close()
	if got := scanTotal(readAll(t, ownQueue)); got != 1 {
		t.Errorf("ordinary user sees %d reports, want only their own 1", got)
	}

	mod := h.issueToken(41, config.RoleModerator)
	modQueue := h.request("GET", "/reports", mod, nil, "")
	defer modQueue.Body.Close()
	if got := scanTotal(readAll(t, modQueue)); got != 2 {
		t.Errorf("moderator sees %d reports, want 2", got)
	}
}

// TestUserAdministrationIsAdminOnly guards the listing and role endpoints.
func TestUserAdministrationIsAdminOnly(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(42, config.RoleUser)

	list := h.request("GET", "/users", token, nil, "")
	defer list.Body.Close()
	if list.StatusCode != http.StatusForbidden {
		t.Errorf("list users as a user: status = %d, want 403", list.StatusCode)
	}

	// A moderator is still not an admin.
	mod := h.issueToken(43, config.RoleModerator)
	promote := h.request("PATCH", "/users/42/role", mod,
		bytesReader(`{"role":"Admin"}`), "application/json")
	defer promote.Body.Close()
	if promote.StatusCode != http.StatusForbidden {
		t.Errorf("promote as a moderator: status = %d, want 403", promote.StatusCode)
	}
}

// TestAdminCanChangeRoles covers the promotion path.
//
// This replaces the manual SQL step the README describes, so the change is
// attributable rather than an unexplained row update.
func TestAdminCanChangeRoles(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(44, config.RoleAdmin)

	promoted := h.request("PATCH", "/users/45/role", admin,
		bytesReader(`{"role":"TrustedUser"}`), "application/json")
	defer promoted.Body.Close()

	if promoted.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", promoted.StatusCode, readAll(t, promoted))
	}
	if got := jsonString(t, readAll(t, promoted), "role"); got != "TrustedUser" {
		t.Errorf("role = %q, want TrustedUser", got)
	}

	// The change must persist, not just appear in the response.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var role int
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "Role" FROM "User" WHERE "Id" = 45`).Scan(&role); err != nil {
		t.Fatalf("read role: %v", err)
	}
	if role != 1 {
		t.Errorf("stored role = %d, want 1 (TrustedUser)", role)
	}
}

// TestRoleUpdateRejectsUnknownRole checks validation rather than a silent default.
func TestRoleUpdateRejectsUnknownRole(t *testing.T) {
	h := newHarness(t)

	admin := h.issueToken(46, config.RoleAdmin)
	resp := h.request("PATCH", "/users/46/role", admin,
		bytesReader(`{"role":"Supremo"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// seedAcceptedImage inserts an accepted image and returns its id.
func seedAcceptedImage(t *testing.T, h *harness) int {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int
	err := h.db.Pool().QueryRow(ctx, `
        INSERT INTO "Image" ("Width","Height","ByteSize","Extension","DominantColor",
                              "IsNsfw","IsAnimated","PerceptualHash","ReviewStatus")
        VALUES (640,480,2048,'.png','#123456',false,false,
                repeat('0', 64)::bit(64), 1)
        RETURNING "Id"`).Scan(&id)
	if err != nil {
		t.Fatalf("seed image: %v", err)
	}
	return id
}
