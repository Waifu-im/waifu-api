using System.ComponentModel;
using System.Text.Json;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Review.ApproveReviewTask;
using WaifuApi.Application.Features.Review.CancelReviewTask;
using WaifuApi.Application.Features.Review.EditReviewTask;
using WaifuApi.Application.Features.Review.GetReviewTaskById;
using WaifuApi.Application.Features.Review.GetReviewTasks;
using WaifuApi.Application.Features.Review.RejectReviewTask;
using WaifuApi.Application.Features.Review.SubmitEdit;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Web.Services;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// The review queue: new content submissions and proposed edits to existing images, tags and artists.
/// </summary>
/// <remarks>
/// New uploads/tags/artists automatically create a NewContent task; users can also submit Edit tasks proposing
/// changes to existing content. Moderators approve (publishing new content or applying an edit) or reject.
/// Trusted users (per the server's review policy) have their edits applied immediately.
/// </remarks>
[ApiController]
[Route("review/tasks")]
[Produces("application/json")]
[Tags("Review")]
public class ReviewController : ControllerBase
{
    private readonly IMediator _mediator;
    private readonly ICurrentUserService _currentUser;
    private readonly IPermissionService _permissionService;

    public ReviewController(IMediator mediator, ICurrentUserService currentUser, IPermissionService permissionService)
    {
        _mediator = mediator;
        _currentUser = currentUser;
        _permissionService = permissionService;
    }

    /// <summary>
    /// Submit a proposed edit to an existing image, tag or artist.
    /// </summary>
    /// <remarks>
    /// The <c>payload</c> contains only the fields you want to change (its shape depends on <c>targetType</c>).
    /// Unknown fields are rejected. You may only have one pending edit per item.
    /// </remarks>
    /// <response code="201">Edit submitted (or applied immediately for trusted users).</response>
    /// <response code="400">Invalid payload or no effective change.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    /// <response code="404">Target not found.</response>
    /// <response code="409">You already have a pending edit for this item, or the open-edit limit was reached.</response>
    [Authorize]
    [HttpPost]
    [ProducesResponseType(typeof(ReviewTaskDto), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<ReviewTaskDto>> SubmitEdit([FromBody] SubmitEditRequest request)
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.SubmitEditMinRole, "submit edits");

        var payloadJson = request.Payload.ValueKind is JsonValueKind.Undefined or JsonValueKind.Null
            ? string.Empty
            : request.Payload.GetRawText();

        var command = new SubmitEditCommand(
            _currentUser.UserId!.Value,
            _currentUser.UserRole,
            request.TargetType,
            request.TargetId,
            request.Reason,
            payloadJson);

        var task = await _mediator.Send(command);
        return CreatedAtAction(nameof(GetById), new { id = task.Id }, task);
    }

    /// <summary>
    /// List review tasks.
    /// </summary>
    /// <remarks>
    /// Moderators see all tasks and can filter freely. Regular users only ever see their own
    /// (the <c>mine</c> flag is implied for them) — this is their "my submissions / my suggestions" view.
    /// </remarks>
    /// <response code="200">Returns the matching review tasks.</response>
    /// <response code="401">Authentication required.</response>
    [Authorize]
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<ReviewTaskDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    public async Task<ActionResult<PaginatedList<ReviewTaskDto>>> Get([FromQuery] GetReviewTasksRequest request)
    {
        var isModerator = _currentUser.IsModeratorOrAdmin;

        // Non-moderators are always scoped to their own tasks.
        long? submitterScope = (!isModerator || request.Mine == true) ? _currentUser.UserId : null;

        var query = new GetReviewTasksQuery
        {
            Status = request.Status,
            Kind = request.Kind,
            TargetType = request.TargetType,
            TargetId = request.TargetId,
            SubmitterId = submitterScope,
            Page = request.Page,
            PageSize = request.PageSize
        };

        var result = await _mediator.Send(query);
        return Ok(result);
    }

