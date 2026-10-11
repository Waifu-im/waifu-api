package authz

import (
	"errors"
	"strings"
	"testing"

	"waifuapi/internal/config"
	"waifuapi/internal/store/models"
)

// These tests exist because PermissionService.cs and ReviewPolicy.cs are the
// two places where a transcription error silently changes who can do what. A
// blank configuration value in C# means "no restriction"; here a missing role
// check is meant to be inexpressible, and these tests are what keep it that
// way.

func allRoles() []config.Role {
	return []config.Role{
		config.RoleUser,
		config.RoleTrustedUser,
		config.RoleModerator,
		config.RoleAdmin,
	}
}

// permissiveConfig grants everything at the lowest role, so a failure points at
// the action under test rather than at a threshold someone else set.
func permissiveConfig() *config.Config {
	return &config.Config{
		Permissions: config.PermissionsConfig{
			ImageUploadMinRole:      config.RoleUser,
			TagCreationMinRole:      config.RoleUser,
			ArtistCreationMinRole:   config.RoleUser,
			SubmitEditMinRole:       config.RoleUser,
			BannerManagementMinRole: config.RoleUser,
		},
	}
}

// restrictiveConfig requires Admin for every configurable action.
func restrictiveConfig() *config.Config {
	return &config.Config{
		Permissions: config.PermissionsConfig{
			ImageUploadMinRole:      config.RoleAdmin,
			TagCreationMinRole:      config.RoleAdmin,
			ArtistCreationMinRole:   config.RoleAdmin,
			SubmitEditMinRole:       config.RoleAdmin,
			BannerManagementMinRole: config.RoleAdmin,
		},
	}
}

func TestEveryKnownActionIsAllowedWhenConfiguredPermissively(t *testing.T) {
	checker := New(permissiveConfig())

	actions := []Action{
		ActionUploadImage,
		ActionCreateTag,
		ActionCreateArtist,
		ActionSubmitEdit,
		ActionManageBanner,
	}
	for _, action := range actions {
		for _, role := range allRoles() {
			if !checker.Allow(action, role) {
				t.Errorf("Allow(%q, %s) = false, want true with a permissive configuration",
					action, role)
			}
		}
	}
}

// TestUnknownActionIsDenied is the guard against a newly added action being
// open until somebody notices. Defaulting to permissive here would be a
// security regression with no other symptom.
func TestUnknownActionIsDenied(t *testing.T) {
	checker := New(permissiveConfig())

	unknown := []Action{
		"",
		"delete everything",
		"upload images ", // trailing space, so not the same constant
		Action("UPLOAD IMAGES"),
		ActionUploadImage + "x",
	}
	for _, action := range unknown {
		if checker.Allow(action, config.RoleAdmin) {
			t.Errorf("Allow(%q, Admin) = true, want false: an unrecognised action must be denied",
				action)
		}
		if err := checker.Require(action, config.RoleAdmin); err == nil {
			t.Errorf("Require(%q, Admin) = nil, want a refusal", action)
		}
	}
}

// TestModerationIsNotConfigurable pins the one action that must ignore
// configuration. ReviewController.cs guards approve and reject with a hardcoded
// Moderator policy, so a Permissions__* setting must not be able to hand
// moderation to an ordinary user.
func TestModerationIsNotConfigurable(t *testing.T) {
	for _, role := range allRoles() {
		checker := New(permissiveConfig())
		want := role == config.RoleModerator || role == config.RoleAdmin
		if got := checker.Allow(ActionModerate, role); got != want {
			t.Errorf("Allow(moderate, %s) = %v, want %v even when Permissions are permissive",
				role, got, want)
		}
	}
}

// TestConfiguredMinimumsAreEnforced is the main behavioural table.
func TestConfiguredMinimumsAreEnforced(t *testing.T) {
	cases := []struct {
		action Action
		set    func(*config.Config, config.Role)
	}{
		{ActionUploadImage, func(c *config.Config, r config.Role) { c.Permissions.ImageUploadMinRole = r }},
		{ActionCreateTag, func(c *config.Config, r config.Role) { c.Permissions.TagCreationMinRole = r }},
		{ActionCreateArtist, func(c *config.Config, r config.Role) { c.Permissions.ArtistCreationMinRole = r }},
		{ActionSubmitEdit, func(c *config.Config, r config.Role) { c.Permissions.SubmitEditMinRole = r }},
		{ActionManageBanner, func(c *config.Config, r config.Role) { c.Permissions.BannerManagementMinRole = r }},
	}

	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			for _, minimum := range allRoles() {
				cfg := restrictiveConfig()
				tc.set(cfg, minimum)
				checker := New(cfg)

				for _, role := range allRoles() {
					want := role >= minimum
					if got := checker.Allow(tc.action, role); got != want {
						t.Errorf("Allow(%q, %s) with minimum %s = %v, want %v",
							tc.action, role, minimum, got, want)
					}

					err := checker.Require(tc.action, role)
					if want && err != nil {
						t.Errorf("Require(%q, %s) with minimum %s = %v, want nil",
							tc.action, role, minimum, err)
					}
					if !want {
						if err == nil {
							t.Errorf("Require(%q, %s) with minimum %s = nil, want a refusal",
								tc.action, role, minimum)
							continue
						}
						var forbidden *ErrForbidden
						if !errors.As(err, &forbidden) {
							t.Errorf("Require(%q, %s) returned %T, want *ErrForbidden",
								tc.action, role, err)
							continue
						}
						if forbidden.Action != tc.action {
							t.Errorf("ErrForbidden.Action = %q, want %q", forbidden.Action, tc.action)
						}
						if forbidden.RequiredRole != minimum {
							t.Errorf("ErrForbidden.RequiredRole = %s, want %s", forbidden.RequiredRole, minimum)
						}
					}
				}
			}
		})
	}
}

