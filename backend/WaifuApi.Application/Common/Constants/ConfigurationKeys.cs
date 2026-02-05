namespace WaifuApi.Application.Common.Constants;

public static class ConfigurationKeys
{
    public const string ApiBasePath = "API_BASE_PATH";

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

    public static class Moderation
    {
        public const string RequireImageReview = "Moderation:RequireImageReview";
        public const string RequireArtistReview = "Moderation:RequireArtistReview";
        public const string RequireTagReview = "Moderation:RequireTagReview";
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

    public static class Review
    {
        public const string DefaultPageSize = "Review:DefaultPageSize";
        public const string MaxPageSize = "Review:MaxPageSize";
    }

    public static class Permissions
    {
        public const string TagCreationMinRole = "Permissions:TagCreationMinRole";
        public const string ArtistCreationMinRole = "Permissions:ArtistCreationMinRole";
        public const string ImageUploadMinRole = "Permissions:ImageUploadMinRole";
    }
}
