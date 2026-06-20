using System;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Services;

/// <inheritdoc />
public class ReviewPolicy : IReviewPolicy
{
    // When no bypass role is configured for a content type, trusted users (and above) skip review.
    private const Role DefaultBypassMinRole = Role.TrustedUser;

    private readonly IConfiguration _configuration;

    public ReviewPolicy(IConfiguration configuration)
    {
        _configuration = configuration;
    }

    public bool RequiresReviewForNewContent(ReviewableContentType type, Role? userRole)
    {
        var (masterKey, bypassKey) = type switch
        {
            ReviewableContentType.Image => (ConfigurationKeys.Review.RequireImageReview, ConfigurationKeys.Review.ImageReviewBypassMinRole),
            ReviewableContentType.Tag => (ConfigurationKeys.Review.RequireTagReview, ConfigurationKeys.Review.TagReviewBypassMinRole),
            ReviewableContentType.Artist => (ConfigurationKeys.Review.RequireArtistReview, ConfigurationKeys.Review.ArtistReviewBypassMinRole),
            _ => throw new ArgumentOutOfRangeException(nameof(type))
        };

        return RequiresReview(masterKey, bypassKey, userRole);
    }

    public bool RequiresReviewForEditRequest(ReviewableContentType type, Role? userRole)
    {
        // Edit requests share a single master switch but have their OWN per-type bypass thresholds,
        // configured independently from the creation/upload bypass.
        var bypassKey = type switch
        {
            ReviewableContentType.Image => ConfigurationKeys.Review.ImageEditReviewBypassMinRole,
            ReviewableContentType.Tag => ConfigurationKeys.Review.TagEditReviewBypassMinRole,
            ReviewableContentType.Artist => ConfigurationKeys.Review.ArtistEditReviewBypassMinRole,
            _ => throw new ArgumentOutOfRangeException(nameof(type))
        };

        return RequiresReview(ConfigurationKeys.Review.RequireEditRequestReview, bypassKey, userRole);
    }

    private bool RequiresReview(string masterSwitchKey, string bypassKey, Role? userRole)
    {
        // Master switch off => nothing of this kind is ever reviewed.
        var requireReview = bool.Parse(_configuration[masterSwitchKey] ?? "true");
        if (!requireReview)
        {
            return false;
        }

        // Otherwise, review is required unless the user is trusted enough to bypass it.
        return !CanBypassReview(_configuration[bypassKey], userRole);
    }

    private static bool CanBypassReview(string? configuredMinRole, Role? userRole)
    {
        if (userRole == null)
        {
            return false;
        }

        // Explicitly disabled => no one bypasses (every submission is reviewed).
        if (string.Equals(configuredMinRole, "None", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        var minRole = DefaultBypassMinRole;
        if (!string.IsNullOrWhiteSpace(configuredMinRole) && Enum.TryParse<Role>(configuredMinRole, ignoreCase: true, out var parsed))
        {
            minRole = parsed;
        }

        return userRole.Value >= minRole;
    }
}