// TestRefusalMessageMatchesTheService guards the user-visible wording, which
// Error() documents as matching the C# response.
func TestRefusalMessageMatchesTheService(t *testing.T) {
	checker := New(restrictiveConfig())
	err := checker.Require(ActionUploadImage, config.RoleUser)
	if err == nil {
		t.Fatal("expected a refusal")
	}

	msg := err.Error()
	if !strings.Contains(msg, config.RoleAdmin.String()) {
		t.Errorf("message %q does not name the required role %q", msg, config.RoleAdmin)
	}
	for _, fragment := range []string{"requires the", "role or higher"} {
		if !strings.Contains(msg, fragment) {
			t.Errorf("message %q is missing %q", msg, fragment)
		}
	}
}

func TestRequireAtLeast(t *testing.T) {
	checker := New(permissiveConfig())

	cases := []struct {
		role, minimum config.Role
		want          bool
	}{
		{config.RoleAdmin, config.RoleModerator, true},
		{config.RoleModerator, config.RoleModerator, true},
		{config.RoleUser, config.RoleModerator, false},
		{config.RoleUser, config.RoleUser, true},
		{config.RoleUser, config.RoleTrustedUser, false},
	}

	for _, tc := range cases {
		err := checker.RequireAtLeast(tc.role, tc.minimum)
		if tc.want && err != nil {
			t.Errorf("RequireAtLeast(%s, %s) = %v, want nil", tc.role, tc.minimum, err)
		}
		if !tc.want {
			if err == nil {
				t.Errorf("RequireAtLeast(%s, %s) = nil, want a refusal", tc.role, tc.minimum)
				continue
			}
			var forbidden *ErrForbidden
			if !errors.As(err, &forbidden) {
				t.Errorf("RequireAtLeast(%s, %s) returned %T, want *ErrForbidden",
					tc.role, tc.minimum, err)
				continue
			}
			if forbidden.RequiredRole != tc.minimum {
				t.Errorf("RequiredRole = %s, want %s", forbidden.RequiredRole, tc.minimum)
			}
		}
	}
}

// reviewConfig builds a config with every review switch on and the supplied
// bypass thresholds.
func reviewConfig(bypass config.Role) *config.Config {
	return &config.Config{
		Review: config.ReviewConfig{
			RequireImageReview:  true,
			RequireTagReview:    true,
			RequireArtistReview: true,
			RequireEditReview:   true,
			ImageBypassMinRole:  bypass,
			TagBypassMinRole:    bypass,
			ArtistBypassMinRole: bypass,
			ImageEditBypassMin:  bypass,
			TagEditBypassMin:    bypass,
			ArtistEditBypassMin: bypass,
		},
	}
}

func TestReviewRequiredWhenMasterSwitchIsOn(t *testing.T) {
	// The default threshold is TrustedUser, matching the deployed settings.
	checker := New(reviewConfig(config.RoleTrustedUser))

	if checker.RequiresReviewForImage(config.RoleUser) != true {
		t.Error("an ordinary User's image must be queued for review")
	}
	if checker.RequiresReviewForTag(config.RoleUser) != true {
		t.Error("an ordinary User's tag must be queued for review")
	}
	if checker.RequiresReviewForArtist(config.RoleUser) != true {
		t.Error("an ordinary User's artist must be queued for review")
	}

	if checker.RequiresReviewForImage(config.RoleTrustedUser) != false {
		t.Error("a TrustedUser bypasses image review")
	}
	if checker.RequiresReviewForImage(config.RoleAdmin) != false {
		t.Error("an Admin bypasses image review")
	}
}

