using System.ComponentModel.DataAnnotations;
using System.Security.Claims;
using Mediator;
using WaifuApi.Application.Common.Exceptions;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Albums.AddImageToAlbum;
using WaifuApi.Application.Features.Albums.CreateAlbum;
using WaifuApi.Application.Features.Albums.DeleteAlbum;
using WaifuApi.Application.Features.Albums.GetAlbumById;
using WaifuApi.Application.Features.Albums.GetAlbums;
using WaifuApi.Application.Features.Albums.RemoveImageFromAlbum;
using WaifuApi.Application.Features.Albums.UpdateAlbum;
using WaifuApi.Application.Features.Images.GetImages;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Models;
using WaifuApi.Web.Models.Requests;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Manage user image albums (collections).
/// </summary>
/// <remarks>
/// Albums allow users to organize their favorite images into collections.
/// Each user has a default "Favorites" album that cannot be deleted.
///
/// **Route parameters:**
/// - `userId`: Use `"me"` for the current user, or a numeric ID
/// - `albumId`: Use `"favorites"` for the default album, or a numeric ID
/// </remarks>
[Authorize]
[ApiController]
[Route("users/{userId}/albums")]
[Produces("application/json")]
[Tags("Albums")]
public class AlbumsController : ControllerBase
{
    private readonly IMediator _mediator;
    private readonly ICurrentUserService _currentUserService;

    public AlbumsController(IMediator mediator, ICurrentUserService currentUserService)
    {
        _mediator = mediator;
        _currentUserService = currentUserService;
    }

    /// <summary>
    /// List albums for a user.
    /// </summary>
    /// <remarks>
    /// Returns all albums for the specified user. The default "Favorites" album is always listed first.
    /// </remarks>
    /// <param name="userId">The user ID or "me" for the current user.</param>
    /// <param name="request">Pagination parameters.</param>
    /// <returns>A paginated list of albums.</returns>
    /// <response code="200">Returns the list of albums.</response>
    /// <response code="404">User not found.</response>
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<AlbumDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<PaginatedList<AlbumDto>>> GetAlbums([FromRoute] string userId, [FromQuery] PaginationRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        if (!CanAccess(resolvedUserId)) return NotFound();

        var query = new GetAlbumsQuery(resolvedUserId)
        {
            Page = request.Page,
            PageSize = request.PageSize
        };
        var albums = await _mediator.Send(query);
        return Ok(albums);
    }

