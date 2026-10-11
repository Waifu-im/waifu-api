package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// Report lifecycle.
//
// A report is one of the few resources where the *author* and the *moderator*
// have different powers over the same row, so the scoping rules are the
// substance here:
//
//   - Non-moderators are scoped to their own reports unconditionally. There is no
//     query parameter that widens that.
//   - A moderator sees everyone's, unless they pass mine=true.
//   - Only the author or a moderator may edit, resolve, reject or cancel.
//   - Every transition is Pending-only, and re-handling a settled report is a
//     409 rather than a silent overwrite.

// reportField reads one column of a report.
func (h *harness) reportField(t *testing.T, id int64, column string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var value string
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT `+column+`::text FROM "Reports" WHERE "Id" = $1`, id).Scan(&value); err != nil {
		t.Fatalf("read %s of report %d: %v", column, id, err)
	}
	return value
}

// createReport files a report and returns its id.
func (h *harness) createReport(t *testing.T, userID, imageID int64, description string) int64 {
	t.Helper()

	token := h.issueToken(userID, config.RoleUser)
	resp := h.request("POST", "/reports", token,
		bytesReader(`{"imageId":`+itoa64(imageID)+`,"description":"`+description+`"}`),
		"application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create report = %d: %s", resp.StatusCode, body)
	}
	return int64(jsonIntOrZero(t, body, "id"))
}

// TestCreateReport covers POST /reports and the duplicate guard.
func TestCreateReport(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	reportID := h.createReport(t, 51, imageID, "wrongly tagged")

	if got := h.reportField(t, reportID, `"Status"`); got != "0" && got != "Pending" {
		t.Errorf("new report status = %q, want pending", got)
	}

	token := h.issueToken(51, config.RoleUser)

	// One open report per user per image.
	resp := h.request("POST", "/reports", token,
		bytesReader(`{"imageId":`+itoa64(imageID)+`,"description":"again"}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("duplicate report = %d, want 409: %s", resp.StatusCode, readAll(t, resp))
	}

	// A different user may report the same image.
	resp = h.request("POST", "/reports", h.issueToken(52, config.RoleUser),
		bytesReader(`{"imageId":`+itoa64(imageID)+`,"description":"also wrong"}`), "application/json")
	if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusCreated {
		t.Errorf("second reporter = %d: %s", status, body)
	}
}

// TestCreateReportUnknownImage distinguishes 404 from 400.
func TestCreateReportUnknownImage(t *testing.T) {
	h := newHarness(t)

	token := h.issueToken(51, config.RoleUser)
	resp := h.request("POST", "/reports", token,
		bytesReader(`{"imageId":999999,"description":"x"}`), "application/json")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("report on a missing image = %d, want 404: %s",
			resp.StatusCode, readAll(t, resp))
	}
}

