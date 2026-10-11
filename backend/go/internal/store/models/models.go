// Package models contains the Go representation of the database rows.
//
// Column names are PascalCase with quoted identifiers, exactly as the C# service
// stores them. They are quoted in every query rather than folded to snake_case,
// because the schema is FROZEN (PLAN.md section 2) and renaming a column would
// be a breaking change to production data.
//
// Enums are mirrored as plain integers with named constants, because that is
// how they are persisted. The C# service compares roles with >= and review
// statuses by equality, so the numeric values are load bearing.
package models

import (
	"fmt"
	"time"
)

// ReviewStatus is the moderation state of a content row.
//
// There is deliberately no Rejected value. Rejecting a submission deletes the
// row rather than flagging it, which is what ApproveReviewTask and
// RejectReviewTask do in the C# service.
type ReviewStatus int

// Review status values.
const (
	// ReviewPending means the content exists but is not publicly visible.
	ReviewPending ReviewStatus = 0
	// ReviewAccepted means the content is publicly visible.
	ReviewAccepted ReviewStatus = 1
)

// String renders the status for logs.
func (r ReviewStatus) String() string {
	switch r {
	case ReviewPending:
		return "Pending"
	case ReviewAccepted:
		return "Accepted"
	default:
		return fmt.Sprintf("ReviewStatus(%d)", int(r))
	}
}

// ReviewTaskStatus is the state of a moderation task.
type ReviewTaskStatus int

// Review task status values.
const (
	// TaskPending means the task is awaiting a decision.
	TaskPending ReviewTaskStatus = 0
	// TaskApproved means the task was approved and applied.
	TaskApproved ReviewTaskStatus = 1
	// TaskRejected means the task was rejected; the target was discarded.
	TaskRejected ReviewTaskStatus = 2
	// TaskCancelled means the submitter withdrew the task.
	TaskCancelled ReviewTaskStatus = 3
)

// String renders the status for logs.
func (r ReviewTaskStatus) String() string {
	switch r {
	case TaskPending:
		return "Pending"
	case TaskApproved:
		return "Approved"
	case TaskRejected:
		return "Rejected"
	case TaskCancelled:
		return "Cancelled"
	default:
		return fmt.Sprintf("ReviewTaskStatus(%d)", int(r))
	}
}

// ReviewTaskKind distinguishes new submissions from edits to existing content.
type ReviewTaskKind int

// Review task kinds.
const (
	// KindNewContent is a newly created tag, artist or image.
	KindNewContent ReviewTaskKind = 0
	// KindEdit is a proposed modification to existing content.
	KindEdit ReviewTaskKind = 1
)

// ReviewableType identifies which kind of content a task refers to.
type ReviewableType int

// Reviewable content types.
const (
	// TargetImage refers to the Image table.
	TargetImage ReviewableType = 0
	// TargetTag refers to the Tag table.
	TargetTag ReviewableType = 1
	// TargetArtist refers to the Artist table.
	TargetArtist ReviewableType = 2
)

// String renders the type for logs and API responses.
func (r ReviewableType) String() string {
	switch r {
	case TargetImage:
		return "Image"
	case TargetTag:
		return "Tag"
	case TargetArtist:
		return "Artist"
	default:
		return fmt.Sprintf("ReviewableType(%d)", int(r))
	}
}

// ParseReviewableType converts the wire representation, which is the C# enum
// name, into a ReviewableType.
//
// The wire format is the string name because the frontend passes
// "?type=Image" to the review page. Accepting only the exact name keeps the
// contract strict; an unknown value is an error rather than a silent default.
func ParseReviewableType(value string) (ReviewableType, error) {
	switch value {
	case "Image":
		return TargetImage, nil
	case "Tag":
		return TargetTag, nil
	case "Artist":
		return TargetArtist, nil
	default:
		return 0, fmt.Errorf("unknown reviewable type %q; expected Image, Tag or Artist", value)
	}
}

// BannerVariant is the visual style of the site banner.
type BannerVariant int

// Banner variants.
const (
	VariantInfo    BannerVariant = 0
	VariantSuccess BannerVariant = 1
	VariantWarning BannerVariant = 2
	Danger         BannerVariant = 3
)

// String returns the wire name, which is the C# enum name.
func (b BannerVariant) String() string {
	switch b {
	case VariantInfo:
		return "Info"
	case VariantSuccess:
		return "Success"
	case VariantWarning:
		return "Warning"
	case Danger:
		return "Danger"
	default:
		return "Info"
	}
}