func TestNoReviewWhenMasterSwitchIsOff(t *testing.T) {
	cfg := reviewConfig(config.RoleTrustedUser)
	cfg.Review.RequireImageReview = false
	cfg.Review.RequireTagReview = false
	cfg.Review.RequireArtistReview = false
	cfg.Review.RequireEditReview = false
	checker := New(cfg)

	for _, role := range allRoles() {
		if checker.RequiresReviewForImage(role) {
			t.Errorf("image review required for %s despite the master switch being off", role)
		}
		if checker.RequiresReviewForTag(role) {
			t.Errorf("tag review required for %s despite the master switch being off", role)
		}
		if checker.RequiresReviewForArtist(role) {
			t.Errorf("artist review required for %s despite the master switch being off", role)
		}
		if checker.RequiresReviewForEdit(role, models.TargetImage) {
			t.Errorf("edit review required for %s despite the master switch being off", role)
		}
	}
}

// TestBypassNoneMeansNobodyBypasses covers the "None" sentinel. It reads as a
// plausible role name, so a wrong comparison here would quietly hand review
// bypass to an ordinary user.
func TestBypassNoneMeansNobodyBypasses(t *testing.T) {
	checker := New(reviewConfig(config.BypassNone))

	for _, role := range allRoles() {
		if !checker.RequiresReviewForImage(role) {
			t.Errorf("Allow(%s) bypassed review, but BypassNone must bypass for nobody", role)
		}
		if !checker.RequiresReviewForEdit(role, models.TargetImage) {
			t.Errorf("%s bypassed edit review, but BypassNone must bypass for nobody", role)
		}
	}
}

// TestEditBypassIsIndependentOfCreation is the distinction the package
// documents: being trusted to publish your own uploads does not make you
// trusted to rewrite someone else's.
func TestEditBypassIsIndependentOfCreation(t *testing.T) {
	cfg := reviewConfig(config.BypassNone)
	// Trusted users publish their own uploads straight away...
	cfg.Review.ImageBypassMinRole = config.RoleTrustedUser
	// ...but still need approval to edit existing content.
	cfg.Review.TagEditBypassMin = config.BypassNone
	cfg.Review.ArtistEditBypassMin = config.BypassNone
	cfg.Review.ImageEditBypassMin = config.BypassNone
	checker := New(cfg)

	const trusted = config.RoleTrustedUser
	if checker.RequiresReviewForImage(trusted) {
		t.Error("TrustedUser should publish their own image immediately")
	}
	if !checker.RequiresReviewForEdit(trusted, models.TargetTag) {
		t.Error("TrustedUser must not bypass tag edit review")
	}
	if !checker.RequiresReviewForEdit(trusted, models.TargetArtist) {
		t.Error("TrustedUser must not bypass artist edit review")
	}
}

func TestRequiresReviewForEditUsesTheTargetSpecificThreshold(t *testing.T) {
	cfg := reviewConfig(config.BypassNone)
	cfg.Review.ImageEditBypassMin = config.RoleModerator
	cfg.Review.TagEditBypassMin = config.RoleAdmin
	cfg.Review.ArtistEditBypassMin = config.BypassNone
	checker := New(cfg)

	const moderator = config.RoleModerator

	// A Moderator clears the image threshold but not the tag threshold.
	if checker.RequiresReviewForEdit(moderator, models.TargetImage) {
		t.Error("Moderator should bypass image edit review")
	}
	if !checker.RequiresReviewForEdit(moderator, models.TargetTag) {
		t.Error("Moderator must not bypass tag edit review, which needs Admin")
	}
	if !checker.RequiresReviewForEdit(moderator, models.TargetArtist) {
		t.Error("Moderator must not bypass artist edit review")
	}
}

// TestRoleOrderingIsTotal pins the ordering the >= comparisons rely on. If this
// ever changes, every Allow in the package silently changes meaning.
func TestRoleOrderingIsTotal(t *testing.T) {
	ordered := allRoles()
	for i := range ordered {
		for j := range ordered {
			got := ordered[i] >= ordered[j]
			want := i >= j
			if got != want {
				t.Errorf("%s >= %s = %v, want %v", ordered[i], ordered[j], got, want)
			}
		}
	}
}

func TestMinimumForUnknownActionDefaultsToModerator(t *testing.T) {
	// Only reachable through the error message, but it must not panic and must
	// not name a role that would mislead a reader into retrying as Guest.
	checker := New(permissiveConfig())
	err := checker.Require("something unrecognised", config.RoleUser)
	if err == nil {
		t.Fatal("expected a refusal")
	}
	var forbidden *ErrForbidden
	if !errors.As(err, &forbidden) {
		t.Fatalf("returned %T, want *ErrForbidden", err)
	}
	if forbidden.RequiredRole != config.RoleModerator {
		t.Errorf("RequiredRole = %s, want %s", forbidden.RequiredRole, config.RoleModerator)
	}
}
