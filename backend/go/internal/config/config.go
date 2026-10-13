// Package config loads and validates the service configuration from the
// environment.
//
// # Deliberate difference from the C# service
//
// The C# implementation reads configuration through the IConfiguration string
// indexer using string constants, with no types and no validation. Two
// consequences bite in production:
//
//   - ReviewPolicy.cs tries to parse a role name and, if that fails, silently
//     falls back to TrustedUser. A typo quietly changes who can bypass review,
//     with no log line anywhere.
//   - PermissionService.cs treats a blank value as "no restriction at all", so
//     an unset variable opens an endpoint rather than closing it.
//
// This package inverts both defaults. A missing or unparseable value is a
// startup failure, not a silent behaviour change. Getting the service to start
// means the configuration is understood.
package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"waifuapi/internal/imageproc"
)

// Role is a user privilege level.
//
// The numeric values matter: they are compared with >= throughout the
// authorisation code, exactly as the C# service does. They are also what the
// Images/Users tables store, so they must never be reordered or renumbered.
//
// Mirrors backend/WaifuApi.Domain/Enums/Role.cs.
type Role int

// Role levels, in ascending order of privilege.
const (
	// RoleUser is an ordinary authenticated user.
	RoleUser Role = 0
	// RoleTrustedUser may bypass content review, depending on configuration.
	RoleTrustedUser Role = 1
	// RoleModerator may approve or reject review tasks.
	RoleModerator Role = 2
	// RoleAdmin has full access.
	RoleAdmin Role = 3
)

// roleNames maps each level to the string used in JWT and API key claims.
//
// Both C# authentication handlers emit the role as a *name*, not a number, so
// these strings are part of the wire contract and must not change.
var roleNames = map[Role]string{
	RoleUser:        "User",
	RoleTrustedUser: "TrustedUser",
	RoleModerator:   "Moderator",
	RoleAdmin:       "Admin",
}

// roleValues is the reverse mapping, used when parsing configuration.
var roleValues = map[string]Role{
	"user":        RoleUser,
	"trusteduser": RoleTrustedUser,
	"moderator":   RoleModerator,
	"admin":       RoleAdmin,
}

// String returns the claim name for the role, for example "TrustedUser".
func (r Role) String() string {
	if name, ok := roleNames[r]; ok {
		return name
	}
	return fmt.Sprintf("Role(%d)", int(r))
}

// IsValid reports whether the role is one of the four defined levels.
func (r Role) IsValid() bool {
	_, ok := roleNames[r]
	return ok
}

// CanAccessUnacceptedContent reports whether this role may see content that is
// still Pending review.
//
// Mirrors RoleUtils.CanAccessNonAcceptedReviewStatus: moderators and admins
// only. Ordinary users must not be able to enumerate pending submissions.
func (r Role) CanAccessUnacceptedContent() bool {
	return r == RoleModerator || r == RoleAdmin
}

// IsModeratorOrAdmin reports whether the role can moderate.
func (r Role) IsModeratorOrAdmin() bool {
	return r == RoleModerator || r == RoleAdmin
}

// ParseRole converts a configuration or claim string into a Role.
//
// Unlike the C# service this never falls back to a default. An unrecognised
// value is an error, because silently choosing a role means silently changing
// who can moderate the site.
func ParseRole(value string) (Role, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("role value is empty")
	}

	role, ok := roleValues[strings.ToLower(trimmed)]
	if !ok {
		// Sorted so the message is stable and easy to scan.
		known := make([]string, 0, len(roleValues))
		for k := range roleValues {
			known = append(known, k)
		}
		sort.Strings(known)
		return 0, fmt.Errorf("unknown role %q; expected one of %s", value, strings.Join(known, ", "))
	}
	return role, nil
}