// ParseBannerVariant converts a wire name into a BannerVariant, defaulting to
// Info for an unrecognised value.
//
// Unlike roles this defaults rather than failing: the banner is cosmetic, and
// refusing to render it because of an unknown style would be worse than
// rendering it in the default style.
func ParseBannerVariant(value string) BannerVariant {
	switch value {
	case "Success":
		return VariantSuccess
	case "Warning":
		return VariantWarning
	case "Danger":
		return Danger
	default:
		return VariantInfo
	}
}

// User is a row of the User table.
type User struct {
	ID                 int64
	DiscordID          string
	Name               string
	AvatarURL          *string
	Role               int
	RequestCount       int64
	JWTRequestCount    int64
	APIKeyRequestCount int64
	IsBlacklisted      bool
	BlacklistReason    *string
}

// Image is a row of the Image table.
type Image struct {
	ID             int64
	UploaderID     *int64
	Width          int64
	Height         int64
	ByteSize       int64
	Extension      string
	DominantColor  string
	Source         *string
	IsNsfw         bool
	IsAnimated     bool
	UploadedAt     time.Time
	PerceptualHash uint64
	ReviewStatus   ReviewStatus
}

// Tag is a row of the Tag table.
type Tag struct {
	ID           int64
	Name         string
	Slug         string
	Description  string
	CreatorID    *int64
	ReviewStatus ReviewStatus
}

// Artist is a row of the Artist table.
type Artist struct {
	ID           int64
	Name         string
	DeviantArt   *string
	Twitter      *string
	Patreon      *string
	Pixiv        *string
	CreatorID    *int64
	ReviewStatus ReviewStatus
}

// ArtistMinimal is the reduced artist shape embedded in image responses.
type ArtistMinimal struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TagMinimal is the reduced tag shape embedded in image responses.
type TagMinimal struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// UserMinimal is the reduced user shape embedded in responses.
type UserMinimal struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	DiscordID       string  `json:"discordId"`
	AvatarURL       *string `json:"avatarUrl,omitempty"`
	Role            string  `json:"role"`
	BlacklistReason *string `json:"blacklistReason,omitempty"`
}

// ReviewTask is a row of the ReviewTask table.
type ReviewTask struct {
	ID                int64
	Kind              ReviewTaskKind
	TargetType        ReviewableType
	TargetId          int64
	SubmitterID       *int64
	Payload           []byte
	Reason            string
	Status            ReviewTaskStatus
	ReviewerNote      string
	ReviewerID        *int64
	ModeratorEditedAt *time.Time
	CreatedAt         time.Time
	ResolvedAt        *time.Time
}

// SiteBanner is a row of the SiteBanner table.
type SiteBanner struct {
	ID          int
	Message     string
	Variant     BannerVariant
	IsEnabled   bool
	UpdatedAt   time.Time
	UpdatedByID *int64
}

// PublicStats is the payload of GET /stats/public.
type PublicStats struct {
	TotalImages   int64 `json:"totalImages"`
	TotalTags     int64 `json:"totalTags"`
	TotalArtists  int64 `json:"totalArtists"`
	TotalRequests int64 `json:"totalRequests"`
}

// APIKey is a row of the ApiKey table.
//
// Only the hash of the key is stored. The plaintext is returned once at
// creation and cannot be recovered afterwards, which is what allows a leaked
// database to be disclosed without also leaking every key.
type APIKey struct {
	ID          int64
	UserID      int64
	Description string
	CreatedAt   time.Time
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
}

// ReportStatus is the state of a user report.
type ReportStatus int

// Report status values.
const (
	// ReportPending means the report has not been reviewed.
	ReportPending ReportStatus = 0
	// ReportAccepted means the report was upheld.
	ReportAccepted ReportStatus = 1
	// ReportRejected means the report was dismissed.
	ReportRejected ReportStatus = 2
)

// String renders the status for the API response.
func (r ReportStatus) String() string {
	switch r {
	case ReportPending:
		return "Pending"
	case ReportAccepted:
		return "Accepted"
	case ReportRejected:
		return "Rejected"
	default:
		return "Pending"
	}
}

// Report is a row of the Report table.
type Report struct {
	ID           int64
	UserID       int64
	ImageID      *int64
	Description  string
	Status       ReportStatus
	ReviewerNote *string
	CreatedAt    time.Time
}
