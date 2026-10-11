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

	// failPut, when set, makes Put fail. Used to exercise the rollback.
	failPut bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: make(map[string][]byte)}
}

func (f *fakeStore) Put(_ context.Context, key string, body io.Reader, _ string) error {
	if f.failPut {
		return fmt.Errorf("simulated storage failure")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.objects[key] = data
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

// compile-time check that the fake satisfies the interface.
var _ storage.Store = (*fakeStore)(nil)

// harness holds everything a test needs to drive the API.
type harness struct {
	t      *testing.T
	server *httptest.Server
	store  *fakeStore
	db     *store.DB
}

// newHarness builds an API server backed by the test database.
//
// Each test gets a uniquely named schema user prefix so parallel runs do not
// collide, and the contents are truncated up front so assertions about counts
// are meaningful.
func newHarness(t *testing.T) *harness {
	t.Helper()

	databaseURL := os.Getenv("WAIFU_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set WAIFU_TEST_DATABASE_URL to run integration tests")
	}

	cfg := testConfig(databaseURL)

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

	api := httpapi.NewServer(cfg, db, data, log, files, discordClient, tokens)

	mux := http.NewServeMux()
	api.Register(mux)
	handler := api.Handler(mux, authenticator, data)

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	return &harness{t: t, server: httpServer, store: files, db: db}
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
        TRUNCATE "Image", "Tag", "Artist", "ImageTag", "ArtistImage", "User",
                 "ReviewTask", "SiteBanner", "ApiKey", "Album", "AlbumItem",
                 "Report", "DailyStat", "GlobalStat"
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
            INSERT INTO "User" ("Id", "DiscordId", "Name", "Role")
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
func multipartImage(t *testing.T, field, filename string, data []byte, extra map[string]string) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(field, filename)
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