// Config is the fully validated service configuration.
type Config struct {
	// Environment is one of "development", "test" or "production".
	Environment string

	// Port is the TCP port the HTTP server listens on.
	Port int

	// BasePath is prefixed to every route, mirroring API_BASE_PATH in .NET.
	// An empty string or "/" means routes are served from the root.
	BasePath string

	// DatabaseURL is the Postgres connection string.
	DatabaseURL string

	// MaxUploadSizeBytes is the largest accepted request body.
	MaxUploadSizeBytes int64

	// Review controls the content moderation pipeline.
	Review ReviewConfig

	// Permissions controls minimum roles for privileged actions.
	Permissions PermissionsConfig

	// JWT configures token issuance and validation.
	JWT JWTConfig

	// S3 and CDN describe object storage.
	S3    S3Config
	CDN   CDNConfig
	Image ImageConfig

	// Cloudflare is optional. With no credentials the edge cache is simply never
	// purged, which is correct for a self-hosted instance that does not use the
	// edge at all, so it must not be a startup failure.
	Cloudflare CloudflareConfig

	// ImageTransform describes how the frontend requests resized variants. It is
	// needed to build the exact URLs a cache purge must target: purging a
	// differently-spelled variant misses the cache entry and the stale image
	// stays visible.
	ImageTransform ImageTransformConfig

	// Discord configures OAuth login.
	Discord DiscordConfig

	// Page sizes are configured per resource, exactly as the C# service does with
	// the {Feature}__DefaultPageSize / {Feature}__MaxPageSize pairs. They are
	// separate settings rather than one global pair because the values in use
	// differ wildly: images default to a single row per request (the gallery is
	// an infinite scroll over a randomised result set) while tags, artists,
	// albums, users, reports and the review queue default to 30.
	Pages PaginationConfig
}

// PaginationConfig is the per-resource default and maximum page size.
type PaginationConfig struct {
	Image       PageSize
	Tag         PageSize
	Artist      PageSize
	Album       PageSize
	User        PageSize
	Report      PageSize
	Review      PageSize
	StorageDiff PageSize
}

// PageSize is one resource's pagination limits.
//
// Max of 0 or less means "no ceiling", matching PaginationUtils.Normalize in the
// C# service, which only clamps when maxPageSize > 0. Production sets MaxPageSize
// to -1 everywhere, so this is the normal case rather than an edge one.
type PageSize struct {
	Default int
	Max     int
}

// ReviewConfig mirrors the Review__* settings.
type ReviewConfig struct {
	RequireImageReview  bool
	RequireTagReview    bool
	RequireArtistReview bool
	RequireEditReview   bool
	ImageBypassMinRole  Role
	TagBypassMinRole    Role
	ArtistBypassMinRole Role
	ImageEditBypassMin  Role
	TagEditBypassMin    Role
	ArtistEditBypassMin Role
	MaxOpenEditsPerUser int
	TaskRetentionDays   int
}

// BypassDisabled is the configuration value that stops anyone bypassing review,
// including admins.
//
// It exists because a Role cannot express "higher than Admin".
const BypassDisabled = "None"

// PermissionsConfig mirrors the Permissions__* settings.
type PermissionsConfig struct {
	TagCreationMinRole      Role
	ArtistCreationMinRole   Role
	ImageUploadMinRole      Role
	SubmitEditMinRole       Role
	BannerManagementMinRole Role
}

// JWTConfig configures token handling.
type JWTConfig struct {
	Key      string
	Issuer   string
	Audience string
}

// S3Config describes object storage.
type S3Config struct {
	// Provider selects provider-specific defaults: generic, aws, r2, idrive or
	// minio. See internal/storage for what each one implies.
	//
	// Default is "generic", which applies nothing and relies entirely on the
	// other fields. Defaulting to a specific provider would be wrong for a
	// self-hosted deployment that happens to speak the same API.
	Provider string

	AccessKey      string
	SecretKey      string
	ServiceURL     string
	Region         string
	BucketName     string
	ForcePathStyle bool
}

// CDNConfig describes how image URLs are built.
type CDNConfig struct {
	BaseURL string
}

