using System.Collections.Generic;
using System.ComponentModel;
using System.ComponentModel.DataAnnotations;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Tags.GetTags;
using WaifuApi.Application.Features.Tags.CreateTag;
using WaifuApi.Application.Features.Tags.DeleteTag;
using WaifuApi.Application.Features.Tags.GetTagById;
using WaifuApi.Application.Features.Tags.GetTagBySlug;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Web.Services;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Manage image tags for categorization and search.
/// </summary>
/// <remarks>
/// Tags are used to categorize images and enable powerful search filtering.
/// Each tag has a unique slug (URL-friendly identifier) and can have a description.
/// New tags created by users require moderator approval before being publicly visible.
/// </remarks>
[ApiController]
[Route("tags")]
[Produces("application/json")]
[Tags("Tags")]
public class TagsController : ControllerBase
{
    private readonly IMediator _mediator;
    private readonly IPermissionService _permissionService;
    private readonly ICurrentUserService _currentUser;

    public TagsController(IMediator mediator, IPermissionService permissionService, ICurrentUserService currentUser)
    {
        _mediator = mediator;
        _permissionService = permissionService;
        _currentUser = currentUser;
    }

    /// <summary>
    /// List all tags with optional filtering.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of tags. Use the `Name` parameter to search by tag name.
    /// By default, only approved tags are returned.
    /// </remarks>
    /// <param name="request">Search and pagination parameters.</param>
    /// <returns>A paginated list of tags.</returns>
    /// <response code="200">Returns the list of tags.</response>
    /// <response code="400">Invalid query parameters.</response>
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<TagDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<PaginatedList<TagDto>>> Get([FromQuery] GetTagsRequest request)
    {
        var query = new GetTagsQuery
        {
            Name = request.Name,
            IncludedIds = request.IncludedIds,
            IncludedSlugs = request.IncludedSlugs,
            Page = request.Page,
            PageSize = request.PageSize,
            UserRole = _currentUser.UserRole,
            UserId = _currentUser.UserId,
            ReviewStatus = request.ReviewStatus,
            IncludeMyPending = request.IncludeMyPending
        };

        var tags = await _mediator.Send(query);
        return Ok(tags);
    }

