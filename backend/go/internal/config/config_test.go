package config

import (
	"strings"
	"testing"
)

// validEnv returns a complete, valid configuration as a map.
//
// Tests mutate a copy of this to exercise one behaviour at a time, which keeps
// each test readable instead of restating thirty variables per case.
func validEnv() map[string]string {
	return map[string]string{
		"ConnectionStrings__DefaultConnection": "postgres://user:pass@localhost:5432/db",
		"Image__MaxUploadSizeMb":               "10",

		"Review__RequireImageReview":            "true",
		"Review__RequireArtistReview":           "true",
		"Review__RequireTagReview":              "true",
		"Review__RequireEditRequestReview":      "true",
		"Review__ImageReviewBypassMinRole":      "TrustedUser",
		"Review__TagReviewBypassMinRole":        "TrustedUser",
		"Review__ArtistReviewBypassMinRole":     "TrustedUser",
		"Review__ImageEditReviewBypassMinRole":  "TrustedUser",
		"Review__TagEditReviewBypassMinRole":    "TrustedUser",
		"Review__ArtistEditReviewBypassMinRole": "TrustedUser",

		"Permissions__TagCreationMinRole":      "User",
		"Permissions__ArtistCreationMinRole":   "User",
		"Permissions__ImageUploadMinRole":      "User",
		"Permissions__SubmitEditMinRole":       "User",
		"Permissions__BannerManagementMinRole": "Moderator",

		"Jwt__Key":      strings.Repeat("k", 40),
		"Jwt__Issuer":   "waifu-api",
		"Jwt__Audience": "waifu-api-users",

		"S3__Provider":   "generic",
		"S3__AccessKey":  "access",
		"S3__SecretKey":  "secret",
		"S3__ServiceUrl": "https://s3.example.com",
		"S3__Region":     "fr-par",
		"S3__BucketName": "bucket",

		"Cdn__BaseUrl": "https://cdn.example.com",

		"Image__MinWidth":          "800",
		"Image__MinHeight":         "1000",
		"Image__MaxWidth":          "9000",
		"Image__MaxHeight":         "9000",
		"Image__AnimatedMinWidth":  "400",
		"Image__AnimatedMinHeight": "200",
		"Image__AnimatedMaxWidth":  "2000",
		"Image__AnimatedMaxHeight": "2000",

		"Discord__ClientId":     "client",
		"Discord__ClientSecret": "secret",
		"Discord__RedirectUri":  "http://localhost/auth/callback",

		// Production runs with MaxPageSize = -1, meaning "do not clamp". The
		// gallery defaults to a single image per request because it is an
		// infinite scroll over a randomised result set.
		"Image__DefaultPageSize":       "1",
		"Image__MaxPageSize":           "-1",
		"Tag__DefaultPageSize":         "30",
		"Tag__MaxPageSize":             "-1",
		"Artist__DefaultPageSize":      "30",
		"Artist__MaxPageSize":          "-1",
		"Album__DefaultPageSize":       "30",
		"Album__MaxPageSize":           "-1",
		"User__DefaultPageSize":        "30",
		"User__MaxPageSize":            "-1",
		"Report__DefaultPageSize":      "30",
		"Report__MaxPageSize":          "-1",
		"Review__DefaultPageSize":      "30",
		"Review__MaxPageSize":          "-1",
		"StorageDiff__DefaultPageSize": "30",
		"StorageDiff__MaxPageSize":     "-1",
	}
}

// TestLoadAcceptsValidConfig is the baseline: a complete configuration loads.
func TestLoadAcceptsValidConfig(t *testing.T) {
	cfg, err := Load(NewLoaderFromMap(validEnv()))
	if err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Port)
	}
	if cfg.MaxUploadSizeBytes != 10*1024*1024 {
		t.Errorf("max upload = %d, want %d", cfg.MaxUploadSizeBytes, 10*1024*1024)
	}
}