// ImageConfig carries the upload validation limits.
type ImageConfig struct {
	MinWidth          int
	MinHeight         int
	MaxWidth          int
	MaxHeight         int
	AnimatedMinWidth  int
	AnimatedMinHeight int
	AnimatedMaxWidth  int
	AnimatedMaxHeight int

	// WebpEnabled gates re-encoding uploads to WebP on ingest. Off by
	// default: it changes what is stored for every future upload, so it is
	// opt-in rather than baked in. Mirrors ImageConverter.cs.
	WebpEnabled bool

	// WebpQuality is 1-100. Defaults to the value the offline migration used.
	WebpQuality int

	// WebpSourceExtensions is the comma-separated list of eligible source
	// formats. PNG only by default; see ImageConverter.cs for why JPEG is
	// excluded.
	WebpSourceExtensions string

	// WebpMethod is the libwebp effort level, 0-6.
	WebpMethod int
}

// WebpConversion projects the WebP settings into the shape the image package
// takes.
//
// Parsing is deferred to that package so the same code path serves this
// constructor and any future caller, rather than duplicating the defaulting
// rules in two places.
func (c ImageConfig) WebpConversion() imageproc.ConversionConfig {
	return imageproc.ConversionConfig{
		Enabled:          c.WebpEnabled,
		Quality:          c.WebpQuality,
		SourceExtensions: imageproc.ParseConversionSourceExtensions(c.WebpSourceExtensions),
		Method:           c.WebpMethod,
	}
}

// DiscordConfig configures OAuth login.
type DiscordConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// CloudflareConfig configures edge cache purging.
//
// Both fields are optional and deliberately so. The C# service reads them
// through the string indexer and no-ops when they are absent; requiring them
// here would make the service unstartable on any deployment that serves images
// without the edge in front.
type CloudflareConfig struct {
	ZoneID string
	Token  string
}

// ImageTransformConfig mirrors the IMG_TRANSFORM_* settings the frontend also
// reads. The two implementations must agree on the variant URL format, because
// a purge is keyed on the cached URL.
type ImageTransformConfig struct {
	Enabled bool
	Path    string
	// WidthParam is the query parameter carrying the requested width.
	WidthParam string
	CardWidth  int
	// PreviewWidth is the width used by the image page's larger preview.
	PreviewWidth int
}

// Loader reads values from the environment, or from an explicit map for tests.
//
// The indirection exists so configuration can be unit tested without mutating
// process-wide state.
type Loader struct {
	values map[string]string
}

// NewLoader returns a Loader backed by the process environment.
func NewLoader() *Loader {
	return &Loader{values: environmentMap()}
}

// NewLoaderFromMap returns a Loader backed by a supplied map. Intended for
// tests.
func NewLoaderFromMap(values map[string]string) *Loader {
	return &Loader{values: values}
}

// environmentMap snapshots os.Environ into a map.
func environmentMap() map[string]string {
	out := make(map[string]string)
	for _, pair := range os.Environ() {
		if idx := strings.Index(pair, "="); idx > 0 {
			out[pair[:idx]] = pair[idx+1:]
		}
	}
	return out
}

// get returns a raw value, or the empty string when unset.
func (l *Loader) get(key string) string {
	return strings.TrimSpace(l.values[key])
}

// require returns a value or an error naming the missing variable.
//
// Missing variables are reported in a single accumulated error rather than one
// at a time, so a misconfigured deployment can be fixed in a single pass
// instead of through repeated restarts.
func (l *Loader) require(key string, missing *[]string) string {
	value := l.get(key)
	if value == "" {
		*missing = append(*missing, key)
	}
	return value
}

// requireInt parses a required integer.
func (l *Loader) requireInt(key string, missing *[]string) int {
	raw := l.require(key, missing)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		*missing = append(*missing, fmt.Sprintf("%s (not an integer: %q)", key, raw))
		return 0
	}
	return value
}

