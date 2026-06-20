namespace WaifuApi.Application.Common.Constants;

public static class ConfigurationKeys
{
    public const string ApiBasePath = "API_BASE_PATH";
    public const string ApiBaseUrl = "API_BASE_URL";

    public static class Frontend
    {
        public const string BaseUrl = "Frontend:BaseUrl";
    }

    public static class Cdn
    {
        public const string BaseUrl = "Cdn:BaseUrl";
    }

    public static class S3
    {
        public const string AccessKey = "S3:AccessKey";
        public const string SecretKey = "S3:SecretKey";
        public const string ServiceUrl = "S3:ServiceUrl";
        public const string BucketName = "S3:BucketName";
        public const string Region = "S3:Region";
    }

    public static class Discord
    {
        public const string ClientId = "Discord:ClientId";
        public const string ClientSecret = "Discord:ClientSecret";
        public const string RedirectUri = "Discord:RedirectUri";
    }

    public static class Jwt
    {
        public const string Key = "Jwt:Key";
        public const string Issuer = "Jwt:Issuer";
        public const string Audience = "Jwt:Audience";
    }

    public static class Review
    {
        public const string RequireImageReview = "Review:RequireImageReview";
        public const string RequireArtistReview = "Review:RequireArtistReview";
        public const string RequireTagReview = "Review:RequireTagReview";

        // Whether edit requests (proposed changes to existing content) require moderator approval.
        public const string RequireEditRequestReview = "Review:RequireEditRequestReview";

        // Minimum role that bypasses review when CREATING/uploading content of each type (auto-accepted).
        // Missing/blank => default (TrustedUser). Set to "None" to disable bypass entirely.
        public const string ImageReviewBypassMinRole = "Review:ImageReviewBypassMinRole";
        public const string TagReviewBypassMinRole = "Review:TagReviewBypassMinRole";
        public const string ArtistReviewBypassMinRole = "Review:ArtistReviewBypassMinRole";

        // Minimum role that bypasses review for EDIT REQUESTS of each type (auto-applied). Configured
        // independently from the creation bypass above. Missing/blank => default (TrustedUser). "None" disables.
        public const string ImageEditReviewBypassMinRole = "Review:ImageEditReviewBypassMinRole";
        public const string TagEditReviewBypassMinRole = "Review:TagEditReviewBypassMinRole";
        public const string ArtistEditReviewBypassMinRole = "Review:ArtistEditReviewBypassMinRole";

        // Review queue pagination.
        public const string DefaultPageSize = "Review:DefaultPageSize";
        public const string MaxPageSize = "Review:MaxPageSize";

        // Maximum number of simultaneously-open (pending) edit tasks a single user may have. 0 = unlimited.
        public const string MaxOpenEditsPerUser = "Review:MaxOpenEditsPerUser";

        // Resolved review tasks older than this are pruned by the cleanup service. 0 = keep forever.
        public const string TaskRetentionDays = "Review:TaskRetentionDays";
    }

    public static class Image
    {
        public const string DefaultPageSize = "Image:DefaultPageSize";
        public const string MaxPageSize = "Image:MaxPageSize";
        public const string MinWidth = "Image:MinWidth";
        public const string MinHeight = "Image:MinHeight";
        public const string MaxWidth = "Image:MaxWidth";
        public const string MaxHeight = "Image:MaxHeight";
        public const string AnimatedMinWidth = "Image:AnimatedMinWidth";
        public const string AnimatedMinHeight = "Image:AnimatedMinHeight";
        public const string AnimatedMaxWidth = "Image:AnimatedMaxWidth";
        public const string AnimatedMaxHeight = "Image:AnimatedMaxHeight";
        public const string MaxUploadSizeMb = "Image:MaxUploadSizeMb";
    }

    public static class Tag
    {
        public const string DefaultPageSize = "Tag:DefaultPageSize";
        public const string MaxPageSize = "Tag:MaxPageSize";
    }

    public static class Artist
    {
        public const string DefaultPageSize = "Artist:DefaultPageSize";
        public const string MaxPageSize = "Artist:MaxPageSize";
    }

    public static class Album
    {
        public const string DefaultPageSize = "Album:DefaultPageSize";
        public const string MaxPageSize = "Album:MaxPageSize";
    }

    public static class User
    {
        public const string DefaultPageSize = "User:DefaultPageSize";
        public const string MaxPageSize = "User:MaxPageSize";
    }

    public static class Report
    {
        public const string DefaultPageSize = "Report:DefaultPageSize";
        public const string MaxPageSize = "Report:MaxPageSize";
    }


    public static class StorageDiff
    {
        public const string DefaultPageSize = "StorageDiff:DefaultPageSize";
        public const string MaxPageSize = "StorageDiff:MaxPageSize";
    }

    public static class Permissions
    {
        public const string TagCreationMinRole = "Permissions:TagCreationMinRole";
        public const string ArtistCreationMinRole = "Permissions:ArtistCreationMinRole";
        public const string ImageUploadMinRole = "Permissions:ImageUploadMinRole";
        public const string SubmitEditMinRole = "Permissions:SubmitEditMinRole";
    }
}