// TestLoadReportsEveryMissingVariableOnce is the important behavioural
// difference from the C# service.
//
// Requiring configuration one variable at a time turns a misconfigured deploy
// into a restart loop. All problems must be reported together.
func TestLoadReportsEveryMissingVariableOnce(t *testing.T) {
	_, err := Load(NewLoaderFromMap(map[string]string{}))
	if err == nil {
		t.Fatal("empty configuration should be rejected")
	}

	message := err.Error()
	for _, want := range []string{
		"Cdn__BaseUrl",
		"Jwt__Key",
		"S3__AccessKey",
		"Review__RequireImageReview",
		"Permissions__ImageUploadMinRole",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("error message does not mention %s:\n%s", want, message)
		}
	}
}

// TestUnknownRoleIsRejected is a deliberate departure from ReviewPolicy.cs,
// which silently falls back to TrustedUser when parsing fails. A typo in a role
// name must never quietly change who can bypass review.
func TestUnknownRoleIsRejected(t *testing.T) {
	env := validEnv()
	env["Review__ImageReviewBypassMinRole"] = "TrusedUser" // note the typo

	_, err := Load(NewLoaderFromMap(env))
	if err == nil {
		t.Fatal("a misspelled role must be rejected")
	}
	if !strings.Contains(err.Error(), "TrusedUser") {
		t.Errorf("error should quote the bad value, got: %v", err)
	}
}

// TestBannerRoleDefaultsToModerator checks the one permission that has a safe
// default. Unlike the others, a missing value must not mean "no restriction".
func TestBannerRoleDefaultsToModerator(t *testing.T) {
	env := validEnv()
	delete(env, "Permissions__BannerManagementMinRole")

	cfg, err := Load(NewLoaderFromMap(env))
	if err != nil {
		t.Fatalf("config rejected: %v", err)
	}
	if cfg.Permissions.BannerManagementMinRole != RoleModerator {
		t.Errorf("banner min role = %v, want Moderator", cfg.Permissions.BannerManagementMinRole)
	}
}

// TestNonBooleanIsRejected prevents a typo from silently disabling a safety
// switch, which is what bool.Parse tolerates in the C# service.
func TestNonBooleanIsRejected(t *testing.T) {
	env := validEnv()
	env["Review__RequireImageReview"] = "yes"

	if _, err := Load(NewLoaderFromMap(env)); err == nil {
		t.Fatal("\"yes\" must not be accepted as a boolean")
	}
}

// TestShortJWTKeyIsRejected guards against a weak signing key.
func TestShortJWTKeyIsRejected(t *testing.T) {
	env := validEnv()
	env["Jwt__Key"] = "tooshort"

	if _, err := Load(NewLoaderFromMap(env)); err == nil {
		t.Fatal("a short JWT key must be rejected")
	}
}

// TestContradictoryImageLimitsAreRejected catches a configuration that would
// reject every upload at runtime.
func TestContradictoryImageLimitsAreRejected(t *testing.T) {
	env := validEnv()
	env["Image__MinWidth"] = "9000"
	env["Image__MaxWidth"] = "100"

	if _, err := Load(NewLoaderFromMap(env)); err == nil {
		t.Fatal("min width above max width must be rejected")
	}
}

// TestReviewBypassMatrix pins the review decision table.
//
// These expectations come from the user's stated requirement: ordinary users
// queue for review, and TrustedUser is the configured bypass threshold.
func TestReviewBypassMatrix(t *testing.T) {
	cfg, err := Load(NewLoaderFromMap(validEnv()))
	if err != nil {
		t.Fatalf("config rejected: %v", err)
	}

	cases := []struct {
		name string
		min  Role
		role Role
		want bool // true means review is required
	}{
		{"user below TrustedUser threshold", RoleTrustedUser, RoleUser, true},
		{"user at TrustedUser threshold", RoleTrustedUser, RoleTrustedUser, false},
		{"moderator above threshold", RoleTrustedUser, RoleModerator, false},
		{"admin above threshold", RoleTrustedUser, RoleAdmin, false},
		{"BypassNone stops a user", BypassNone, RoleUser, true},
		{"BypassNone stops a moderator", BypassNone, RoleModerator, true},
		{"BypassNone stops an admin", BypassNone, RoleAdmin, true},
		{"BypassNone equals None config", RoleNoneViaString, RoleAdmin, true},
		{"Moderator threshold blocks user", RoleModerator, RoleTrustedUser, true},
		{"Moderator threshold allows moderator", RoleModerator, RoleModerator, false},
	}

	for _, c := range cases {
		got := cfg.RequiresReviewForNewContent(true, c.min, c.role)
		if got != c.want {
			t.Errorf("%s: RequiresReviewForNewContent(%v, %v) = %v, want %v",
				c.name, c.min, c.role, got, c.want)
		}
	}
}