// optionalInt parses an integer with a default when unset or blank.
func (l *Loader) optionalInt(key string, fallback int) (int, error) {
	raw := l.get(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return value, nil
}

// requireBool parses a required boolean.
//
// Only the literal forms strconv accepts as true or false are allowed, so a
// typo like "yes" fails loudly instead of being treated as false and silently
// disabling a safety feature.
func (l *Loader) requireBool(key string, missing *[]string) bool {
	raw := l.require(key, missing)
	if raw == "" {
		return false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		*missing = append(*missing, fmt.Sprintf("%s (not a boolean: %q)", key, raw))
		return false
	}
	return value
}

// requireRole parses a required role name.
func (l *Loader) requireRole(key string, missing *[]string) Role {
	raw := l.require(key, missing)
	if raw == "" {
		return RoleUser
	}
	role, err := ParseRole(raw)
	if err != nil {
		*missing = append(*missing, fmt.Sprintf("%s (%v)", key, err))
		return RoleUser
	}
	return role
}

// requireBypassRole parses a review bypass role, which additionally accepts the
// special value "None" meaning nobody bypasses.
func (l *Loader) requireBypassRole(key string, missing *[]string) Role {
	raw := l.require(key, missing)
	if raw == "" {
		return RoleTrustedUser
	}
	if strings.EqualFold(raw, BypassDisabled) {
		// Represented as RoleUser with a sentinel meaning, since no Role value
		// means "higher than Admin". See BypassNone.
		return BypassNone
	}
	role, err := ParseRole(raw)
	if err != nil {
		*missing = append(*missing, fmt.Sprintf("%s (%v)", key, err))
		return RoleTrustedUser
	}
	return role
}

// BypassNone is a sentinel meaning that review is required for every role,
// including admins.
//
// A dedicated value is clearer than overloading RoleUser, which would make
// "min role" comparisons silently wrong.
const BypassNone Role = -1

// Load reads and validates the entire configuration.
//
// Every problem is accumulated and returned together. Callers should treat a
// non-nil result as fatal: the service must not start with a configuration it
// does not fully understand.
func Load(l *Loader) (*Config, error) {
	var missing []string

	cfg := &Config{}

	cfg.Environment = l.get("ASPNETCORE_ENVIRONMENT")
	if cfg.Environment == "" {
		cfg.Environment = l.get("APP_ENVIRONMENT")
	}
	if cfg.Environment == "" {
		cfg.Environment = "production"
	}

	cfg.Port = l.optionalIntOrAccumulate("PORT", 8080, &missing)
	cfg.BasePath = normalizeBasePath(l.get("API_BASE_PATH"))
	cfg.DatabaseURL = l.require("ConnectionStrings__DefaultConnection", &missing)

	// Upload size is expressed in megabytes in configuration, matching the
	// Image__MaxUploadSizeMb variable the .NET service uses.
	maxUploadMB := l.requireInt("Image__MaxUploadSizeMb", &missing)
	if maxUploadMB <= 0 {
		maxUploadMB = 10
	}
	cfg.MaxUploadSizeBytes = int64(maxUploadMB) * 1024 * 1024

	// Review switches. These are required because the C# service treats them
	// as mandatory and fails to boot without them; keeping them required here
	// means the two services agree about what a valid configuration is.
	cfg.Review.RequireImageReview = l.requireBool("Review__RequireImageReview", &missing)
	cfg.Review.RequireArtistReview = l.requireBool("Review__RequireArtistReview", &missing)
	cfg.Review.RequireTagReview = l.requireBool("Review__RequireTagReview", &missing)
	cfg.Review.RequireEditReview = l.requireBool("Review__RequireEditRequestReview", &missing)

	cfg.Review.ImageBypassMinRole = l.requireBypassRole("Review__ImageReviewBypassMinRole", &missing)
	cfg.Review.TagBypassMinRole = l.requireBypassRole("Review__TagReviewBypassMinRole", &missing)
	cfg.Review.ArtistBypassMinRole = l.requireBypassRole("Review__ArtistReviewBypassMinRole", &missing)
	cfg.Review.ImageEditBypassMin = l.requireBypassRole("Review__ImageEditReviewBypassMinRole", &missing)
	cfg.Review.TagEditBypassMin = l.requireBypassRole("Review__TagEditReviewBypassMinRole", &missing)
	cfg.Review.ArtistEditBypassMin = l.requireBypassRole("Review__ArtistEditReviewBypassMinRole", &missing)

	maxEdits, err := l.optionalInt("Review__MaxOpenEditsPerUser", 20)
	if err != nil {
		missing = append(missing, err.Error())
	}
	cfg.Review.MaxOpenEditsPerUser = maxEdits

	retention, err := l.optionalInt("Review__TaskRetentionDays", 90)
	if err != nil {
		missing = append(missing, err.Error())
	}
	cfg.Review.TaskRetentionDays = retention

	cfg.Permissions.TagCreationMinRole = l.requireRole("Permissions__TagCreationMinRole", &missing)
	cfg.Permissions.ArtistCreationMinRole = l.requireRole("Permissions__ArtistCreationMinRole", &missing)
	cfg.Permissions.ImageUploadMinRole = l.requireRole("Permissions__ImageUploadMinRole", &missing)
	// Permissions::SubmitEditMinRole is declared in ConfigurationKeys and read
	// nowhere in the C# service, so it must not be able to stop a deployment
	// from booting. The default is what .env.example documents.
	submitEditRole, err := l.optionalRole("Permissions__SubmitEditMinRole", RoleUser)
	if err != nil {
		missing = append(missing, err.Error())
	}
	cfg.Permissions.SubmitEditMinRole = submitEditRole

	// Banner management defaults to Moderator rather than being required,
	// because unlike the others its absence is not an open door: the fallback
	// is still a privileged role. This mirrors PermissionService.cs, which
	// passes Role.Moderator as the default for exactly this key.
	bannerRole, err := l.optionalRole("Permissions__BannerManagementMinRole", RoleModerator)
	if err != nil {
		missing = append(missing, err.Error())
	}
	cfg.Permissions.BannerManagementMinRole = bannerRole

	cfg.JWT.Key = l.require("Jwt__Key", &missing)
	cfg.JWT.Issuer = l.require("Jwt__Issuer", &missing)
	cfg.JWT.Audience = l.require("Jwt__Audience", &missing)

	cfg.S3.Provider = l.get("S3__Provider")
	if cfg.S3.Provider == "" {
		cfg.S3.Provider = "generic"
	}
	cfg.S3.AccessKey = l.require("S3__AccessKey", &missing)
	cfg.S3.SecretKey = l.require("S3__SecretKey", &missing)
	cfg.S3.ServiceURL = l.require("S3__ServiceUrl", &missing)
	cfg.S3.Region = l.require("S3__Region", &missing)
	cfg.S3.BucketName = l.require("S3__BucketName", &missing)
	if forcePath, err := l.optionalBool("S3__ForcePathStyle", false); err == nil {
		cfg.S3.ForcePathStyle = forcePath
	} else {
		missing = append(missing, err.Error())
	}

	cfg.CDN.BaseURL = l.require("Cdn__BaseUrl", &missing)

	// Edge cache purging. Optional: absent credentials mean the cache is simply
	// never purged, which is correct for a deployment not using the edge.
	cfg.Cloudflare.ZoneID = l.get("Cloudflare__ZoneId")
	cfg.Cloudflare.Token = l.get("Cloudflare__ApiToken")

	transformEnabled, err := l.optionalBool("IMG_TRANSFORM_ENABLED", false)
	if err != nil {
		missing = append(missing, err.Error())
	}
	cfg.ImageTransform.Enabled = transformEnabled
	cfg.ImageTransform.Path = l.get("IMG_TRANSFORM_PATH")

	widthParam := l.get("IMG_WIDTH_PARAM")
	if widthParam == "" {
		widthParam = "width"
	}
	cfg.ImageTransform.WidthParam = widthParam
	cfg.ImageTransform.CardWidth = l.optionalIntOrAccumulate("IMG_CARD_WIDTH", 640, &missing)
	cfg.ImageTransform.PreviewWidth = l.optionalIntOrAccumulate("IMG_PREVIEW_WIDTH", 1280, &missing)

	cfg.Image.MinWidth = l.requireInt("Image__MinWidth", &missing)
	cfg.Image.MinHeight = l.requireInt("Image__MinHeight", &missing)
	cfg.Image.MaxWidth = l.requireInt("Image__MaxWidth", &missing)
	cfg.Image.MaxHeight = l.requireInt("Image__MaxHeight", &missing)
	cfg.Image.AnimatedMinWidth = l.requireInt("Image__AnimatedMinWidth", &missing)
	cfg.Image.AnimatedMinHeight = l.requireInt("Image__AnimatedMinHeight", &missing)
	cfg.Image.AnimatedMaxWidth = l.requireInt("Image__AnimatedMaxWidth", &missing)
	cfg.Image.AnimatedMaxHeight = l.requireInt("Image__AnimatedMaxHeight", &missing)

	// WebP conversion settings are optional, and default to off.
	//
	// The C# service reads these with GetValue<T>, which yields the type's zero
	// value when the key is absent: false, 0 and an empty string respectively.
	// The defaults applied here restore the values ImageConverter.cs falls back
	// to internally, because a quality of 0 would be rejected by the encoder
	// rather than meaning "unset".
	cfg.Image.WebpEnabled, err = l.optionalBool("Image__WebpConversionEnabled", false)
	if err != nil {
		missing = append(missing, "Image__WebpConversionEnabled")
	}
	cfg.Image.WebpSourceExtensions = l.get("Image__WebpSourceExtensions")

	quality, qualityErr := l.optionalInt("Image__WebpQuality", imageproc.DefaultWebpQuality)
	if qualityErr != nil {
		missing = append(missing, "Image__WebpQuality")
	}
	cfg.Image.WebpQuality = quality

	method, methodErr := l.optionalInt("Image__WebpMethod", imageproc.DefaultWebpMethod)
	if methodErr != nil {
		missing = append(missing, "Image__WebpMethod")
	}
	cfg.Image.WebpMethod = method

	cfg.Discord.ClientID = l.require("Discord__ClientId", &missing)
	cfg.Discord.ClientSecret = l.require("Discord__ClientSecret", &missing)
	cfg.Discord.RedirectURI = l.require("Discord__RedirectUri", &missing)

	// Page sizes. These are required in the C# service too: every query handler
	// does int.Parse(configuration[key] ?? throw), so an unset value stops the
	// process from booting. Keeping them required means the two services agree on
	// what a complete configuration is.
	cfg.Pages.Image = l.requirePageSize("Image", &missing)
	cfg.Pages.Tag = l.requirePageSize("Tag", &missing)
	cfg.Pages.Artist = l.requirePageSize("Artist", &missing)
	cfg.Pages.Album = l.requirePageSize("Album", &missing)
	cfg.Pages.User = l.requirePageSize("User", &missing)
	cfg.Pages.Report = l.requirePageSize("Report", &missing)
	cfg.Pages.Review = l.optionalPageSize("Review", 30, -1, &missing)
	cfg.Pages.StorageDiff = l.requirePageSize("StorageDiff", &missing)

	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("invalid configuration, %d problem(s):\n  - %s",
			len(missing), strings.Join(missing, "\n  - "))
	}

	if err := cfg.validateImageLimits(); err != nil {
		return nil, err
	}
	if len(cfg.JWT.Key) < 32 {
		return nil, fmt.Errorf("Jwt__Key must be at least 32 characters, got %d", len(cfg.JWT.Key))
	}

	return cfg, nil
}

