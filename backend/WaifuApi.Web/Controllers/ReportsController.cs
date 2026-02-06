using System.ComponentModel.DataAnnotations;
using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Reports.CreateReport;
using WaifuApi.Application.Features.Reports.GetReports;
using WaifuApi.Application.Features.Reports.ResolveReport;
using WaifuApi.Domain.Entities;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Web.Models.Requests;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Report inappropriate or problematic images.
/// </summary>
/// <remarks>
/// Users can report images that violate guidelines or contain inappropriate content.
/// Reports are reviewed by moderators who can then take appropriate action.
///
/// **Report workflow:**
/// 1. User submits a report with the image ID and optional description
/// 2. Moderator reviews the report and the reported image
/// 3. Moderator takes action (delete image, warn user, etc.)
/// 4. Moderator marks the report as resolved
/// </remarks>
[ApiController]
[Route("reports")]
[Produces("application/json")]
[Tags("Reports")]
public class ReportsController : ControllerBase
{
    private readonly IMediator _mediator;

    public ReportsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Report an image.
    /// </summary>
    /// <remarks>
    /// Submit a report for an image that violates guidelines or contains inappropriate content.
    /// Please provide a clear description of the issue to help moderators review the report efficiently.
    ///
    /// **Common report reasons:**
    /// - Wrong tags (e.g., marked as SFW but contains NSFW content)
    /// - Copyright violation
    /// - Duplicate image
    /// - Low quality or inappropriate content
    /// </remarks>
    /// <param name="request">The report details including image ID and description.</param>
    /// <returns>The created report.</returns>
    /// <response code="201">Report submitted successfully.</response>
    /// <response code="400">Invalid report data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Image not found.</response>
    [Authorize]
    [HttpPost]
    [ProducesResponseType(typeof(Report), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<Report>> Create([FromBody] CreateReportRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var report = await _mediator.Send(new CreateReportCommand(userId, request.ImageId, request.Description));
        return CreatedAtAction(nameof(Get), new { id = report.Id }, report);
    }

    /// <summary>
    /// List all reports.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of image reports. Use the `isResolved` filter to show
    /// only pending reports (false) or completed reports (true).
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="request">Filter and pagination parameters.</param>
    /// <returns>A paginated list of reports.</returns>
    /// <response code="200">Returns the list of reports.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<ReportDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<PaginatedList<ReportDto>>> Get([FromQuery] GetReportsRequest request)
    {
        var reports = await _mediator.Send(new GetReportsQuery(request.IsResolved, request.Page, request.PageSize));
        return Ok(reports);
    }

    /// <summary>
    /// Mark a report as resolved.
    /// </summary>
    /// <remarks>
    /// Mark a report as resolved after taking appropriate action on the reported image.
    /// This removes the report from the pending queue.
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="id">The report ID to resolve.</param>
    /// <response code="204">Report marked as resolved.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    /// <response code="404">Report not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}/resolve")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> Resolve([FromRoute] long id)
    {
        await _mediator.Send(new ResolveReportCommand(id));
        return NoContent();
    }
}

/// <summary>
/// Request model for reporting an image.
/// </summary>
public class CreateReportRequest
{
    /// <summary>
    /// The ID of the image being reported.
    /// </summary>
    [Required]
    public long ImageId { get; set; }

    /// <summary>
    /// A description explaining why this image is being reported.
    /// </summary>
    /// <example>This image is incorrectly tagged as SFW but contains NSFW content.</example>
    public string? Description { get; set; }
}
