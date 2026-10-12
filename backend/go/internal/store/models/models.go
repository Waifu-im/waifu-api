// Package models contains the Go representation of the database rows.
//
// Column names are PascalCase with quoted identifiers, exactly as the C# service
// stores them. They are quoted in every query rather than folded to snake_case,
// because the schema is frozen: it holds production data, so renaming a column
// is a breaking change rather than a refactor.
//
// Enums are mirrored as plain integers with named constants, because that is
// how they are persisted. The C# service compares roles with >= and review
// statuses by equality, so the numeric values are load bearing.
package models

import (
	"fmt"
	"strconv"
	"strings"
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

// ParseReviewStatus converts a wire value.
//
// This is the strict counterpart to the review *filter* parser: the filter
// accepts a third value ("All") because it selects a set, whereas a stored
// status only has two members.
func ParseReviewStatus(value string) (ReviewStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending":
		return ReviewPending, true
	case "accepted":
		return ReviewAccepted, true
	default:
		return 0, false
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

// ReviewStatusFilter selects which review states a list query returns.
//
// The numeric values are persisted nowhere, but the wire form is the C# enum
// name, so ParseReviewStatusFilter is part of the contract.
type ReviewStatusFilter int

// Review status filter values.
const (
	// FilterAccepted is the default: published content only.
	FilterAccepted ReviewStatusFilter = 0
	// FilterPending is content awaiting a decision.
	FilterPending ReviewStatusFilter = 1
	// FilterAll disables the review status clause entirely.
	FilterAll ReviewStatusFilter = 2
)

// ParseReviewStatusFilter converts a wire value.
//
// Both the names and the underlying integers are accepted. Query-string enum
// binding in ASP.NET Core runs through the JsonStringEnumConverter registered in
// Program.cs, and that converter leaves AllowIntegerValues on, so "reviewStatus=1"
// selects pending content in the original service rather than being rejected.
//
// Anything unrecognised is an error rather than a silent default: a client
// asking for "pending" and receiving accepted content would see a wrong list
// with no indication that the filter was dropped.
func ParseReviewStatusFilter(value string) (ReviewStatusFilter, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))

	switch trimmed {
	case "", "accepted":
		return FilterAccepted, nil
	case "pending":
		return FilterPending, nil
	case "all":
		return FilterAll, nil
	}

	if number, err := strconv.Atoi(trimmed); err == nil {
		switch ReviewStatusFilter(number) {
		case FilterAccepted, FilterPending, FilterAll:
			return ReviewStatusFilter(number), nil
		}
	}

	return 0, fmt.Errorf("unknown review status %q; expected Accepted, Pending or All", value)
}

// String renders the filter for the API response.
func (r ReviewStatusFilter) String() string {
	switch r {
	case FilterPending:
		return "Pending"
	case FilterAll:
		return "All"
	default:
		return "Accepted"
	}
}

// NsfwMode is the tri-state NSFW filter.
//
// It is a mode rather than a boolean because "unspecified" is not an option on
// the public endpoint: the default is SFW only, so an anonymous caller cannot
// reach NSFW content by omitting the parameter.
type NsfwMode int

// NSFW filter values.
const (
	NsfwFalse NsfwMode = 0
	NsfwTrue  NsfwMode = 1
	NsfwAll   NsfwMode = 2
)

// ParseNsfwMode converts a wire value. The empty string is the default, SFW only.
func ParseNsfwMode(value string) (NsfwMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false":
		return NsfwFalse, nil
	case "true":
		return NsfwTrue, nil
	case "all":
		return NsfwAll, nil
	default:
		return 0, fmt.Errorf("unknown nsfw mode %q; expected False, True or All", value)
	}
}

// AnimatedMode is the tri-state animation filter, defaulting to All.
type AnimatedMode int

// Animated filter values.
const (
	AnimatedFalse AnimatedMode = 0
	AnimatedTrue  AnimatedMode = 1
	AnimatedAll   AnimatedMode = 2
)

// ParseAnimatedMode converts a wire value. The empty string is the default, All.
func ParseAnimatedMode(value string) (AnimatedMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "false":
		return AnimatedFalse, nil
	case "true":
		return AnimatedTrue, nil
	case "", "all":
		return AnimatedAll, nil
	default:
		return 0, fmt.Errorf("unknown animated mode %q; expected False, True or All", value)
	}
}

