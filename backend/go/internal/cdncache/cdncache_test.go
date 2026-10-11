package cdncache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// fakeLogger records warnings so tests can assert that a failure was surfaced
// rather than silently dropped.
type fakeLogger struct {
	mu       sync.Mutex
	warnings []string
}

func (f *fakeLogger) Warn(msg string, err error, _ ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err != nil {
		f.warnings = append(f.warnings, msg+": "+err.Error())
		return
	}
	f.warnings = append(f.warnings, msg)
}

func (f *fakeLogger) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.warnings)
}

// capture records the purge bodies a Purger sent.
type capture struct {
	mu      sync.Mutex
	batches [][]string
	auth    []string
}

func (c *capture) add(batch []string, auth string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.batches = append(c.batches, batch)
	c.auth = append(c.auth, auth)
}

func (c *capture) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, b := range c.batches {
		out = append(out, b...)
	}
	return out
}

// newCapturingPurger points a Purger at a test server and records what it sends.
func newCapturingPurger(t *testing.T, cfg Config, status int) (*Purger, *capture, *fakeLogger) {
	t.Helper()
	got := &capture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Files []string `json:"files"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode purge body: %v", err)
		}
		got.add(body.Files, r.Header.Get("Authorization"))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(server.Close)

	cfg.APIBaseURL = server.URL
	log := &fakeLogger{}
	return New(cfg, server.Client(), log), got, log
}

func baseConfig() Config {
	return Config{
		ZoneID:  "zone123",
		Token:   "secret-token",
		BaseURL: "https://cdn.example.com",
	}
}

func TestIsConfiguredRequiresAllThree(t *testing.T) {
	full := baseConfig()
	if !full.IsConfigured() {
		t.Error("a fully populated config should report configured")
	}

	cases := map[string]func(Config) Config{
		"no zone":    func(c Config) Config { c.ZoneID = ""; return c },
		"no token":   func(c Config) Config { c.Token = ""; return c },
		"no baseURL": func(c Config) Config { c.BaseURL = ""; return c },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			if mutate(full).IsConfigured() {
				t.Errorf("%s should report not configured", name)
			}
		})
	}
}

// TestUnconfiguredPurgerMakesNoRequest is the self-hosting case: a deployment
// with no Cloudflare credentials must not attempt a purge, and must not fail.
func TestUnconfiguredPurgerMakesNoRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := baseConfig()
	cfg.Token = ""
	cfg.APIBaseURL = server.URL
	purger := New(cfg, server.Client(), &fakeLogger{})

	purger.PurgeImage(context.Background(), 1, []string{".png"})
	purger.PurgeFiles(context.Background(), []string{"1.png"})

	if requests != 0 {
		t.Errorf("made %d requests while unconfigured, want 0", requests)
	}
}

func TestPurgeImageBuildsIDExtensionNames(t *testing.T) {
	cfg := baseConfig()
	purger := New(cfg, nil, &fakeLogger{})

	urls := purger.buildURLs([]string{"42.png"})
	if want := []string{"https://cdn.example.com/42.png"}; !reflect.DeepEqual(urls, want) {
		t.Errorf("buildURLs = %v, want %v", urls, want)
	}
}

func TestPurgeImageJoinsIDAndExtension(t *testing.T) {
	cfg := baseConfig()
	purger := New(cfg, nil, &fakeLogger{})

	// A replacement that changed extension produces two objects; both need
	// purging or the old file stays visible at the edge.
	purger, got, _ := newCapturingPurger(t, cfg, http.StatusOK)
	purger.PurgeImage(context.Background(), 8616, []string{".webp", ".png"})

	urls := got.all()
	if len(urls) != 2 {
		t.Fatalf("purged %v, want two URLs", urls)
	}
	for _, want := range []string{
		"https://cdn.example.com/8616.webp",
		"https://cdn.example.com/8616.png",
	} {
		if !contains(urls, want) {
			t.Errorf("missing %q in %v", want, urls)
		}
	}
}

func TestEmptyExtensionsAndNamesAreSkipped(t *testing.T) {
	cfg := baseConfig()
	purger := New(cfg, nil, &fakeLogger{})

	got := purger.buildURLs([]string{"", "1.png", ""})
	if want := []string{"https://cdn.example.com/1.png"}; !reflect.DeepEqual(got, want) {
		t.Errorf("buildURLs = %v, want %v", got, want)
	}

	if urls := purger.buildURLs([]string{"", ""}); len(urls) != 0 {
		t.Errorf("buildURLs on empty input = %v, want nothing", urls)
	}
}

// TestResizeSeparatorMatchesFrontend is the parity test that matters most.
//
// The purge is keyed on the exact cached URL, so the separator this produces
// must be byte-identical to what frontend/src/utils/cfImage.ts produces. A
// mismatch does not error; it silently leaves a stale image at the edge.
func TestResizeSeparatorMatchesFrontend(t *testing.T) {
	cases := []struct {
		name          string
		transformPath string
		want          string
	}{
		{
			name:          "plain path uses slash",
			transformPath: "/cdn-cgi/image",
			want:          "https://cdn.example.com/cdn-cgi/image/width=640/1.png",
		},
		{
			name:          "trailing slash needs no separator",
			transformPath: "/cdn-cgi/image/",
			want:          "https://cdn.example.com/cdn-cgi/image/width=640/1.png",
		},
		{
			name:          "path with options uses comma",
			transformPath: "/cdn-cgi/image/format=auto",
			want:          "https://cdn.example.com/cdn-cgi/image/format=auto,width=640/1.png",
		},
		{
			name:          "trailing comma needs no separator",
			transformPath: "/cdn-cgi/image/format=auto,anim=false",
			want:          "https://cdn.example.com/cdn-cgi/image/format=auto,anim=false,width=640/1.png",
		},
		{
			name:          "bare comma terminated path",
			transformPath: "/cdn-cgi/image,",
			want:          "https://cdn.example.com/cdn-cgi/image,width=640/1.png",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.TransformEnabled = true
			cfg.TransformPath = tc.transformPath
			cfg.CardWidth = 640
			purger := New(cfg, nil, &fakeLogger{})

			got := purger.buildURLs([]string{"1.png"})
			// The original is always first; the variant follows.
			if len(got) != 2 {
				t.Fatalf("buildURLs = %v, want the original plus one variant", got)
			}
			if got[1] != tc.want {
				t.Errorf("variant URL = %q, want %q", got[1], tc.want)
			}
		})
	}
}

func TestCustomWidthParamIsUsed(t *testing.T) {
	cfg := baseConfig()
	cfg.TransformEnabled = true
	cfg.TransformPath = "/cdn-cgi/image"
	cfg.WidthParam = "w"
	cfg.CardWidth = 300
	purger := New(cfg, nil, &fakeLogger{})

	got := purger.buildURLs([]string{"1.png"})
	if len(got) < 2 {
		t.Fatalf("buildURLs = %v", got)
	}
	if want := "https://cdn.example.com/cdn-cgi/image/w=300/1.png"; got[1] != want {
		t.Errorf("variant URL = %q, want %q", got[1], want)
	}
}

func TestEmptyWidthParamFallsBackToWidth(t *testing.T) {
	// The frontend defaults to "width" when unset; a purge built with an empty
	// parameter would not match the cached key.
	cfg := baseConfig()
	cfg.TransformEnabled = true
	cfg.TransformPath = "/cdn-cgi/image"
	cfg.WidthParam = ""
	cfg.CardWidth = 640
	purger := New(cfg, nil, &fakeLogger{})

	got := purger.buildURLs([]string{"1.png"})
	if len(got) < 2 {
		t.Fatalf("buildURLs = %v", got)
	}
	if !strings.Contains(got[1], "width=640") {
		t.Errorf("variant URL = %q, want it to use the default width parameter", got[1])
	}
}

// TestVariantsOnlyWhenTransformEnabled: without transforms those URLs were
// never cached, so purging them would be a wasted API call.
func TestVariantsOnlyWhenTransformEnabled(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		cfg := baseConfig()
		cfg.TransformEnabled = false
		cfg.TransformPath = "/cdn-cgi/image"
		cfg.CardWidth = 640
		purger := New(cfg, nil, &fakeLogger{})

		got := purger.buildURLs([]string{"1.png"})
		if want := []string{"https://cdn.example.com/1.png"}; !reflect.DeepEqual(got, want) {
			t.Errorf("buildURLs = %v, want %v", got, want)
		}
	})

	t.Run("enabled but no path", func(t *testing.T) {
		cfg := baseConfig()
		cfg.TransformEnabled = true
		cfg.TransformPath = ""
		cfg.CardWidth = 640
		purger := New(cfg, nil, &fakeLogger{})

		got := purger.buildURLs([]string{"1.png"})
		if want := []string{"https://cdn.example.com/1.png"}; !reflect.DeepEqual(got, want) {
			t.Errorf("buildURLs = %v, want %v", got, want)
		}
	})
}

func TestWidthsAreDedupedAndOrdered(t *testing.T) {
	cases := []struct {
		name          string
		card, preview int
		want          []int
	}{
		{"both distinct", 640, 1280, []int{640, 1280}},
		{"identical collapse", 640, 640, []int{640}},
		{"unset card skipped", 0, 1280, []int{1280}},
		{"unset preview skipped", 640, 0, []int{640}},
		{"both unset", 0, 0, nil},
		{"negative skipped", -5, -6, nil},
		{"negative mixed", -5, 900, []int{900}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseConfig()
			cfg.CardWidth = tc.card
			cfg.PreviewWidth = tc.preview
			purger := New(cfg, nil, &fakeLogger{})

			if got := purger.widths(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("widths() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDuplicateURLsAreCollapsed(t *testing.T) {
	cfg := baseConfig()
	cfg.TransformEnabled = true
	cfg.TransformPath = "/cdn-cgi/image"
	cfg.CardWidth = 640
	cfg.PreviewWidth = 640 // same as card, produces the same variant URL
	purger := New(cfg, nil, &fakeLogger{})

	// "1.png" twice must not produce four URLs.
	got := purger.buildURLs([]string{"1.png", "1.png"})
	if len(got) != 2 {
		t.Errorf("buildURLs = %v, want the original and one variant", got)
	}
}

func TestTrailingSlashOnBaseURLIsNormalised(t *testing.T) {
	cfg := baseConfig()
	cfg.BaseURL = "https://cdn.example.com/"
	purger := New(cfg, nil, &fakeLogger{})

	got := purger.buildURLs([]string{"1.png"})
	if len(got) != 1 || got[0] != "https://cdn.example.com/1.png" {
		t.Errorf("buildURLs = %v, want a single normalised URL", got)
	}
}

// TestBatchesRespectCloudflareLimit: Cloudflare accepts at most 30 files per
// call, and silently ignoring that returns a 400 for the whole batch.
func TestBatchesRespectCloudflareLimit(t *testing.T) {
	cfg := baseConfig()
	cfg.TransformEnabled = true
	cfg.TransformPath = "/cdn-cgi/image"
	cfg.CardWidth = 640
	cfg.PreviewWidth = 1280
	purger, got, _ := newCapturingPurger(t, cfg, http.StatusOK)

	names := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		names = append(names, itoa(i)+".png")
	}
	purger.PurgeFiles(context.Background(), names)

	if len(got.batches) < 2 {
		t.Fatalf("sent %d batches for 20 names x3 URLs; expected chunking", len(got.batches))
	}
	total := 0
	for _, batch := range got.batches {
		if len(batch) > maxURLsPerPurge {
			t.Errorf("batch of %d exceeds the %d limit", len(batch), maxURLsPerPurge)
		}
		total += len(batch)
	}
	if want := 60; total != want {
		t.Errorf("purged %d URLs in total, want %d", total, want)
	}
}

func TestSingleBatchForSmallInput(t *testing.T) {
	cfg := baseConfig()
	purger, got, _ := newCapturingPurger(t, cfg, http.StatusOK)

	purger.PurgeFiles(context.Background(), []string{"1.png", "2.png"})

	if len(got.batches) != 1 {
		t.Fatalf("sent %d batches, want 1", len(got.batches))
	}
	if len(got.batches[0]) != 2 {
		t.Errorf("batch = %v, want two URLs", got.batches[0])
	}
}

func TestAuthorizationHeaderIsSent(t *testing.T) {
	cfg := baseConfig()
	purger, got, _ := newCapturingPurger(t, cfg, http.StatusOK)

	purger.PurgeFiles(context.Background(), []string{"1.png"})

	if len(got.auth) != 1 || got.auth[0] != "Bearer secret-token" {
		t.Errorf("Authorization = %v, want Bearer secret-token", got.auth)
	}
}

// TestPurgeFailureIsLoggedNotFatal is deliberate: a failed purge leaves a stale
// image until the TTL expires, which is cosmetic. Failing the delete that
// triggered it would report a rollback that did not happen.
func TestPurgeFailureIsLoggedNotFatal(t *testing.T) {
	cases := map[string]int{
		"server error": http.StatusInternalServerError,
		"rate limited": http.StatusTooManyRequests,
		"unauthorized": http.StatusUnauthorized,
		"bad request":  http.StatusBadRequest,
	}

	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := baseConfig()
			purger, _, log := newCapturingPurger(t, cfg, status)

			// Must not panic and must return.
			purger.PurgeFiles(context.Background(), []string{"1.png"})

			if log.count() == 0 {
				t.Error("a rejected purge was not logged")
			}
		})
	}
}

func TestTransportFailureIsLoggedNotFatal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := server.Client()
	server.Close() // now unreachable

	cfg := baseConfig()
	cfg.APIBaseURL = server.URL + "/unreachable"
	log := &fakeLogger{}
	purger := New(cfg, client, log)
	purger.PurgeFiles(context.Background(), []string{"1.png"})

	if log.count() == 0 {
		t.Error("a transport failure was not logged")
	}
}

func TestNilPurgerIsSafe(t *testing.T) {
	// A nil receiver is reachable if the purger is wired up conditionally, and
	// panicking in a delete handler over a cosmetic cache would be absurd.
	var purger *Purger
	purger.PurgeFiles(context.Background(), []string{"1.png"})
	purger.PurgeImage(context.Background(), 1, []string{".png"})
}

func TestNilLoggerDoesNotPanic(t *testing.T) {
	cfg := baseConfig()
	purger, _, _ := newCapturingPurger(t, cfg, http.StatusInternalServerError)
	purger.log = nil

	// The rejected status must not panic through a nil logger.
	purger.PurgeFiles(context.Background(), []string{"1.png"})
}

func TestNewAppliesADefaultTimeout(t *testing.T) {
	purger := New(baseConfig(), nil, &fakeLogger{})
	if purger.client == nil {
		t.Fatal("New left the client nil")
	}
	if purger.client.Timeout == 0 {
		t.Error("default client has no timeout; a hung connection would tie up the caller")
	}
}

func TestBaseURLWithPathPrefix(t *testing.T) {
	cfg := baseConfig()
	cfg.BaseURL = "https://cdn.example.com/images"
	purger := New(cfg, nil, &fakeLogger{})

	got := purger.buildURLs([]string{"1.png"})
	if len(got) != 1 || got[0] != "https://cdn.example.com/images/1.png" {
		t.Errorf("buildURLs = %v, want the prefix preserved", got)
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
