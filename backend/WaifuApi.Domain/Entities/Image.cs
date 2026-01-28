using System;
using System.Collections;
using System.Collections.Generic;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

public class Image
{
    public long Id { get; set; }
    public required BitArray PerceptualHash { get; set; }
    public required string Extension { get; set; }
    public required string DominantColor { get; set; }
    public string? Source { get; set; }
    
    public Artist? Artist { get; set; }
    
    public long UploaderId { get; set; }
    public User Uploader { get; set; } = null!;

    public DateTime UploadedAt { get; set; }
    public bool IsNsfw { get; set; }
    public bool IsAnimated { get; set; }
    public long Width { get; set; }
    public long Height { get; set; }
    public long ByteSize { get; set; }
    public ReviewStatus ReviewStatus { get; set; } = ReviewStatus.Pending;
    public List<Tag> Tags { get; set; } = new();
}
