using System;
using System.Collections.Generic;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Common.Models;

public class ImageDto
{
    public long Id { get; set; }
    public required string PerceptualHash { get; set; }
    public string Extension { get; set; } = string.Empty;
    public string DominantColor { get; set; } = string.Empty;
    public string? Source { get; set; }
    public Artist? Artist { get; set; }
    public long UploaderId { get; set; }
    public DateTime UploadedAt { get; set; }
    public bool IsNsfw { get; set; }
    public bool IsAnimated { get; set; }
    public long Width { get; set; }
    public long Height { get; set; }
    public long ByteSize { get; set; }
    public string Url { get; set; } = string.Empty;
    public List<Tag> Tags { get; set; } = new();
    
    // Dynamic properties
    public long Favorites { get; set; }
    public DateTime? LikedAt { get; set; }
    public DateTime? AddedToAlbumAt { get; set; }
}