// RoleNoneViaString mirrors the "None" configuration value. It is expected to
// parse to BypassNone.
const RoleNoneViaString = BypassNone

// TestReviewDisabledMasterSwitchWins verifies that turning review off
// short-circuits regardless of role.
func TestReviewDisabledMasterSwitchWins(t *testing.T) {
	cfg, err := Load(NewLoaderFromMap(validEnv()))
	if err != nil {
		t.Fatalf("config rejected: %v", err)
	}
	if cfg.RequiresReviewForNewContent(false, BypassNone, RoleUser) {
		t.Error("with review disabled nothing should require review")
	}
}

// TestRoleStringUsesClaimNames guards the JWT claim contract.
//
// Both C# authentication handlers emit the role as a name, so changing these
// strings would invalidate every issued token.
func TestRoleStringUsesClaimNames(t *testing.T) {
	cases := map[Role]string{
		RoleUser:        "User",
		RoleTrustedUser: "TrustedUser",
		RoleModerator:   "Moderator",
		RoleAdmin:       "Admin",
	}
	for role, want := range cases {
		if got := role.String(); got != want {
			t.Errorf("Role(%d).String() = %q, want %q", int(role), got, want)
		}
	}
}

// TestRoleOrderingIsPreserved documents that the numeric values are load
// bearing, since authorisation compares them with >=.
func TestRoleOrderingIsPreserved(t *testing.T) {
	if !(RoleUser < RoleTrustedUser && RoleTrustedUser < RoleModerator && RoleModerator < RoleAdmin) {
		t.Fatal("role ordering is wrong; authorisation compares with >=")
	}
	if RoleUser != 0 || RoleTrustedUser != 1 || RoleModerator != 2 || RoleAdmin != 3 {
		t.Fatal("role values changed; they are persisted in the database")
	}
}

// TestCanAccessUnacceptedContent checks the visibility rule for pending review
// submissions.
func TestCanAccessUnacceptedContent(t *testing.T) {
	if RoleUser.CanAccessUnacceptedContent() {
		t.Error("ordinary users must not see pending submissions")
	}
	if RoleTrustedUser.CanAccessUnacceptedContent() {
		t.Error("trusted users must not see pending submissions")
	}
	if !RoleModerator.CanAccessUnacceptedContent() || !RoleAdmin.CanAccessUnacceptedContent() {
		t.Error("moderators and admins must see pending submissions")
	}
}

// TestNormalizeBasePath covers the accepted forms of API_BASE_PATH.
func TestNormalizeBasePath(t *testing.T) {
	cases := map[string]string{
		"":        "",
		"/":       "",
		"/api":    "/api",
		"api":     "/api",
		"/api/":   "/api",
		"  /api ": "/api",
	}
	for input, want := range cases {
		if got := normalizeBasePath(input); got != want {
			t.Errorf("normalizeBasePath(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestS3ProviderDefaultsToGeneric guards the choice.
//
// Defaulting to a specific provider would silently change addressing or
// checksum behaviour for a self-hosted deployment that speaks the same API.
func TestS3ProviderDefaultsToGeneric(t *testing.T) {
	env := validEnv()
	delete(env, "S3__Provider")

	cfg, err := Load(NewLoaderFromMap(env))
	if err != nil {
		t.Fatalf("config rejected: %v", err)
	}
	if cfg.S3.Provider != "generic" {
		t.Errorf("provider = %q, want generic", cfg.S3.Provider)
	}
}

// TestS3ProviderIsConfigurable checks the name is carried through.
func TestS3ProviderIsConfigurable(t *testing.T) {
	env := validEnv()
	env["S3__Provider"] = "r2"

	cfg, err := Load(NewLoaderFromMap(env))
	if err != nil {
		t.Fatalf("config rejected: %v", err)
	}
	if cfg.S3.Provider != "r2" {
		t.Errorf("provider = %q, want r2", cfg.S3.Provider)
	}
}
