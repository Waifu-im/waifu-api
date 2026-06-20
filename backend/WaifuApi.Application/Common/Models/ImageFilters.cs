using System.Collections.Generic;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

public class ImageFilters
{
    public NsfwMode IsNsfw { get; set; } = NsfwMode.False;
    public List<string> IncludedTags { get; set; } = new();
    public List<string> ExcludedTags { get; set; } = new();
    public List<string> IncludedArtists { get; set; } = new();
    public List<string> ExcludedArtists { get; set; } = new();
    public List<string> IncludedIds { get; set; } = new();
    public List<string> ExcludedIds { get; set; } = new();
    public AnimatedMode IsAnimated { get; set; } = AnimatedMode.All;
    public ImageOrderBy OrderBy { get; set; } = ImageOrderBy.Random;
    public Orientation Orientation { get; set; } = Orientation.All;
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;
    public long? UserId { get; set; }
    public long? AlbumId { get; set; }
    public ReviewStatusFilter ReviewStatus { get; set; } = ReviewStatusFilter.Accepted;
    public long? UploaderId { get; set; }

    /// <summary>Union the caller's own pending uploads (UploaderId == UserId) onto an otherwise-accepted result.</summary>
    public bool IncludeMyPending { get; set; }
}
