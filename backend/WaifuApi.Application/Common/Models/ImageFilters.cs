using System.Collections.Generic;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

public class ImageFilters
{
    public NsfwMode IsNsfw { get; set; } = NsfwMode.Safe;
    public List<string> IncludedTags { get; set; } = new();
    public List<string> ExcludedTags { get; set; } = new();
    public List<string> IncludedArtists { get; set; } = new();
    public List<string> ExcludedArtists { get; set; } = new();
    public List<string> IncludedIds { get; set; } = new();
    public List<string> ExcludedIds { get; set; } = new();
    public bool? IsAnimated { get; set; }
    public string OrderBy { get; set; } = string.Empty;
    public string Orientation { get; set; } = string.Empty;
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;
    public long UserId { get; set; }
    public long? AlbumId { get; set; }
}