// TestListReportsScoping covers the mine=true rule.
func TestListReportsScoping(t *testing.T) {
	h := newHarness(t)

	first := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	second := seededImage{Width: 11, Height: 11, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	h.createReport(t, 51, first, "mine")
	h.createReport(t, 52, second, "theirs")

	moderator := h.issueToken(61, config.RoleModerator)

	// A moderator sees everything.
	listing := readAll(t, h.request("GET", "/reports", moderator, nil, "")) //nolint:bodyclose
	for _, want := range []string{"mine", "theirs"} {
		if !strings.Contains(string(listing), want) {
			t.Errorf("moderator listing should contain %q: %s", want, listing)
		}
	}

	// A non-moderator sees only their own, and no parameter widens that.
	for _, q := range []string{"", "?mine=false", "?mine=true"} {
		listing = readAll(t, h.request("GET", "/reports"+q, h.issueToken(51, config.RoleUser), nil, "")) //nolint:bodyclose
		if strings.Contains(string(listing), "theirs") {
			t.Errorf("ordinary user listing%s leaked another user's report: %s", q, listing)
		}
	}

	// A moderator can narrow to their own.
	moderatorReport := h.createReport(t, 61, first, "by-the-moderator")
	_ = moderatorReport
	listing = readAll(t, h.request("GET", "/reports?mine=true", moderator, nil, "")) //nolint:bodyclose
	if strings.Contains(string(listing), "theirs") {
		t.Errorf("mine=true should scope a moderator to their own reports: %s", listing)
	}
}

// TestReportTransitions covers the full lifecycle.
func TestReportTransitions(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	author := h.issueToken(51, config.RoleUser)
	moderator := h.issueToken(61, config.RoleModerator)

	for _, tc := range []struct {
		action string
		want   string
	}{
		{"validate", "Resolved"},
		{"reject", "Rejected"},
		{"cancel", "Cancelled"},
	} {
		reportID := h.createReport(t, 51, imageID, "for "+tc.action)

		// A non-moderator may not validate or reject.
		if tc.action != "cancel" {
			resp := h.request("PATCH", "/reports/"+itoa64(reportID)+"/"+tc.action, author, nil, "")
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("author %s = %d, want 403", tc.action, resp.StatusCode)
			}
		}

		resp := h.request("PATCH", "/reports/"+itoa64(reportID)+"/"+tc.action, moderator,
			bytesReader(`{"reason":"handled"}`), "application/json")
		if status, body := resp.StatusCode, readAll(t, resp); status != http.StatusNoContent {
			t.Fatalf("%s = %d: %s", tc.action, status, body)
		}

		if got := h.reportField(t, reportID, `"Status"`); got != tc.want && got != statusOf(tc.want) {
			t.Errorf("after %s, status = %q, want %s", tc.action, got, tc.want)
		}

		// A settled report cannot be settled again.
		resp = h.request("PATCH", "/reports/"+itoa64(reportID)+"/validate", moderator, nil, "")
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("re-validating a %s report = %d, want 409", tc.want, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func statusOf(name string) string {
	switch name {
	case "Pending":
		return "0"
	case "Resolved":
		return "1"
	case "Rejected":
		return "2"
	case "Cancelled":
		return "3"
	default:
		return ""
	}
}

// TestEditReport covers PATCH /reports/{id} and its guards.
func TestEditReport(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	reportID := h.createReport(t, 51, imageID, "first wording")

	author := h.issueToken(51, config.RoleUser)
	intruder := h.issueToken(52, config.RoleUser)
	moderator := h.issueToken(61, config.RoleModerator)

	// A bystander may not edit.
	resp := h.request("PATCH", "/reports/"+itoa64(reportID), intruder,
		bytesReader(`{"description":"not yours"}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("intruder editing = %d, want 403", resp.StatusCode)
	}

	// The author may, while it is open.
	resp = h.request("PATCH", "/reports/"+itoa64(reportID), author,
		bytesReader(`{"description":"clearer wording"}`), "application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("author editing = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "description"); got != "clearer wording" {
		t.Errorf("description = %q, want the revised text", got)
	}

	// A moderator may edit anyone's open report.
	resp = h.request("PATCH", "/reports/"+itoa64(reportID), moderator,
		bytesReader(`{"description":"moderator wording"}`), "application/json")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("moderator editing = %d, want 200", resp.StatusCode)
	}

	// Once resolved, editing is refused.
	resp = h.request("PATCH", "/reports/"+itoa64(reportID)+"/validate", moderator, nil, "")
	resp.Body.Close()

	resp = h.request("PATCH", "/reports/"+itoa64(reportID), author,
		bytesReader(`{"description":"too late"}`), "application/json")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("editing a resolved report = %d, want 409", resp.StatusCode)
	}
}

// TestReportRequiresAuth pins that the whole controller is authenticated.
func TestReportRequiresAuth(t *testing.T) {
	h := newHarness(t)

	imageID := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)
	reportID := h.createReport(t, 51, imageID, "private")

	for _, tc := range []struct{ method, path string }{
		{"GET", "/reports"},
		{"POST", "/reports"},
		{"PATCH", "/reports/" + itoa64(reportID)},
		{"PATCH", "/reports/" + itoa64(reportID) + "/validate"},
		{"PATCH", "/reports/" + itoa64(reportID) + "/reject"},
		{"PATCH", "/reports/" + itoa64(reportID) + "/cancel"},
	} {
		resp := h.request(tc.method, tc.path, "", nil, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s = %d, want 401", tc.method, tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}