// requirePageSize reads the Default/Max pair for one resource.
//
// Max is deliberately allowed to be zero or negative, because -1 is what
// production uses and it means "do not clamp".
func (l *Loader) requirePageSize(prefix string, missing *[]string) PageSize {
	return PageSize{
		Default: l.requireInt(prefix+"__DefaultPageSize", missing),
		Max:     l.requireInt(prefix+"__MaxPageSize", missing),
	}
}

// optionalPageSize reads a page size that the C# service does not require.
//
// The Review pair is the case that matters. Review:DefaultPageSize and
// Review:MaxPageSize are declared in ConfigurationKeys but read nowhere in the
// C# service — not by Program.cs's required list and not by any handler — while
// every other section's page size is required in both. Requiring them here
// invented a startup failure the reference implementation does not have, and it
// was found the hard way: a deployment whose environment is a faithful copy of
// production's refuses to boot over two keys nothing reads.
//
// The defaults match .env.example, so a deployment that sets them behaves
// exactly as before.
func (l *Loader) optionalPageSize(prefix string, defaultSize, maxSize int, missing *[]string) PageSize {
	return PageSize{
		Default: l.optionalIntOrAccumulate(prefix+"__DefaultPageSize", defaultSize, missing),
		Max:     l.optionalIntOrAccumulate(prefix+"__MaxPageSize", maxSize, missing),
	}
}

