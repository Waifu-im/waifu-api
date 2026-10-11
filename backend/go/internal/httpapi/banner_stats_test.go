package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// Banner, public stats and admin stats.
//
// These three share a theme: the read side is public and must stay cheap and
// unconditional, while the write side is moderator-gated. The easy mistake is
// letting the gate leak onto the read endpoint, because then the site has no
// banner and no counter until someone logs in.

// TestGetBannerNoContentIs204 covers the empty case.
//
// "No banner" is 204, not a 200 with a null body: the frontend distinguishes
// them, and a 200 with null makes it render an empty bar.
func TestGetBannerNoContentIs204(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/banner", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("no banner = %d, want 204", resp.StatusCode)
	}
}

// TestBannerRoundTrip covers PUT /banner/settings and its effect on GET /banner.
func TestBannerRoundTrip(t *testing.T) {
	h := newHarness(t)

	moderator := h.issueToken(61, config.RoleModerator)

	resp := h.request("PUT", "/banner/settings", moderator,
		bytesReader(`{"message":"Scheduled maintenance","variant":"warning","isEnabled":true}`),
		"application/json")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update banner = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "message"); got != "Scheduled maintenance" {
		t.Errorf("message = %q", got)
	}
	// The response echoes the enum member name verbatim: JsonStringEnumConverter
	// has no naming policy set, so it does not camel-case what it writes back.
	if got := jsonString(t, body, "variant"); got != "Warning" {
		t.Errorf("variant = %q, want Warning", got)
	}

	// The public read now returns it.
	resp = h.request("GET", "/banner", "", nil, "")
	body = readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get banner = %d: %s", resp.StatusCode, body)
	}
	if got := jsonString(t, body, "message"); got != "Scheduled maintenance" {
		t.Errorf("public banner message = %q, want the configured one", got)
	}

	// Disabling it takes it back off the public endpoint, while the settings
	// endpoint still reports it.
	resp = h.request("PUT", "/banner/settings", moderator,
		bytesReader(`{"message":"Scheduled maintenance","variant":"warning","isEnabled":false}`),
		"application/json")
	if status, respBody := resp.StatusCode, readAll(t, resp); status != http.StatusOK {
		t.Fatalf("disable banner = %d: %s", status, respBody)
	}

	resp = h.request("GET", "/banner", "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("disabled banner = %d, want 204", resp.StatusCode)
	}

	resp = h.request("GET", "/banner/settings", moderator, nil, "")
	body = readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("settings after disable = %d: %s", resp.StatusCode, body)
	}
	// The banner row survives being disabled: the settings endpoint describes the
	// configuration, not only what is live.
	if !strings.Contains(string(body), `"isEnabled":false`) {
		t.Errorf("settings should still report the disabled banner: %s", body)
	}
}

// TestBannerValidation covers the validator rules.
func TestBannerValidation(t *testing.T) {
	h := newHarness(t)

	moderator := h.issueToken(61, config.RoleModerator)

	cases := []struct {
		body string
		why  string
	}{
		{`{"message":"","variant":"info","isEnabled":true}`, "an enabled banner needs a message"},
		{`{"message":"   ","variant":"info","isEnabled":true}`, "whitespace is not a message"},
		{`{"variant":"info","isEnabled":true}`, "message is required"},
		{`{"message":"` + strings.Repeat("m", 501) + `","variant":"info","isEnabled":true}`, "over-long"},
		{`{"message":"ok","variant":"not-a-variant","isEnabled":false}`, "invalid variant"},
	}

	for _, c := range cases {
		resp := h.request("PUT", "/banner/settings", moderator, bytesReader(c.body), "application/json")
		status, body := resp.StatusCode, readAll(t, resp)
		if status != http.StatusBadRequest {
			t.Errorf("%s (%s) = %d, want 400: %s", c.why, c.body, status, body)
		}
	}
}

