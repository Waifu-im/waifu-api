using System;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

/// <summary>The banner currently shown to visitors.</summary>
public class BannerDto
{
    public string Message { get; set; } = string.Empty;
    public BannerVariant Variant { get; set; }

    /// <summary>When the banner was last changed. Dismissals are tied to this value, so an edited banner reappears.</summary>
    public DateTime UpdatedAt { get; set; }
}

/// <summary>Full banner configuration, including a disabled banner, for users allowed to manage it.</summary>
public class BannerSettingsDto
{
    public string Message { get; set; } = string.Empty;
    public BannerVariant Variant { get; set; }
    public bool IsEnabled { get; set; }

    /// <summary>Null when the banner has never been configured.</summary>
    public DateTime? UpdatedAt { get; set; }

    public UserMinimalDto? UpdatedBy { get; set; }
}
