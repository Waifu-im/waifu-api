using System;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

/// <summary>
/// Site-wide announcement banner, editable by users meeting Permissions:BannerManagementMinRole. Single-row
/// table: the banner always lives at <see cref="SingletonId"/>.
/// </summary>
public class SiteBanner
{
    public const int SingletonId = 1;

    public int Id { get; set; } = SingletonId;
    public string Message { get; set; } = string.Empty;
    public BannerVariant Variant { get; set; } = BannerVariant.Info;
    public bool IsEnabled { get; set; }

    /// <summary>Last change; the frontend keys dismissals on it so an edited banner shows up again.</summary>
    public DateTime UpdatedAt { get; set; }

    public long? UpdatedById { get; set; }
    public User? UpdatedBy { get; set; }
}
