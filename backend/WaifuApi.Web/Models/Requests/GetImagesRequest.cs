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
    /// Only include images with ALL these tags (AND logic). Example: ?includedTags=waifu&amp;includedTags=blonde-hair
    /// </summary>
    [Description("Only include images with ALL these tags (AND logic). Example: ?includedTags=waifu&includedTags=blonde-hair")]
    public List<string> IncludedTags { get; set; } = new();

    /// <summary>
    /// Exclude images with ANY of these tags. Example: ?excludedTags=maid&amp;excludedTags=school-uniform
    /// </summary>
    [Description("Exclude images with ANY of these tags. Example: ?excludedTags=maid&excludedTags=school-uniform")]
    public List<string> ExcludedTags { get; set; } = new();

    /// <summary>
    /// Only include images by these artists (by ID, AND logic). Example: ?includedArtists=123&amp;includedArtists=456
    /// </summary>
    [Description("Only include images by these artists (by ID, AND logic). Example: ?includedArtists=123&includedArtists=456")]
    public List<string> IncludedArtists { get; set; } = new();

    /// <summary>
    /// Exclude images by these artists (by ID). Example: ?excludedArtists=789
    /// </summary>
    [Description("Exclude images by these artists (by ID). Example: ?excludedArtists=789")]
    public List<string> ExcludedArtists { get; set; } = new();

    /// <summary>
    /// Only include these specific image IDs. Example: ?includedIds=1001&amp;includedIds=1002
    /// </summary>
    [Description("Only include these specific image IDs. Example: ?includedIds=1001&includedIds=1002")]
    public List<string> IncludedIds { get; set; } = new();

    /// <summary>
    /// Exclude these specific image IDs. Example: ?excludedIds=999
    /// </summary>
    [Description("Exclude these specific image IDs. Example: ?excludedIds=999")]
    public List<string> ExcludedIds { get; set; } = new();

    /// <summary>
    /// Filter by animation status. Default: all.
    /// </summary>
    [Description("Filter by animation status. Default: all.")]
    public AnimatedMode IsAnimated { get; set; } = AnimatedMode.All;

    /// <summary>
    /// Sort order. Default: random. Note: addedToAlbum only works on album images endpoint.
    /// </summary>
    [Description("Sort order. Default: random. Note: addedToAlbum only works on album images endpoint.")]
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
}
