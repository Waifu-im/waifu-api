using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.Albums.AddImageToAlbum;
using WaifuApi.Application.Features.Albums.CreateAlbum;
using WaifuApi.Application.Features.Albums.DeleteAlbum;
using WaifuApi.Application.Features.Albums.GetAlbums;
using WaifuApi.Application.Features.Albums.RemoveImageFromAlbum;
using WaifuApi.Application.Features.Albums.UpdateAlbum;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("users/{userId}/albums")]
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
    /// Retrieves albums for a specific user.
    /// </summary>
    /// <param name="userId">The user ID or "me" for the current user.</param>
    /// <returns>A list of albums.</returns>
    [HttpGet]
    public async Task<ActionResult<List<Album>>> GetAlbums([FromRoute] string userId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var albums = await _mediator.Send(new GetAlbumsQuery(resolvedUserId));
        return Ok(albums);
    }

    /// <summary>
    /// Creates a new album.
    /// </summary>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="request">The album details.</param>
    /// <returns>The created album.</returns>
    [Authorize]
    [HttpPost]
    public async Task<ActionResult<Album>> CreateAlbum([FromRoute] string userId, [FromBody] CreateAlbumRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        if (!CanAccess(resolvedUserId)) return Forbid();

        var album = await _mediator.Send(new CreateAlbumCommand(resolvedUserId, request.Name, request.Description));
        return CreatedAtAction(nameof(GetAlbums), new { userId = resolvedUserId }, album);
    }

    /// <summary>
    /// Updates an existing album.
    /// </summary>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites" for the default album.</param>
    /// <param name="request">The updated album details.</param>
    /// <returns>The updated album.</returns>
    [Authorize]
    [HttpPut("{albumId}")]
    public async Task<ActionResult<Album>> UpdateAlbum([FromRoute] string userId, [FromRoute] string albumId, [FromBody] UpdateAlbumRequest request)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);
        
        if (!CanAccess(resolvedUserId)) return Forbid();

        var album = await _mediator.Send(new UpdateAlbumCommand(resolvedUserId, resolvedAlbumId!.Value, request.Name, request.Description));
        return Ok(album);
    }

    /// <summary>
    /// Deletes an album.
    /// </summary>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID.</param>
    [Authorize]
    [HttpDelete("{albumId}")]
    public async Task<IActionResult> DeleteAlbum([FromRoute] string userId, [FromRoute] string albumId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return Forbid();

        await _mediator.Send(new DeleteAlbumCommand(resolvedUserId, resolvedAlbumId!.Value));
        return NoContent();
    }

    /// <summary>
    /// Adds an image to an album.
    /// </summary>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="imageId">The ID of the image to add.</param>
    [Authorize]
    [HttpPost("{albumId}/images/{imageId:long}")]
    public async Task<IActionResult> AddImage([FromRoute] string userId, [FromRoute] string albumId, [FromRoute] long imageId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return Forbid();

        await _mediator.Send(new AddImageToAlbumCommand(resolvedUserId, resolvedAlbumId!.Value, imageId));
        return NoContent();
    }

    /// <summary>
    /// Removes an image from an album.
    /// </summary>
    /// <param name="userId">The user ID or "me".</param>
    /// <param name="albumId">The album ID or "favorites".</param>
    /// <param name="imageId">The ID of the image to remove.</param>
    [Authorize]
    [HttpDelete("{albumId}/images/{imageId:long}")]
    public async Task<IActionResult> RemoveImage([FromRoute] string userId, [FromRoute] string albumId, [FromRoute] long imageId)
    {
        var resolvedUserId = await _currentUserService.ResolveUserIdAsync(userId);
        var resolvedAlbumId = await _currentUserService.ResolveAlbumIdAsync(resolvedUserId, albumId);

        if (!CanAccess(resolvedUserId)) return Forbid();

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

public class CreateAlbumRequest
{
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
}

public class UpdateAlbumRequest
{
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
}