    /// <summary>
    /// Get album details.
    /// </summary>
    /// <remarks>
    /// Returns details for a specific album including image count.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites" for the default album.</param>
    /// <returns>The album details.</returns>
    /// <response code="200">Returns the album.</response>
    /// <response code="404">Album or user not found.</response>
    [HttpGet("{albumId}")]
    [ProducesResponseType(typeof(AlbumDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<AlbumDto>> GetAlbum([FromRoute] string userId, [FromRoute] string albumId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        if (!CanAccess(resolvedUserId)) return NotFound();

        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (resolvedAlbumId == null) return NotFound();

        var album = await _mediator.Send(new GetAlbumByIdQuery(resolvedAlbumId.Value));
        return Ok(album);
    }

    /// <summary>
    /// Create a new album.
    /// </summary>
    /// <remarks>
    /// Create a new album to organize images. You can only create albums for yourself.
    /// </remarks>
    /// <param name="userId">The user ID or "me" (must be your own account).</param>
    /// <param name="request">The album details.</param>
    /// <returns>The created album.</returns>
    /// <response code="201">Album created successfully.</response>
    /// <response code="400">Invalid album data.</response>
    /// <response code="401">Authentication required.</response>
    [HttpPost]
    [ProducesResponseType(typeof(AlbumDto), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    public async Task<ActionResult<AlbumDto>> CreateAlbum([FromRoute] string userId, [FromBody] CreateAlbumRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        if (!CanAccess(resolvedUserId)) return NotFound();

        var album = await _mediator.Send(new CreateAlbumCommand(resolvedUserId, request.Name, request.Description));
        return CreatedAtAction(nameof(GetAlbum), new { userId = resolvedUserId, albumId = album.Id }, album);
    }

    /// <summary>
    /// Update an album.
    /// </summary>
    /// <remarks>
    /// Update album name or description. You can only update your own albums.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="request">The fields to update.</param>
    /// <returns>The updated album.</returns>
    /// <response code="200">Album updated successfully.</response>
    /// <response code="400">Invalid update data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Album not found.</response>
    [HttpPatch("{albumId}")]
    [ProducesResponseType(typeof(AlbumDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<AlbumDto>> UpdateAlbum([FromRoute] string userId, [FromRoute] string albumId, [FromBody] UpdateAlbumRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return NotFound();

        var album = await _mediator.Send(new UpdateAlbumCommand(resolvedUserId, resolvedAlbumId!.Value, request.Name, request.Description));
        return Ok(album);
    }

    /// <summary>
    /// Delete an album.
    /// </summary>
    /// <remarks>
    /// Permanently delete an album. The default "Favorites" album cannot be deleted.
    /// Images in the album are NOT deleted, only removed from the album.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID (cannot be "favorites").</param>
    /// <response code="204">Album deleted successfully.</response>
    /// <response code="400">Cannot delete default album.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Album not found.</response>
    [HttpDelete("{albumId}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> DeleteAlbum([FromRoute] string userId, [FromRoute] string albumId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return NotFound();

        await _mediator.Send(new DeleteAlbumCommand(resolvedUserId, resolvedAlbumId!.Value));
        return NoContent();
    }

    /// <summary>
    /// List images in an album.
    /// </summary>
    /// <remarks>
    /// Returns images in the album with full filtering support.
    /// Supports the same filters as the main images endpoint.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="request">Filter and pagination parameters.</param>
    /// <returns>A paginated list of images in the album.</returns>
    /// <response code="200">Returns the images.</response>
    /// <response code="400">Invalid filter parameters.</response>
    /// <response code="404">Album not found.</response>
    [HttpGet("{albumId}/images")]
    [ProducesResponseType(typeof(PaginatedList<ImageDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<PaginatedList<ImageDto>>> GetAlbumImages([FromRoute] string userId, [FromRoute] string albumId, [FromQuery] GetImagesRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        if (!CanAccess(resolvedUserId)) return NotFound();

        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (resolvedAlbumId == null) return NotFound();

        // Map Request to Query with AlbumId
        var query = new GetImagesQuery
        {
            IsNsfw = request.IsNsfw,
            IncludedTags = request.IncludedTags,
            ExcludedTags = request.ExcludedTags,
            IncludedArtists = request.IncludedArtists,
            ExcludedArtists = request.ExcludedArtists,
            IncludedIds = request.IncludedIds,
            ExcludedIds = request.ExcludedIds,
            IsAnimated = request.IsAnimated,
            OrderBy = request.OrderBy,
            Orientation = request.Orientation,
            Width = request.Width,
            Height = request.Height,
            ByteSize = request.ByteSize,
            Page = request.Page,
            PageSize = request.PageSize,
            AlbumId = resolvedAlbumId,
            UserId = _currentUserService.UserId,
            UserRole = _currentUserService.UserRole,
            ReviewStatus = request.ReviewStatus,
            ChildrenReviewStatus = request.ChildrenReviewStatus,
            IncludeMyPending = request.IncludeMyPending,
            IncludeMyPendingChildren = request.IncludeMyPendingChildren
        };

        var images = await _mediator.Send(query);
        return Ok(images);
    }

    /// <summary>
    /// Add an image to an album.
    /// </summary>
    /// <remarks>
    /// Add an image to one of your albums. Use "favorites" as albumId to add to your favorites.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="imageId">The image ID to add.</param>
    /// <response code="204">Image added successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Album or image not found.</response>
    [HttpPost("{albumId}/images/{imageId:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> AddImage([FromRoute] string userId, [FromRoute] string albumId, [FromRoute] long imageId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return NotFound();

        await _mediator.Send(new AddImageToAlbumCommand(resolvedUserId, resolvedAlbumId!.Value, imageId));
        return NoContent();
    }

    /// <summary>
    /// Remove an image from an album.
    /// </summary>
    /// <remarks>
    /// Remove an image from one of your albums. The image is not deleted, only unlinked from the album.
    /// </remarks>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="imageId">The image ID to remove.</param>
    /// <response code="204">Image removed successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">Album or image not found in album.</response>
    [HttpDelete("{albumId}/images/{imageId:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> RemoveImage([FromRoute] string userId, [FromRoute] string albumId, [FromRoute] long imageId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return NotFound();

        await _mediator.Send(new RemoveImageFromAlbumCommand(resolvedUserId, resolvedAlbumId!.Value, imageId));
        return NoContent();
    }

    private bool CanAccess(long targetUserId)
    {
        var currentUserId = _currentUserService.UserId;
        if (currentUserId == null) return false;

        var userRole = User.FindFirst(ClaimTypes.Role)?.Value;

        return targetUserId == currentUserId || userRole == Role.Admin.ToString();
    }
}

/// <summary>
/// Request model for creating a new album.
/// </summary>
public class CreateAlbumRequest
{
    /// <summary>
    /// The name of the album.
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// An optional description for the album.
    /// </summary>
    public string Description { get; set; } = string.Empty;
}

/// <summary>
/// Request model for updating an album.
/// </summary>
public class UpdateAlbumRequest
{
    /// <summary>
    /// The updated album name.
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// The updated description.
    /// </summary>
    public string Description { get; set; } = string.Empty;
}
