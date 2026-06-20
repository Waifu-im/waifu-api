using System.ComponentModel;
using System.ComponentModel.DataAnnotations;
using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Reports.CreateReport;
using WaifuApi.Application.Features.Reports.GetReports;
using WaifuApi.Application.Features.Reports.SetReportStatus;
using WaifuApi.Application.Features.Reports.UpdateReport;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;

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
    private readonly ICurrentUserService _currentUser;

    public ReportsController(IMediator mediator, ICurrentUserService currentUser)
    {
        _mediator = mediator;
        _currentUser = currentUser;
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
    /// <response code="409">You already have a pending report for this image.</response>
    [Authorize]
    [HttpPost]
    [ProducesResponseType(typeof(Report), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<Report>> Create([FromBody] CreateReportRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var report = await _mediator.Send(new CreateReportCommand(userId, request.ImageId, request.Description));
        return CreatedAtAction(nameof(Get), new { id = report.Id }, report);
    }

    /// <summary>
    /// List reports.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of image reports. Use the `Status` filter to show a single lifecycle status
    /// (Pending, Resolved, Rejected, Cancelled).
    ///
    /// Moderators see all reports; regular users only ever see their own. A moderator can pass `mine=true`
    /// to see just their own reports (their "My Reports" view).
    /// </remarks>
    /// <param name="request">Filter and pagination parameters.</param>
    /// <returns>A paginated list of reports.</returns>
    /// <response code="200">Returns the list of reports.</response>
    /// <response code="401">Authentication required.</response>
    [Authorize]
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<ReportDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    public async Task<ActionResult<PaginatedList<ReportDto>>> Get([FromQuery] GetReportsRequest request)
    {
        var isMod = _currentUser.IsModeratorOrAdmin;
        // Non-moderators are always scoped to their own reports; moderators see all unless they ask for theirs.
        long? scope = (!isMod || request.Mine == true) ? _currentUser.UserId : null;
        var reports = await _mediator.Send(new GetReportsQuery(request.Status, request.Page, request.PageSize, scope, _currentUser.UserRole));
        return Ok(reports);
    }

    /// <summary>
    /// Edit one of your reports.
    /// </summary>
    /// <remarks>
    /// Update the description of a report you submitted. Only allowed while the report is still open
    /// (not yet resolved). Moderators may edit any open report.
    /// </remarks>
    /// <param name="id">The report ID to edit.</param>
    /// <param name="request">The updated description.</param>
    /// <returns>The updated report.</returns>
    /// <response code="200">Report updated successfully.</response>
    /// <response code="400">Invalid description.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Not your report.</response>
    /// <response code="404">Report not found.</response>
    /// <response code="409">Report already resolved.</response>
    [Authorize]
    [HttpPatch("{id:long}")]
    [ProducesResponseType(typeof(ReportDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<ReportDto>> Update([FromRoute] long id, [FromBody] UpdateReportRequest request)
    {
        var report = await _mediator.Send(new UpdateReportCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin, request.Description ?? string.Empty));
        return Ok(report);
    }

    /// <summary>
    /// Validate a report (mark it resolved).
    /// </summary>
    /// <remarks>The report was legitimate and has been handled. **Requires:** Moderator role or higher.</remarks>
    /// <param name="id">The report ID.</param>
    /// <param name="request">Optional moderator note shown to the reporter.</param>
    /// <response code="204">Report resolved.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    /// <response code="404">Report not found.</response>
    /// <response code="409">Report already handled.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}/validate")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Validate([FromRoute] long id, [FromBody] ReportActionRequest? request = null)
    {
        await _mediator.Send(new SetReportStatusCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin, ReportStatus.Resolved, request?.Reason));
        return NoContent();
    }

    /// <summary>
    /// Reject a report (dismiss it).
    /// </summary>
    /// <remarks>The report was not valid and has been dismissed. **Requires:** Moderator role or higher.</remarks>
    /// <param name="id">The report ID.</param>
    /// <param name="request">Optional moderator note shown to the reporter.</param>
    /// <response code="204">Report rejected.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    /// <response code="404">Report not found.</response>
    /// <response code="409">Report already handled.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}/reject")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Reject([FromRoute] long id, [FromBody] ReportActionRequest? request = null)
    {
        await _mediator.Send(new SetReportStatusCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin, ReportStatus.Rejected, request?.Reason));
        return NoContent();
    }

    /// <summary>
    /// Cancel (withdraw) a report.
    /// </summary>
    /// <remarks>The report's author withdraws it. Moderators may cancel any report. Only pending reports can be cancelled.</remarks>
    /// <param name="id">The report ID.</param>
    /// <response code="204">Report cancelled.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Not your report.</response>
    /// <response code="404">Report not found.</response>
    /// <response code="409">Report already handled.</response>
    [Authorize]
    [HttpPatch("{id:long}/cancel")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Cancel([FromRoute] long id)
    {
        await _mediator.Send(new SetReportStatusCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin, ReportStatus.Cancelled));
        return NoContent();
    }
}

/// <summary>
/// Request model for reporting an image.
/// </summary>
/// <summary>
/// Request model for listing reports.
/// </summary>
public class GetReportsRequest
{
    /// <summary>
    /// Filter by lifecycle status (Pending, Resolved, Rejected, Cancelled).
    /// </summary>
    [Description("Filter by lifecycle status (Pending, Resolved, Rejected, Cancelled).")]
    public ReportStatus? Status { get; set; }

    /// <summary>
    /// Moderators only: return just your own reports instead of everyone's. (Non-moderators are always scoped to their own.)
    /// </summary>
    [Description("Moderators only: return just your own reports instead of everyone's.")]
    public bool? Mine { get; set; }

    /// <summary>
    /// Page number for pagination. Default: 1.
    /// </summary>
    [Description("Page number for pagination. Default: 1.")]
    public int Page { get; set; } = 1;

    /// <summary>
    /// Number of results per page.
    /// </summary>
    [Description("Number of results per page.")]
    public int PageSize { get; set; }
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

/// <summary>
/// Request model for editing a report's description.
/// </summary>
public class UpdateReportRequest
{
    /// <summary>
    /// The updated description explaining why this image is being reported.
    /// </summary>
    [Required]
    public string? Description { get; set; }
}

/// <summary>
/// Request model for answering a report (validate/reject) with an optional moderator note.
/// </summary>
public class ReportActionRequest
{
    /// <summary>
    /// Optional note from the moderator shown to the reporter (e.g. why the report was rejected).
    /// </summary>
    public string? Reason { get; set; }
}