// Orientation filters images by aspect ratio.
type Orientation int

// Orientation values.
const (
	OrientationAll       Orientation = 0
	OrientationLandscape Orientation = 1
	OrientationPortrait  Orientation = 2
	OrientationSquare    Orientation = 3
)

// ParseOrientation converts a wire value. The empty string is the default, All.
func ParseOrientation(value string) (Orientation, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return OrientationAll, nil
	case "landscape":
		return OrientationLandscape, nil
	case "portrait":
		return OrientationPortrait, nil
	case "square":
		return OrientationSquare, nil
	default:
		return 0, fmt.Errorf("unknown orientation %q; expected All, Landscape, Portrait or Square", value)
	}
}

// ImageOrderBy selects the sort order of an image query.
//
// AddedToAlbum is only meaningful inside an album and is rejected elsewhere,
// which is why it is a separate member rather than another sort column.
type ImageOrderBy int

// Image order values.
const (
	OrderRandom       ImageOrderBy = 0
	OrderUploadedAt   ImageOrderBy = 1
	OrderFavorites    ImageOrderBy = 2
	OrderAddedToAlbum ImageOrderBy = 3
)

// ParseImageOrderBy converts a wire value. The empty string is the default,
// random.
func ParseImageOrderBy(value string) (ImageOrderBy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "random":
		return OrderRandom, nil
	case "uploadedat":
		return OrderUploadedAt, nil
	case "favorites":
		return OrderFavorites, nil
	case "addedtoalbum":
		return OrderAddedToAlbum, nil
	default:
		return 0, fmt.Errorf(
			"unknown orderBy %q; expected Random, UploadedAt, Favorites or AddedToAlbum", value)
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

// ParseBannerVariant converts a wire name into a BannerVariant.
//
// Matching is case-insensitive because System.Text.Json's JsonStringEnumConverter
// resolves enum names that way, so the original service accepts "warning" just
// as readily as "Warning".
//
// An unrecognised value is reported rather than folded into Info. The banner is
// cosmetic, but the C# validator marks this field with IsInEnum and rejects it,
// so a lenient parse here would quietly accept a request the original refuses
// and then store a style the caller never asked for. Note that an ABSENT field is
// not unrecognised: the request model's default is Info, and that arrives here as
// the explicit string "Info".
func ParseBannerVariant(value string) (BannerVariant, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info":
		return VariantInfo, true
	case "success":
		return VariantSuccess, true
	case "warning":
		return VariantWarning, true
	case "danger":
		return Danger, true
	default:
		return VariantInfo, false
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
//
// The names and order mirror ReportStatus.cs: a report starts Pending and ends
// up Resolved (upheld), Rejected (dismissed) or Cancelled (withdrawn). The
// numeric values are stored in the database, so they cannot change.
type ReportStatus int

// Report status values.
const (
	// ReportPending means the report has not been reviewed.
	ReportPending ReportStatus = 0
	// ReportResolved means a moderator upheld the report.
	ReportResolved ReportStatus = 1
	// ReportRejected means a moderator dismissed the report.
	ReportRejected ReportStatus = 2
	// ReportCancelled means the reporter withdrew it.
	ReportCancelled ReportStatus = 3
)

// String renders the status for the API response.
//
// The API serialises enums as names, because Program.cs registers
// JsonStringEnumConverter. These strings are therefore part of the contract.
func (r ReportStatus) String() string {
	switch r {
	case ReportPending:
		return "Pending"
	case ReportResolved:
		return "Resolved"
	case ReportRejected:
		return "Rejected"
	case ReportCancelled:
		return "Cancelled"
	default:
		return "Pending"
	}
}

// ParseReportStatus converts a wire name into a ReportStatus.
//
// Case-insensitive because System.Text.Json's enum converter accepts any casing
// on the way in, so "?status=resolved" round-trips in the C# service and must
// here too.
func ParseReportStatus(value string) (ReportStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending":
		return ReportPending, true
	case "resolved":
		return ReportResolved, true
	case "rejected":
		return ReportRejected, true
	case "cancelled", "canceled":
		return ReportCancelled, true
	default:
		return 0, false
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