    /// <summary>
    /// Get a single review task by ID.
    /// </summary>
    /// <response code="200">Returns the review task.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Review task not found (or not yours).</response>
    [Authorize]
    [HttpGet("{id:long}")]
    [ProducesResponseType(typeof(ReviewTaskDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<ReviewTaskDto>> GetById([FromRoute] long id)
    {
        long? restrictTo = _currentUser.IsModeratorOrAdmin ? null : _currentUser.UserId;
        var result = await _mediator.Send(new GetReviewTaskByIdQuery(id, restrictTo));
        return Ok(result);
    }

    /// <summary>
    /// Edit a pending task's content — the submitted entity (NewContent) or the proposed change (Edit).
    /// </summary>
    /// <remarks>
    /// Allowed for the original submitter (their own task) or any moderator, and only while the task is Pending.
    /// Content-only: it never changes review status (use Approve/Reject for that). When a moderator edits
    /// someone else's submission, it is flagged so the submitter sees a "modified by a moderator" cue.
    /// </remarks>
    /// <response code="200">The updated task.</response>
    /// <response code="400">Invalid content.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Not your submission.</response>
    /// <response code="404">Task or target not found.</response>
    /// <response code="409">The task has already been resolved.</response>
    [Authorize]
    [HttpPatch("{id:long}")]
    [ProducesResponseType(typeof(ReviewTaskDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<ReviewTaskDto>> Edit([FromRoute] long id, [FromBody] EditReviewTaskRequest request)
    {
        var contentJson = request.Content.ValueKind is JsonValueKind.Undefined or JsonValueKind.Null
            ? "{}"
            : request.Content.GetRawText();

        var task = await _mediator.Send(new EditReviewTaskCommand(
            id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin, contentJson));
        return Ok(task);
    }

    /// <summary>
    /// Approve a review task (publish new content, or apply a proposed edit).
    /// </summary>
    /// <remarks>
    /// **Requires:** Moderator role or higher. For edits, the proposed values simply overwrite the current ones.
    /// </remarks>
    /// <param name="id">The review task ID.</param>
    /// <response code="204">Task approved.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    /// <response code="404">Task or its target not found.</response>
    /// <response code="409">The task was already resolved.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}/approve")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Approve([FromRoute] long id)
    {
        await _mediator.Send(new ApproveReviewTaskCommand(id, _currentUser.UserId!.Value));
        return NoContent();
    }

    /// <summary>
    /// Reject a review task (delete the submitted content, or decline a proposed edit).
    /// </summary>
    /// <remarks>**Requires:** Moderator role or higher.</remarks>
    /// <response code="204">Task rejected.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    /// <response code="404">Task not found.</response>
    /// <response code="409">The task was already resolved.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}/reject")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Reject([FromRoute] long id, [FromBody] RejectReviewTaskRequest request)
    {
        await _mediator.Send(new RejectReviewTaskCommand(id, _currentUser.UserId!.Value, request.Reason));
        return NoContent();
    }

    /// <summary>
    /// Cancel a review task. The original submitter (while pending) or a moderator may cancel.
    /// Cancelling a new submission withdraws (deletes) it.
    /// </summary>
    /// <response code="204">Task cancelled.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">You can only cancel your own submissions.</response>
    /// <response code="404">Task not found.</response>
    /// <response code="409">The task was already resolved.</response>
    [Authorize]
    [HttpPatch("{id:long}/cancel")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<IActionResult> Cancel([FromRoute] long id)
    {
        await _mediator.Send(new CancelReviewTaskCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin));
        return NoContent();
    }
}

/// <summary>Request model for submitting an edit.</summary>
public class SubmitEditRequest
{
    /// <summary>The kind of entity to edit.</summary>
    public ReviewableContentType TargetType { get; set; }

    /// <summary>The id of the image, tag or artist to edit.</summary>
    public long TargetId { get; set; }

    /// <summary>Optional explanation of why the change should be made.</summary>
    public string? Reason { get; set; }

    /// <summary>
    /// The proposed changes — only the fields you want to change, e.g.
    /// <c>{ "name": "New name" }</c> for a tag, or <c>{ "addTagSlugs": ["maid"], "isNsfw": true }</c> for an image.
    /// </summary>
    public JsonElement Payload { get; set; }
}

/// <summary>Request model for listing review tasks.</summary>
public class GetReviewTasksRequest
{
    /// <summary>Filter by status (e.g. Pending). Default: all.</summary>
    [Description("Filter by status (e.g. Pending). Default: all.")]
    public ReviewTaskStatus? Status { get; set; }

    /// <summary>Filter by kind (NewContent or Edit).</summary>
    [Description("Filter by kind (NewContent or Edit).")]
    public ReviewTaskKind? Kind { get; set; }

    /// <summary>Filter by target type.</summary>
    [Description("Filter by target type.")]
    public ReviewableContentType? TargetType { get; set; }

    /// <summary>Filter by a specific target id.</summary>
    [Description("Filter by a specific target id.")]
    public long? TargetId { get; set; }

    /// <summary>Moderators only: restrict the results to your own submissions.</summary>
    [Description("Moderators only: restrict the results to your own submissions.")]
    public bool? Mine { get; set; }

    /// <summary>Page number for pagination. Default: 1.</summary>
    [Description("Page number for pagination. Default: 1.")]
    public int Page { get; set; } = 1;

    /// <summary>Number of results per page.</summary>
    [Description("Number of results per page.")]
    public int PageSize { get; set; }
}

/// <summary>Request model for rejecting a review task.</summary>
public class RejectReviewTaskRequest
{
    /// <summary>Optional reason shown to the submitter.</summary>
    public string? Reason { get; set; }
}

/// <summary>Request model for editing a pending task's content.</summary>
public class EditReviewTaskRequest
{
    /// <summary>
    /// The new content. For NewContent: the full editable fields of the entity (e.g. image
    /// <c>{ "source", "isNsfw", "tags": [], "artists": [] }</c>). For Edit: the proposed-change payload.
    /// </summary>
    public JsonElement Content { get; set; }
}
