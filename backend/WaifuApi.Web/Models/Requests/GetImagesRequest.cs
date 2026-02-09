using System.Collections.Generic;
using System.ComponentModel;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Web.Models.Requests;

/// <summary>
/// Request model for searching and filtering images.
/// </summary>
public class GetImagesRequest
{
    /// <summary>
    /// Filter by NSFW status. Default: false (SFW only).
    /// </summary>
    [Description("Filter by NSFW status. Default: false (SFW only).")]
    public NsfwMode IsNsfw { get; set; } = NsfwMode.False;

    /// <summary>
    /// Only include images with ALL these tags (AND logic). Example: ?IncludedTags=waifu&amp;IncludedTags=maid
    /// </summary>
    [Description("Only include images with ALL these tags (AND logic). Example: ?IncludedTags=waifu&IncludedTags=maid")]
    public List<string> IncludedTags { get; set; } = new();

    /// <summary>
    /// Exclude images with ANY of these tags. Example: ?ExcludedTags=uniform&amp;ExcludedTags=maid
    /// </summary>
    [Description("Exclude images with ANY of these tags. Example: ?ExcludedTags=uniform&ExcludedTags=maid")]
    public List<string> ExcludedTags { get; set; } = new();

    /// <summary>
    /// Only include images by these artists (by ID, AND logic). Example: ?IncludedArtists=1
    /// </summary>
    [Description("Only include images by these artists (by ID, AND logic). Example: ?IncludedArtists=1")]
    public List<string> IncludedArtists { get; set; } = new();

    /// <summary>
    /// Exclude images by these artists (by ID). Example: ?ExcludedArtists=1
    /// </summary>
    [Description("Exclude images by these artists (by ID). Example: ?ExcludedArtists=1")]
    public List<string> ExcludedArtists { get; set; } = new();

    /// <summary>
    /// Only include these specific image IDs. Example: ?IncludedIds=8108&amp;IncludedIds=8008
    /// </summary>
    [Description("Only include these specific image IDs. Example: ?IncludedIds=8108&IncludedIds=8008")]
    public List<string> IncludedIds { get; set; } = new();

    /// <summary>
    /// Exclude these specific image IDs. Example: ?ExcludedIds=8108
    /// </summary>
    [Description("Exclude these specific image IDs. Example: ?ExcludedIds=8108")]
    public List<string> ExcludedIds { get; set; } = new();

    /// <summary>
    /// Filter by animation status. Default: all.
    /// </summary>
    [Description("Filter by animation status. Default: all.")]
    public AnimatedMode IsAnimated { get; set; } = AnimatedMode.All;

    /// <summary>
    /// Sort order. Default: random. Note: AddedToAlbum only works on album images endpoint.
    /// </summary>
    [Description("Sort order. Default: random. Note: AddedToAlbum only works on album images endpoint.")]
    public ImageOrderBy OrderBy { get; set; } = ImageOrderBy.Random;

    /// <summary>
    /// Filter by image orientation. Default: all.
    /// </summary>
    [Description("Filter by image orientation. Default: all.")]
    public Orientation Orientation { get; set; } = Orientation.All;

    /// <summary>
    /// Page number for pagination. Default: 1.
    /// </summary>
    [Description("Page number for pagination. Default: 1.")]
    public int Page { get; set; } = 1;

    /// <summary>
    /// Number of results per page.
    /// </summary>
    [Description("Number of results per page.")]                                                                                                                                                                         
    public int PageSize { get; set; }

    /// <summary>
    /// Filter by width in pixels. Examples: 1920 (exact), >=1920 (min), &lt;=1920 (max), 1280..1920 (range).
    /// </summary>
    [Description("Filter by width in pixels. Examples: 1920 (exact), >=1920 (min), <=1920 (max), 1280..1920 (range).")]
    public string Width { get; set; } = string.Empty;

    /// <summary>
    /// Filter by height in pixels. Examples: 1080 (exact), >=1080 (min), &lt;=1080 (max), 720..1080 (range).
    /// </summary>
    [Description("Filter by height in pixels. Examples: 1080 (exact), >=1080 (min), <=1080 (max), 720..1080 (range).")]
    public string Height { get; set; } = string.Empty;

    /// <summary>
    /// Filter by file size in bytes. Examples: 1000000 (exact), >=500000 (min), &lt;=2000000 (max), 500000..2000000 (range).
    /// </summary>
    [Description("Filter by file size in bytes. Examples: 1000000 (exact), >=500000 (min), <=2000000 (max), 500000..2000000 (range).")]
    public string ByteSize { get; set; } = string.Empty;

    /// <summary>
    /// Filter by uploader user ID (Moderator/Admin only).
    /// </summary>
    [Description("Filter by uploader user ID (Moderator/Admin only).")]
    public long? UploaderId { get; set; }

    /// <summary>
    /// Filter by review status (Moderator/Admin only). Default: Accepted.
    /// </summary>
    [Description("Filter by review status (Moderator/Admin only). Default: Accepted.")]
    public ReviewStatusFilter ReviewStatus { get; set; } = ReviewStatusFilter.Accepted;

    /// <summary>
    /// Filter child entities (tags, artists) by review status (Moderator/Admin only). Default: Accepted.
    /// </summary>
    [Description("Filter child entities (tags, artists) by review status (Moderator/Admin only). Default: Accepted.")]
    public ReviewStatusFilter ChildReviewStatus { get; set; } = ReviewStatusFilter.Accepted;
}
