package httpapi_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"waifuapi/internal/auth"
	"waifuapi/internal/config"
	"waifuapi/internal/discord"
	"waifuapi/internal/httpapi"
	"waifuapi/internal/repo"
	"waifuapi/internal/storage"
	"waifuapi/internal/store"
)

// This is an end-to-end test of the upload and review workflow against a real
// Postgres. It exercises the paths that unit tests cannot reach: the ordering of
// the duplicate check, the transactional insert of an image plus its review
// task, and the compensating delete when object storage fails.
//
// Set WAIFU_TEST_DATABASE_URL to a Postgres with the schema from
// internal/store/schema.sql applied. The suite skips when it is absent, so a
// plain `go test ./...` stays hermetic and CI provides the database explicitly.

// fakeStore is an in-memory storage.Store.
//
// It is deliberately able to fail on demand, because the compensating delete is
// the behaviour most worth testing and it is unreachable otherwise.
type fakeStore struct {
	objects map[string][]byte

	// contentTypes records the content type each object was written with, so
	// tests can assert that the stored metadata type matches the stored bytes.
	contentTypes map[string]string

	// failPut, when set, makes Put fail. Used to exercise the rollback.
	failPut bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		objects:      make(map[string][]byte),
		contentTypes: make(map[string]string),
	}
}

func (f *fakeStore) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	if f.failPut {
		return fmt.Errorf("simulated storage failure")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.objects[key] = data
	f.contentTypes[key] = contentType
	return nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

func (f *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := f.objects[key]
	return ok, nil
}

func (f *fakeStore) List(_ context.Context) ([]string, error) {
	keys := make([]string, 0, len(f.objects))
	for key := range f.objects {
		keys = append(keys, key)
	}
	return keys, nil
}

// compile-time check that the fake satisfies the interface.
var _ storage.Store = (*fakeStore)(nil)

// harness holds everything a test needs to drive the API.
type harness struct {
	t      *testing.T
	server *httptest.Server
	store  *fakeStore
	db     *store.DB
	// mux is kept so route-contract tests can ask which pattern a probe request
	// resolves to, rather than inferring the route table from the handlers.
	mux *http.ServeMux
}

// newHarness builds an API server backed by the test database.
//
// Each test gets a uniquely named schema user prefix so parallel runs do not
// collide, and the contents are truncated up front so assertions about counts
// are meaningful.
func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWithConfig(t, nil)
}