    /// <summary>
    /// Get a tag by its ID.
    /// </summary>
    /// <param name="id">The unique identifier of the tag.</param>
    /// <returns>The tag details.</returns>
    /// <response code="200">Returns the tag.</response>
    /// <response code="404">Tag not found.</response>
    [HttpGet("{id:long}")]
    [ProducesResponseType(typeof(TagDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<TagDto>> GetById([FromRoute] long id)
    {
        var tag = await _mediator.Send(new GetTagByIdQuery(id));
        return Ok(tag);
    }

    /// <summary>
    /// Get a tag by its slug.
    /// </summary>
    /// <remarks>
    /// Slugs are URL-friendly identifiers (e.g., "waifu", "marin-kitagawa").
    /// This endpoint is useful when you know the tag slug but not its ID.
    /// </remarks>
    /// <param name="slug">The URL-friendly slug of the tag.</param>
    /// <returns>The tag details.</returns>
    /// <response code="200">Returns the tag.</response>
    /// <response code="404">Tag not found.</response>
    [HttpGet("by-slug/{slug}")]
    [ProducesResponseType(typeof(TagDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<TagDto>> GetBySlug([FromRoute] string slug)
    {
        var tag = await _mediator.Send(new GetTagBySlugQuery(slug));
        return Ok(tag);
    }

    /// <summary>
    /// Create a new tag.
    /// </summary>
    /// <remarks>
    /// Creates a new tag for categorizing images.
    /// The tag will be created with "Pending" review status and must be approved by a moderator.
    ///
    /// If no slug is provided, one will be automatically generated from the name.
    /// </remarks>
    /// <param name="request">The tag details.</param>
    /// <returns>The created tag.</returns>
    /// <response code="201">Tag created successfully.</response>
    /// <response code="400">Invalid tag data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (role restriction configured by API administrator).</response>
    /// <response code="409">A tag with this name or slug already exists.</response>
    [Authorize]
    [HttpPost]
    [ProducesResponseType(typeof(TagDto), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<TagDto>> Create([FromBody] CreateTagRequest request)
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.TagCreationMinRole, "create tags");
        var tag = await _mediator.Send(new CreateTagCommand(request.Name, request.Description, request.Slug, _currentUser.UserId, _currentUser.UserRole));
        return CreatedAtAction(nameof(Get), new { id = tag.Id }, tag);
    }

    /// <summary>
    /// Update an existing tag.
    /// </summary>
    /// <remarks>
    /// Update a tag's name, description, slug, or review status.
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="id">The tag ID to update.</param>
    /// <param name="request">The fields to update.</param>
    /// <returns>The updated tag.</returns>
    /// <response code="200">Tag updated successfully.</response>
    /// <response code="400">Invalid update data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    /// <response code="404">Tag not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}")]
    [ProducesResponseType(typeof(TagDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<TagDto>> Update([FromRoute] long id, [FromBody] UpdateTagRequest request)
    {
        var command = new UpdateTagCommand(id, request.Name, request.Description, request.Slug, request.ReviewStatus, request.CreatorId);
        var tag = await _mediator.Send(command);
        return Ok(tag);
    }

    /// <summary>
    /// Delete a tag permanently.
    /// </summary>
    /// <remarks>
    /// Permanently removes a tag. Images that had this tag will no longer be associated with it.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="id">The tag ID to delete.</param>
    /// <response code="204">Tag deleted successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    /// <response code="404">Tag not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [HttpDelete("{id:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        await _mediator.Send(new DeleteTagCommand(id));
        return NoContent();
    }
}

/// <summary>
/// Request model for creating a new tag.
/// </summary>
/// <summary>
/// Request model for searching and filtering tags.
/// </summary>
public class GetTagsRequest
{
    /// <summary>
    /// Filter by tag name (partial match, case-insensitive). Example: ?Name=Waifu
    /// </summary>
    [Description("Filter by tag name (partial match, case-insensitive). Example: ?Name=Waifu")]
    public string? Name { get; set; }

    /// <summary>
    /// Filter by specific tag IDs (exact match). Example: ?IncludedIds=12&amp;IncludedIds=13
    /// </summary>
    [Description("Filter by specific tag IDs (exact match). Example: ?IncludedIds=12&IncludedIds=13")]
    public List<long> IncludedIds { get; set; } = new();

    /// <summary>
    /// Filter by specific tag slugs (exact match). Example: ?IncludedSlugs=waifu&amp;IncludedSlugs=maid
    /// </summary>
    [Description("Filter by specific tag slugs (exact match). Example: ?IncludedSlugs=waifu&IncludedSlugs=maid")]
    public List<string> IncludedSlugs { get; set; } = new();

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

    /// <summary>
    /// Filter by review status (Moderator/Admin only). Default: Accepted.
    /// </summary>
    [Description("Filter by review status (Moderator/Admin only). Default: Accepted.")]
    public ReviewStatusFilter ReviewStatus { get; set; } = ReviewStatusFilter.Accepted;

    /// <summary>
    /// Also include your own pending (not-yet-approved) tags alongside the accepted ones. Lets a logged-in user
    /// see tags they created that are still awaiting review — e.g. to attach them in the upload/edit picker.
    /// </summary>
    [Description("Also include your own pending tags alongside accepted ones.")]
    public bool IncludeMyPending { get; set; }
}

/// <summary>
/// Request model for creating a new tag.
/// </summary>
public class CreateTagRequest
{
    /// <summary>
    /// The display name of the tag (e.g., "Waifu").
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// A description explaining what this tag represents.
    /// </summary>
    public string Description { get; set; } = string.Empty;

    /// <summary>
    /// Optional URL-friendly slug. If not provided, will be auto-generated from the name.
    /// </summary>
    /// <example>marin-kitagawa</example>
    public string? Slug { get; set; }
}

/// <summary>
/// Request model for updating an existing tag.
/// </summary>
public class UpdateTagRequest
{
    /// <summary>
    /// The updated display name.
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// The updated description.
    /// </summary>
    public string Description { get; set; } = string.Empty;

    /// <summary>
    /// The updated slug.
    /// </summary>
    public string? Slug { get; set; }

    /// <summary>
    /// The review status (Moderator only). Set to Accepted to approve a pending tag.
    /// </summary>
    public ReviewStatus? ReviewStatus { get; set; }

    /// <summary>
    /// The creator user ID. Set to reassign who created this tag.
    /// </summary>
    public long? CreatorId { get; set; }
}