// optionalIntOrAccumulate reads an integer with a fallback, recording an error
// if the value is present but malformed.
func (l *Loader) optionalIntOrAccumulate(key string, fallback int, missing *[]string) int {
	value, err := l.optionalInt(key, fallback)
	if err != nil {
		*missing = append(*missing, err.Error())
		return fallback
	}
	return value
}

// optionalRole reads a role with a fallback.
func (l *Loader) optionalRole(key string, fallback Role) (Role, error) {
	raw := l.get(key)
	if raw == "" {
		return fallback, nil
	}
	return ParseRole(raw)
}

// optionalBool reads a boolean with a fallback.
func (l *Loader) optionalBool(key string, fallback bool) (bool, error) {
	raw := l.get(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q", key, raw)
	}
	return value, nil
}

// validateImageLimits rejects contradictory dimension bounds at startup rather
// than rejecting every upload at runtime.
func (c *Config) validateImageLimits() error {
	checks := []struct {
		name     string
		min, max int
	}{
		{"static", c.Image.MinWidth, c.Image.MaxWidth},
		{"animated", c.Image.AnimatedMinWidth, c.Image.AnimatedMaxWidth},
	}
	for _, check := range checks {
		if check.min <= 0 {
			return fmt.Errorf("%s minimum width must be positive, got %d", check.name, check.min)
		}
		if check.max < check.min {
			return fmt.Errorf("%s maximum width %d is below minimum %d", check.name, check.max, check.min)
		}
	}
	if c.MaxUploadSizeBytes <= 0 {
		return fmt.Errorf("maximum upload size must be positive, got %d bytes", c.MaxUploadSizeBytes)
	}
	return nil
}