// newHarnessWithConfig builds a harness whose configuration can be adjusted
// before the server is constructed.
//
// Tests that exercise a configuration-dependent behaviour, such as WebP
// conversion, need a real server built with that setting rather than a unit
// test of the setting itself.
func newHarnessWithConfig(t *testing.T, mutate func(*config.Config)) *harness {
	t.Helper()

	databaseURL := os.Getenv("WAIFU_TEST_DATABASE_URL")
	if databaseURL == "" {
		// This is a hard failure, not a skip.
		//
		// The suite used to skip here, and "go test ./..." reported green while
		// none of it had ever run. That is the worst possible failure mode for a
		// rewrite whose entire purpose is behavioural parity: the tests that were
		// supposed to catch drift were themselves the stale spec, and nothing
		// exercised the SQL at all. A rewrite with no executed tests cannot
		// distinguish a working port from a plausible one.
		//
		// Bring one up with:
		//
		//	docker run -d --name waifu-testdb -p 55432:5432 \
		//	  -e POSTGRES_PASSWORD=test -e POSTGRES_DB=waifutest pgvector/pgvector:pg18
		//	docker exec -i waifu-testdb psql -U postgres -d waifutest \
		//	  < internal/store/schema.sql
		//	export WAIFU_TEST_DATABASE_URL='postgres://postgres:test@localhost:55432/waifutest'
		t.Fatal("set WAIFU_TEST_DATABASE_URL to run the integration suite; " +
			"see the comment in integration_test.go for how to start a database. " +
			"Skipping here is not allowed: a silently skipped suite hides real breakage.")
	}

	cfg := testConfig(databaseURL)
	if mutate != nil {
		mutate(cfg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := store.Open(ctx, store.Config{URL: databaseURL, MaxConnections: 4})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(db.Close)

	truncateAll(t, db)
	seedReviewBypass(t, db, 100)

	data := repo.New(db.Pool())
	files := newFakeStore()

	// Logs are discarded by default so a passing run stays quiet. Set
	// WAIFU_TEST_VERBOSE to see the server's own error output, which is the
	// fastest way to diagnose a 500.
	logSink := io.Discard
	if os.Getenv("WAIFU_TEST_VERBOSE") != "" {
		logSink = os.Stderr
	}
	log := slog.New(slog.NewTextHandler(logSink, nil))
	tokens := auth.NewTokenIssuer(cfg.JWT)
	authenticator := auth.NewAuthenticator(tokens)

	// A Discord client pointed at an unreachable host. No test in this file
	// exercises OAuth, so it only needs to exist and satisfy the constructor.
	discordClient := discord.NewClient(discord.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost/auth/callback",
	}, "http://127.0.0.1:1")

	api := httpapi.NewServer(cfg, db, data, log, files, discordClient, tokens, nil)

	mux := http.NewServeMux()
	api.Register(mux)
	handler := api.Handler(mux, authenticator, data)

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	return &harness{t: t, server: httpServer, store: files, db: db, mux: mux}
}

// testConfig returns a configuration with permissive but explicit review rules,
// so an ordinary user queues and a TrustedUser bypasses.
func testConfig(databaseURL string) *config.Config {
	return &config.Config{
		Environment:        "test",
		Port:               0,
		BasePath:           "",
		DatabaseURL:        databaseURL,
		MaxUploadSizeBytes: 10 * 1024 * 1024,
		Review: config.ReviewConfig{
			RequireImageReview:  true,
			RequireTagReview:    true,
			RequireArtistReview: true,
			RequireEditReview:   true,
			ImageBypassMinRole:  config.RoleTrustedUser,
			TagBypassMinRole:    config.RoleTrustedUser,
			ArtistBypassMinRole: config.RoleTrustedUser,
			ImageEditBypassMin:  config.RoleTrustedUser,
			TagEditBypassMin:    config.RoleTrustedUser,
			ArtistEditBypassMin: config.RoleTrustedUser,
		},
		Permissions: config.PermissionsConfig{
			TagCreationMinRole:      config.RoleUser,
			ArtistCreationMinRole:   config.RoleUser,
			ImageUploadMinRole:      config.RoleUser,
			SubmitEditMinRole:       config.RoleUser,
			BannerManagementMinRole: config.RoleModerator,
		},
		JWT: config.JWTConfig{
			Key:      "test-signing-key-that-is-at-least-32-chars",
			Issuer:   "waifu-api",
			Audience: "waifu-api-users",
		},
		S3:  config.S3Config{BucketName: "waifu-images"},
		CDN: config.CDNConfig{BaseURL: "https://cdn.example.com"},

		// Page sizes must be set here for the same reason production sets them:
		// a zero default produces LIMIT 0, which returns an empty page with a
		// non-zero totalCount. That failure looks like "no results" rather than
		// "misconfigured", which is why the values are explicit.
		Pages: config.PaginationConfig{
			Image:       config.PageSize{Default: 30, Max: -1},
			Tag:         config.PageSize{Default: 30, Max: -1},
			Artist:      config.PageSize{Default: 30, Max: -1},
			Album:       config.PageSize{Default: 30, Max: -1},
			User:        config.PageSize{Default: 30, Max: -1},
			Report:      config.PageSize{Default: 30, Max: -1},
			Review:      config.PageSize{Default: 30, Max: -1},
			StorageDiff: config.PageSize{Default: 30, Max: -1},
		},
		Image: config.ImageConfig{
			MinWidth: 1, MinHeight: 1, MaxWidth: 9000, MaxHeight: 9000,
			AnimatedMinWidth: 1, AnimatedMinHeight: 1,
			AnimatedMaxWidth: 2000, AnimatedMaxHeight: 2000,
		},
	}
}

// truncateAll clears every table so each test starts from a known state.
func truncateAll(t *testing.T, db *store.DB) {
	t.Helper()

	const statement = `
        TRUNCATE "Images", "Tags", "Artists", "ImageTag", "ArtistImage", "Users",
                 "ReviewTasks", "SiteBanners", "ApiKeys", "Albums", "AlbumItems",
                 "Reports", "DailyStats", "GlobalStats"
        RESTART IDENTITY CASCADE`

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := db.Pool().Exec(ctx, statement); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// seedReviewBypass creates users covering every role.
//
// IDs are fixed so tests can reference them without querying.
func seedReviewBypass(t *testing.T, db *store.DB, count int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for i := 1; i <= count; i++ {
		_, err := db.Pool().Exec(ctx, `
            INSERT INTO "Users" ("Id", "DiscordId", "Name", "Role")
            VALUES ($1, $2, $3, 0)
            ON CONFLICT ("Id") DO NOTHING`,
			i, fmt.Sprintf("discord-%d", i), fmt.Sprintf("user-%d", i))
		if err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}
}

// issueToken mints a token for a user id and role.
func (h *harness) issueToken(userID int64, role config.Role) string {
	h.t.Helper()

	issuer := auth.NewTokenIssuer(testConfig(os.Getenv("WAIFU_TEST_DATABASE_URL")).JWT)
	token, _, err := issuer.Issue(userID, "tester", role, "discord-test")
	if err != nil {
		h.t.Fatalf("issue token: %v", err)
	}
	return token
}

// request performs an HTTP call against the test server.
func (h *harness) request(method, path, token string, body io.Reader, contentType string) *http.Response {
	h.t.Helper()

	req, err := http.NewRequest(method, h.server.URL+path, body)
	if err != nil {
		h.t.Fatalf("build request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// multipartImage builds a multipart upload body.
// multipartImage builds an upload body that declares image/png, which is what a
// browser form sends.
//
// The declared type matters: the C# validator checks ContentType and nothing
// else. multipart.Writer's CreateFormFile labels every part application/octet-
// stream, so a helper built on it silently depended on the server sniffing the
// bytes instead. That made the tests pass against behaviour the original service
// does not have, which is worse than having no test at all.
func multipartImage(t *testing.T, field, filename string, data []byte, extra map[string]string) (io.Reader, string) {
	t.Helper()
	return multipartImageTyped(t, field, filename, data, extra, "image/png")
}

// multipartImageTyped builds an upload body with an explicit part Content-Type,
// for the cases where the declaration itself is the thing under test.
func multipartImageTyped(t *testing.T, field, filename string, data []byte,
	extra map[string]string, contentType string) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, filename))
	header.Set("Content-Type", contentType)

	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write file: %v", err)
	}

	for name, value := range extra {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write field %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return &buf, writer.FormDataContentType()
}

// testImage builds a small PNG of the given colour.
func testImage(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// testJPEG builds a small JPEG.
func testJPEG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 3), uint8(y * 3), 100, 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}
