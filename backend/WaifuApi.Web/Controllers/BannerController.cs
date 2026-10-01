using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Banner.GetActiveBanner;
using WaifuApi.Application.Features.Banner.GetBannerSettings;
using WaifuApi.Application.Features.Banner.UpdateBanner;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Models;
using WaifuApi.Web.Services;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Site-wide announcement banner.
/// </summary>
/// <remarks>
/// A single banner shown at the top of the website. Viewing/editing its settings requires the role configured in
/// Permissions:BannerManagementMinRole (Moderator by default).
/// Visitors can dismiss it; editing the banner makes it show up again.
/// </remarks>
[ApiController]
[Route("banner")]
[Produces("application/json")]
[Tags("Banner")]
public class BannerController : ControllerBase
{
    private const string ManageAction = "manage the site banner";

    private readonly IMediator _mediator;
    private readonly IPermissionService _permissionService;

    public BannerController(IMediator mediator, IPermissionService permissionService)
    {
        _mediator = mediator;
        _permissionService = permissionService;
    }

    /// <summary>
    /// Get the active banner.
    /// </summary>
    /// <remarks>
    /// Returns the banner currently shown to visitors, or no content when there is none.
    /// </remarks>
    /// <returns>The active banner.</returns>
    /// <response code="200">Returns the active banner.</response>
    /// <response code="204">No banner is currently active.</response>
    [HttpGet]
    [ProducesResponseType(typeof(BannerDto), StatusCodes.Status200OK)]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    public async Task<ActionResult<BannerDto>> Get()
    {
        var banner = await _mediator.Send(new GetActiveBannerQuery());
        return banner == null ? NoContent() : Ok(banner);
    }

    /// <summary>
    /// Get the banner settings.
    /// </summary>
    /// <remarks>
    /// Returns the full banner configuration, including a disabled banner.
    ///
    /// **Requires:** Moderator role or higher by default (configurable by the API administrator).
    /// </remarks>
    /// <returns>The banner settings.</returns>
    /// <response code="200">Returns the banner settings.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    [Authorize]
    [HttpGet("settings")]
    [ProducesResponseType(typeof(BannerSettingsDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<BannerSettingsDto>> GetSettings()
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.BannerManagementMinRole, ManageAction, Role.Moderator);
        return Ok(await _mediator.Send(new GetBannerSettingsQuery()));
    }

    /// <summary>
    /// Update the banner.
    /// </summary>
    /// <remarks>
    /// Replaces the banner message, style and visibility.
    ///
    /// **Requires:** Moderator role or higher by default (configurable by the API administrator).
    /// </remarks>
    /// <param name="request">The new banner settings.</param>
    /// <returns>The updated banner settings.</returns>
    /// <response code="200">Banner updated.</response>
    /// <response code="400">Invalid banner data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    [Authorize]
    [HttpPut("settings")]
    [ProducesResponseType(typeof(BannerSettingsDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<BannerSettingsDto>> UpdateSettings([FromBody] UpdateBannerRequest request)
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.BannerManagementMinRole, ManageAction, Role.Moderator);
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var result = await _mediator.Send(new UpdateBannerCommand(userId, request.Message ?? string.Empty, request.Variant, request.IsEnabled));
        return Ok(result);
    }
}

/// <summary>
/// Request model for updating the site banner.
/// </summary>
public class UpdateBannerRequest
{
    /// <summary>
    /// The message shown in the banner (plain text, max 500 characters). Required when the banner is enabled.
    /// </summary>
    public string? Message { get; set; }

    /// <summary>
    /// Visual style of the banner.
    /// </summary>
    public BannerVariant Variant { get; set; } = BannerVariant.Info;

    /// <summary>
    /// Whether the banner is shown to visitors.
    /// </summary>
    public bool IsEnabled { get; set; }
}