// TestBannerPermissions covers the moderator gate.
func TestBannerPermissions(t *testing.T) {
	h := newHarness(t)

	body := `{"message":"hello","variant":"info","isEnabled":true}`

	// Reading the public banner needs nothing at all.
	resp := h.request("GET", "/banner", "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("anonymous GET /banner = %d, want 204", resp.StatusCode)
	}

	// But the settings endpoints are gated.
	for _, path := range []string{"/banner/settings"} {
		resp = h.request("GET", path, "", nil, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("anonymous GET %s = %d, want 401", path, resp.StatusCode)
		}
		resp.Body.Close()

		resp = h.request("GET", path, h.issueToken(51, config.RoleUser), nil, "")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("User GET %s = %d, want 403", path, resp.StatusCode)
		}
		resp.Body.Close()

		resp = h.request("PUT", path, h.issueToken(51, config.RoleUser), bytesReader(body), "application/json")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("User PUT %s = %d, want 403", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

// TestPublicStats covers GET /stats/public, which has no auth at all.
func TestPublicStats(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "counted")
	ensureArtist(t, h, "Counted")
	seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	resp := h.request("GET", "/stats/public", "", nil, "")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public stats = %d: %s", resp.StatusCode, body)
	}

	var stats struct {
		TotalRequests *int64 `json:"totalRequests"`
		TotalImages   *int64 `json:"totalImages"`
		TotalTags     *int64 `json:"totalTags"`
		TotalArtists  *int64 `json:"totalArtists"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("decode stats: %v (%s)", err, body)
	}

	// Nulls would make the frontend render "null" instead of a number.
	for name, value := range map[string]*int64{
		"totalRequests": stats.TotalRequests,
		"totalImages":   stats.TotalImages,
		"totalTags":     stats.TotalTags,
		"totalArtists":  stats.TotalArtists,
	} {
		if value == nil {
			t.Errorf("%s is null, want a number: %s", name, body)
		}
	}

	if stats.TotalTags == nil || *stats.TotalTags < 1 {
		t.Errorf("totalTags = %v, want at least the seeded tag", stats.TotalTags)
	}
	if stats.TotalArtists == nil || *stats.TotalArtists < 1 {
		t.Errorf("totalArtists = %v, want at least the seeded artist", stats.TotalArtists)
	}
}

// TestAdminStatsIsAdminOnly covers the gate on GET /stats/admin.
func TestAdminStatsIsAdminOnly(t *testing.T) {
	h := newHarness(t)

	resp := h.request("GET", "/stats/admin", "", nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous admin stats = %d, want 401", resp.StatusCode)
	}

	// A moderator is still not an admin here; this endpoint names Role.Admin
	// explicitly rather than using the moderator policy.
	resp = h.request("GET", "/stats/admin", h.issueToken(61, config.RoleModerator), nil, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("moderator admin stats = %d, want 403", resp.StatusCode)
	}

	resp = h.request("GET", "/stats/admin", h.issueToken(71, config.RoleAdmin), nil, "")
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin stats = %d: %s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "totalImages") {
		t.Errorf("admin stats should embed the public figures: %s", body)
	}
}

// TestRequestCounting covers the middleware that feeds /stats.
//
// The counter is written asynchronously, so the assertion polls rather than
// reading immediately: a test that slept a fixed duration would be either slow
// or flaky, and the flakiness would eventually be "fixed" by lengthening the
// sleep until the assertion stopped testing anything.
func TestRequestCounting(t *testing.T) {
	h := newHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Warm up so that any one-off first-request effects are behind us.
	h.request("GET", "/stats/public", "", nil, "").Body.Close()

	var before int64
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT COALESCE(SUM("RequestCount"), 0) FROM "DailyStats"`).Scan(&before); err != nil {
		t.Fatalf("read request stats: %v", err)
	}

	for i := 0; i < 3; i++ {
		resp := h.request("GET", "/tags", "", nil, "")
		resp.Body.Close()
	}

	after := before
	for attempt := 0; attempt < 50 && after <= before; attempt++ {
		time.Sleep(100 * time.Millisecond)
		if err := h.db.Pool().QueryRow(ctx,
			`SELECT COALESCE(SUM("RequestCount"), 0) FROM "DailyStats"`).Scan(&after); err != nil {
			t.Fatalf("read request stats: %v", err)
		}
	}

	if after <= before {
		t.Errorf("request counter did not advance (was %d, still %d)", before, after)
	}
}
