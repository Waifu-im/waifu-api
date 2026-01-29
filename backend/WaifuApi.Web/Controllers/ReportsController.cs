using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Reports.CreateReport;
using WaifuApi.Application.Features.Reports.GetReports;
using WaifuApi.Application.Features.Reports.ResolveReport;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("reports")]
public class ReportsController : ControllerBase
{
    private readonly IMediator _mediator;

    public ReportsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Creates a new report for an image.
    /// </summary>
    /// <param name="request">The report details.</param>
    /// <returns>The created report.</returns>
    [Authorize]
    [HttpPost]
    public async Task<ActionResult<Report>> Create([FromBody] CreateReportRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var report = await _mediator.Send(new CreateReportCommand(userId, request.ImageId, request.Description));
        return CreatedAtAction(nameof(Get), new { id = report.Id }, report);
    }

    /// <summary>
    /// Retrieves a list of reports (Moderator only).
    /// </summary>
    /// <param name="isResolved">Optional filter by resolution status.</param>
    /// <param name="page">Page number.</param>
    /// <param name="pageSize">Page size.</param>
    /// <returns>A list of reports.</returns>
    [Authorize(Policy = "Moderator")]
    [HttpGet]
    public async Task<ActionResult<PaginatedList<ReportDto>>> Get([FromQuery] bool? isResolved, [FromQuery] int page = 1, [FromQuery] int pageSize = 20)
    {
        var reports = await _mediator.Send(new GetReportsQuery(isResolved, page, pageSize));
        return Ok(reports);
    }

    /// <summary>
    /// Marks a report as resolved (Moderator only).
    /// </summary>
    /// <param name="id">The ID of the report to resolve.</param>
    [Authorize(Policy = "Moderator")]
    [HttpPut("{id:long}/resolve")]
    public async Task<IActionResult> Resolve([FromRoute] long id)
    {
        await _mediator.Send(new ResolveReportCommand(id));
        return NoContent();
    }
}

public class CreateReportRequest
{
    public long ImageId { get; set; }
    public string? Description { get; set; }
}