// RequiresReviewForNewContent decides whether newly submitted content of the
// given kind must be reviewed before it is publicly visible.
//
// This is the Go equivalent of ReviewPolicy.RequiresReviewForNewContent. The
// comparison uses >= on the raw role values, matching the C# service.
func (c *Config) RequiresReviewForNewContent(require bool, bypassMin Role, userRole Role) bool {
	if !require {
		return false
	}
	return !canBypass(bypassMin, userRole)
}

// RequiresReviewForEditRequest decides whether a submitted edit needs approval.
func (c *Config) RequiresReviewForEditRequest(bypassMin Role, userRole Role) bool {
	return c.RequiresReviewForNewContent(c.Review.RequireEditReview, bypassMin, userRole)
}

// canBypass reports whether the role is high enough to skip review.
//
// BypassNone means nobody bypasses, not even an admin.
func canBypass(min Role, userRole Role) bool {
	if min == BypassNone {
		return false
	}
	return userRole >= min
}

// HasPermission reports whether a role satisfies a configured minimum.
func (c *Config) HasPermission(min Role, userRole Role) bool {
	return userRole >= min
}

// normalizeBasePath cleans API_BASE_PATH into either "" or "/prefix" form.
//
// The .NET service calls UsePathBase with the raw value, so "/" and "" are
// both common in existing deployments and both must work.
func normalizeBasePath(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

// BasePathOrRoot returns the configured base path, or "/" when unset.
//
// http.ServeMux requires patterns to start with "/", so an empty base path is
// inconvenient at the call site; this keeps the handlers readable.
func (c *Config) BasePathOrRoot() string {
	if c.BasePath == "" {
		return "/"
	}
	return c.BasePath
}
