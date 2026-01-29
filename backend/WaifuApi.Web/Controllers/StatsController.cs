using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.Stats.GetAdminStats;
using WaifuApi.Application.Features.Stats.GetPublicStats;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("stats")]
public class StatsController : ControllerBase
{
    private readonly IMediator _mediator;

    public StatsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    [HttpGet("public")]
    public async Task<ActionResult<PublicStatsDto>> GetPublicStats()
    {
        return Ok(await _mediator.Send(new GetPublicStatsQuery()));
    }

    [HttpGet("admin")]
    [Authorize(Roles = nameof(Role.Admin))]
    public async Task<ActionResult<AdminStatsDto>> GetAdminStats()
    {
        return Ok(await _mediator.Send(new GetAdminStatsQuery()));
    }
}