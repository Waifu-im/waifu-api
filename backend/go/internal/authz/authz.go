// Package authz turns configuration into authorisation decisions.
//
// # The bug this avoids
//
// PermissionService.cs treats a blank or unparseable configuration value as
// "no restriction", so an unset variable silently opens an endpoint instead of
// closing it. That is why this package never reads a raw string: it takes a
// config.Role that has already been validated at startup, and a missing role
// check is impossible to express here.
package authz

import (
	"fmt"

	"waifuapi/internal/config"
)

// Action names a privileged operation. It is a distinct type so a permission
// check cannot be given a free-form string and silently miss.
type Action string

// The guarded actions.
const (
	// ActionUploadImage permits POST /images/upload.
	ActionUploadImage Action = "upload images"
	// ActionCreateTag permits POST /tags.
	ActionCreateTag Action = "create tags"
	// ActionCreateArtist permits POST /artists.
	ActionCreateArtist Action = "create artists"
	// ActionSubmitEdit permits POST /review/tasks.
	ActionSubmitEdit Action = "submit edits"
	// ActionManageBanner permits reading and writing banner settings.
	ActionManageBanner Action = "manage the site banner"
	// ActionModerate permits approving and rejecting review tasks.
	ActionModerate Action = "moderate content"
)

// ErrForbidden is returned when the caller lacks the required role.
type ErrForbidden struct {
	// Action is the operation that was refused.
	Action Action
	// RequiredRole is the minimum role that would have been accepted.
	RequiredRole config.Role
}

// Error renders the refusal.
//
// The wording matches the C# service so existing users see the same message.
func (e *ErrForbidden) Error() string {
	return fmt.Sprintf("This action requires the %s role or higher. "+
		"This restriction is configured by the API administrator.", e.RequiredRole)
}

// Checker answers authorisation questions from validated configuration.
type Checker struct {
	cfg *config.Config
}

// New builds a Checker.
func New(cfg *config.Config) *Checker {
	return &Checker{cfg: cfg}
}

// Allow reports whether a role satisfies the minimum for an action.
//
// An unknown action is denied rather than allowed. Defaulting to permissive
// would mean a newly added action is open until someone notices.
func (c *Checker) Allow(action Action, role config.Role) bool {
	switch action {
	case ActionUploadImage:
		return c.cfg.HasPermission(c.cfg.Permissions.ImageUploadMinRole, role)
	case ActionCreateTag:
		return c.cfg.HasPermission(c.cfg.Permissions.TagCreationMinRole, role)
	case ActionCreateArtist:
		return c.cfg.HasPermission(c.cfg.Permissions.ArtistCreationMinRole, role)
	case ActionSubmitEdit:
		return c.cfg.HasPermission(c.cfg.Permissions.SubmitEditMinRole, role)
	case ActionManageBanner:
		return c.cfg.HasPermission(c.cfg.Permissions.BannerManagementMinRole, role)
	case ActionModerate:
		// Moderation is not configurable, matching ReviewController.cs where
		// the approve and reject endpoints carry a hardcoded Moderator policy.
		return role == config.RoleModerator || role == config.RoleAdmin
	default:
		return false
	}
}

// Require returns an error when the role is not permitted.
//
// Callers use this at the top of a handler so the refusal happens before any
// parsing or database work.
func (c *Checker) Require(action Action, role config.Role) error {
	if c.Allow(action, role) {
		return nil
	}
	return &ErrForbidden{Action: action, RequiredRole: c.minimumFor(action)}
}

// minimumFor reports the configured minimum, for the error message.
func (c *Checker) minimumFor(action Action) config.Role {
	switch action {
	case ActionUploadImage:
		return c.cfg.Permissions.ImageUploadMinRole
	case ActionCreateTag:
		return c.cfg.Permissions.TagCreationMinRole
	case ActionCreateArtist:
		return c.cfg.Permissions.ArtistCreationMinRole
	case ActionSubmitEdit:
		return c.cfg.Permissions.SubmitEditMinRole
	case ActionManageBanner:
		return c.cfg.Permissions.BannerManagementMinRole
	default:
		return config.RoleModerator
	}
}

// RequiresReviewForImage decides whether a new image needs approval.
//
// The role comparison mirrors ReviewPolicy.RequiresReviewForNewContent exactly,
// including the "None" sentinel meaning nobody bypasses.
func (c *Checker) RequiresReviewForImage(role config.Role) bool {
	return c.cfg.RequiresReviewForNewContent(
		c.cfg.Review.RequireImageReview, c.cfg.Review.ImageBypassMinRole, role)
}

// RequiresReviewForTag decides whether a new tag needs approval.
func (c *Checker) RequiresReviewForTag(role config.Role) bool {
	return c.cfg.RequiresReviewForNewContent(
		c.cfg.Review.RequireTagReview, c.cfg.Review.TagBypassMinRole, role)
}

// RequiresReviewForArtist decides whether a new artist needs approval.
func (c *Checker) RequiresReviewForArtist(role config.Role) bool {
	return c.cfg.RequiresReviewForNewContent(
		c.cfg.Review.RequireArtistReview, c.cfg.Review.ArtistBypassMinRole, role)
}
