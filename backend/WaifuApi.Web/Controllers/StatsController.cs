using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.Stats.GetAdminStats;
using WaifuApi.Application.Features.Stats.GetPublicStats;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// API usage statistics.
/// </summary>
[ApiController]
[Route("stats")]
[Produces("application/json")]
[Tags("Statistics")]
public class StatsController : ControllerBase
{
    private readonly IMediator _mediator;

    public StatsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Get public statistics.
    /// </summary>
    /// <remarks>
    /// Returns: total requests, total images, total tags, total artists.
    /// </remarks>
    /// <returns>Public statistics.</returns>
    /// <response code="200">Returns the public statistics.</response>
    [HttpGet("public")]
    [ProducesResponseType(typeof(PublicStatsDto), StatusCodes.Status200OK)]
    public async Task<ActionResult<PublicStatsDto>> GetPublicStats()
    {
        return Ok(await _mediator.Send(new GetPublicStatsQuery()));
    }

    /// <summary>
    /// Get admin statistics.
    /// </summary>
    /// <remarks>
    /// Returns detailed request statistics: requests today, daily history (last year),
    /// top users by requests, authenticated vs unauthenticated breakdown,
    /// API key vs JWT breakdown, plus all public stats.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <returns>Admin statistics.</returns>
    /// <response code="200">Returns the admin statistics.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    [HttpGet("admin")]
    [Authorize(Roles = nameof(Role.Admin))]
    [ProducesResponseType(typeof(AdminStatsDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<AdminStatsDto>> GetAdminStats()
    {
        return Ok(await _mediator.Send(new GetAdminStatsQuery()));
    }
}
