using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Artists.GetArtists;
using WaifuApi.Application.Features.Images.GetImages;
using WaifuApi.Application.Features.Tags.GetTags;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Web.Models.Requests;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Content moderation review queue.
/// </summary>
/// <remarks>
/// This controller provides access to items pending moderator review.
/// All user-submitted content (images, tags, artists) starts with a "Pending" status
/// and must be approved by a moderator before becoming publicly visible.
///
/// **Requires:** Moderator role or higher.
///
/// **Workflow:**
/// 1. User submits new content (image, tag, or artist)
/// 2. Content is created with "Pending" review status
/// 3. Moderator retrieves pending items from this controller
/// 4. Moderator approves or rejects using the respective resource endpoints
/// </remarks>
[ApiController]
[Route("review")]
[Authorize(Policy = AuthorizationPolicies.Moderator)]
[Produces("application/json")]
[Tags("Review")]
[ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
[ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
public class ReviewController : ControllerBase
{
    private readonly IMediator _mediator;
    public ReviewController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Get images pending review.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of images that have been uploaded but not yet approved.
    /// Use the PATCH `/images/{id}` endpoint with `reviewStatus: "Accepted"` to approve an image.
    /// </remarks>
    /// <param name="request">Pagination parameters.</param>
    /// <returns>A paginated list of pending images.</returns>
    /// <response code="200">Returns the list of pending images.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    [HttpGet("images")]
    [ProducesResponseType(typeof(PaginatedList<ImageDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<PaginatedList<ImageDto>>> GetPendingImages([FromQuery] PaginationRequest request)
    {
        var query = new GetImagesQuery
        {
            Page = request.Page,
            PageSize = request.PageSize,
            ReviewStatus = ReviewStatus.Pending,
            IsNsfw = NsfwMode.All,
            OrderBy = ImageOrderBy.UploadedAt,
            IsModeratorOrAdmin = true
        };
        var images = await _mediator.Send(query);
        return Ok(images);
    }

    /// <summary>
    /// Get artists pending review.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of artists that have been submitted but not yet approved.
    /// Use the PATCH `/artists/{id}` endpoint with `reviewStatus: "Accepted"` to approve an artist.
    /// </remarks>
    /// <param name="request">Pagination parameters.</param>
    /// <returns>A paginated list of pending artists.</returns>
    /// <response code="200">Returns the list of pending artists.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    [HttpGet("artists")]
    [ProducesResponseType(typeof(PaginatedList<ArtistDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<PaginatedList<ArtistDto>>> GetPendingArtists([FromQuery] PaginationRequest request)
    {
        var query = new GetArtistsQuery
        {
            Page = request.Page,
            PageSize = request.PageSize,
            ReviewStatus = ReviewStatus.Pending,
            IsModeratorOrAdmin = true
        };
        var artists = await _mediator.Send(query);
        return Ok(artists);
    }

    /// <summary>
    /// Get tags pending review.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of tags that have been submitted but not yet approved.
    /// Use the PATCH `/tags/{id}` endpoint with `reviewStatus: "Accepted"` to approve a tag.
    /// </remarks>
    /// <param name="request">Pagination parameters.</param>
    /// <returns>A paginated list of pending tags.</returns>
    /// <response code="200">Returns the list of pending tags.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Moderator required).</response>
    [HttpGet("tags")]
    [ProducesResponseType(typeof(PaginatedList<TagDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<PaginatedList<TagDto>>> GetPendingTags([FromQuery] PaginationRequest request)
    {
        var query = new GetTagsQuery
        {
            Page = request.Page,
            PageSize = request.PageSize,
            ReviewStatus = ReviewStatus.Pending,
            IsModeratorOrAdmin = true
        };
        var tags = await _mediator.Send(query);
        return Ok(tags);
    }
}
