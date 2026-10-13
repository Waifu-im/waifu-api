using System;
using System.Collections;
using System.Collections.Generic;
using System.ComponentModel.DataAnnotations.Schema;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

public class Image
{
    public long Id { get; set; }
    public required BitArray PerceptualHash { get; set; }

    /// <summary>
    /// The fingerprint as the C# implementation produced it, preserved so the Go service's
    /// column can be filled without destroying the history.
    /// </summary>
    /// <remarks>
    /// Frozen after the one-off backfill. The C# service never wrote a consistent value —
    /// ImageSharp's resampler branches on AVX2, so roughly half these rows came from a
    /// different algorithm than the other half — which is why they are kept rather than
    /// trusted. Nullable: rows uploaded after the switch have no legacy value.
    /// See backend/go/internal/images/phash/MIGRATION.md.
    /// </remarks>
    public BitArray? PerceptualHashLegacy { get; set; }
    public required string Extension { get; set; }
    public required string DominantColor { get; set; }
    public string? Source { get; set; }
    
    public List<Artist> Artists { get; set; } = new();
    
    public long? UploaderId { get; set; }
    public User? Uploader { get; set; }

    public DateTime UploadedAt { get; set; }
    public bool IsNsfw { get; set; }
    public bool IsAnimated { get; set; }
    public long Width { get; set; }
    public long Height { get; set; }
    public long ByteSize { get; set; }
    public ReviewStatus ReviewStatus { get; set; } = ReviewStatus.Pending;
    public List<Tag> Tags { get; set; } = new();
}
