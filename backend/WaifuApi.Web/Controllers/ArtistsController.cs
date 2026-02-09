using System.Collections.Generic;
using System.ComponentModel;
using System.ComponentModel.DataAnnotations;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Artists.CreateArtist;
using WaifuApi.Application.Features.Artists.DeleteArtist;
using WaifuApi.Application.Features.Artists.GetArtistById;
using WaifuApi.Application.Features.Artists.GetArtistByName;
using WaifuApi.Application.Features.Artists.GetArtists;
using WaifuApi.Application.Features.Artists.UpdateArtist;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Web.Services;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Manage artists who created the images.
/// </summary>
/// <remarks>
/// Artists can be associated with images to give proper credit.
/// Each artist can have links to their social media profiles (Pixiv, Twitter, Patreon, DeviantArt).
/// New artists require moderator approval before being publicly visible.
/// </remarks>
[ApiController]
[Route("artists")]
[Produces("application/json")]
[Tags("Artists")]
public class ArtistsController : ControllerBase
{
    private readonly IMediator _mediator;
    private readonly IPermissionService _permissionService;
    private readonly ICurrentUserService _currentUser;

    public ArtistsController(IMediator mediator, IPermissionService permissionService, ICurrentUserService currentUser)
    {
        _mediator = mediator;
        _permissionService = permissionService;
        _currentUser = currentUser;
    }

    /// <summary>
    /// List all artists with optional filtering.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of artists. Use the `Name` parameter to search by artist name.
    /// By default, only approved artists are returned.
    /// </remarks>
    /// <param name="request">Search and pagination parameters.</param>
    /// <returns>A paginated list of artists.</returns>
    /// <response code="200">Returns the list of artists.</response>
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<ArtistDto>), StatusCodes.Status200OK)]
    public async Task<ActionResult<PaginatedList<ArtistDto>>> Get([FromQuery] GetArtistsRequest request)
    {
        var query = new GetArtistsQuery
        {
            Name = request.Name,
            IncludedIds = request.IncludedIds,
            Page = request.Page,
            PageSize = request.PageSize,
            IsModeratorOrAdmin = _currentUser.IsModeratorOrAdmin,
            ReviewStatus = _currentUser.IsModeratorOrAdmin ? request.ReviewStatus : null
        };

        var artists = await _mediator.Send(query);
        return Ok(artists);
    }

    /// <summary>
    /// Get an artist by ID.
    /// </summary>
    /// <param name="id">The unique identifier of the artist.</param>
    /// <returns>The artist details.</returns>
    /// <response code="200">Returns the artist.</response>
    /// <response code="404">Artist not found.</response>
    [HttpGet("{id:long}")]
    [ProducesResponseType(typeof(ArtistDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<ArtistDto>> GetById([FromRoute] long id)
    {
        var artist = await _mediator.Send(new GetArtistByIdQuery(id));
        return Ok(artist);
    }

    /// <summary>
    /// Get an artist by name.
    /// </summary>
    /// <remarks>
    /// Search for an artist by their exact name. Case-insensitive.
    /// </remarks>
    /// <param name="name">The name of the artist.</param>
    /// <returns>The artist details.</returns>
    /// <response code="200">Returns the artist.</response>
    /// <response code="404">Artist not found.</response>
    [HttpGet("by-name/{name}")]
    [ProducesResponseType(typeof(Artist), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<Artist>> GetByName([FromRoute] string name)
    {
        var artist = await _mediator.Send(new GetArtistByNameQuery(name));
        return Ok(artist);
    }

    /// <summary>
    /// Create a new artist.
    /// </summary>
    /// <remarks>
    /// Register a new artist in the database. Include their social media links if available.
    /// The artist will be created with "Pending" review status and must be approved by a moderator.
    /// </remarks>
    /// <param name="request">The artist details.</param>
    /// <returns>The created artist.</returns>
    /// <response code="201">Artist created successfully.</response>
    /// <response code="400">Invalid artist data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (role restriction configured by API administrator).</response>
    /// <response code="409">An artist with this name already exists.</response>
    [Authorize]
    [HttpPost]
    [ProducesResponseType(typeof(Artist), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status409Conflict)]
    public async Task<ActionResult<Artist>> Create([FromBody] CreateArtistRequest request)
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.ArtistCreationMinRole, "create artists");
        var command = new CreateArtistCommand(
            request.Name,
            request.Patreon,
            request.Pixiv,
            request.Twitter,
            request.DeviantArt,
            _currentUser.UserId
        );
        var artist = await _mediator.Send(command);
        return CreatedAtAction(nameof(Get), new { id = artist.Id }, artist);
    }

    /// <summary>
    /// Update an existing artist.
    /// </summary>
    /// <remarks>
    /// Update an artist's name, social links, or review status.
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="id">The artist ID to update.</param>
    /// <param name="request">The fields to update.</param>
    /// <returns>The updated artist.</returns>
    /// <response code="200">Artist updated successfully.</response>
    /// <response code="400">Invalid update data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    /// <response code="404">Artist not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}")]
    [ProducesResponseType(typeof(Artist), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<Artist>> Update([FromRoute] long id, [FromBody] UpdateArtistRequest request)
    {
        var command = new UpdateArtistCommand(
            id,
            request.Name,
            request.Patreon,
            request.Pixiv,
            request.Twitter,
            request.DeviantArt,
            request.ReviewStatus,
            request.CreatorId
        );
        var artist = await _mediator.Send(command);
        return Ok(artist);
    }

    /// <summary>
    /// Delete an artist permanently.
    /// </summary>
    /// <remarks>
    /// Permanently removes an artist. Images associated with this artist will no longer have this artist linked.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="id">The artist ID to delete.</param>
    /// <response code="204">Artist deleted successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    /// <response code="404">Artist not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [HttpDelete("{id:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        await _mediator.Send(new DeleteArtistCommand(id));
        return NoContent();
    }
}

/// <summary>
/// Request model for creating a new artist.
/// </summary>
/// <summary>
/// Request model for searching and filtering artists.
/// </summary>
public class GetArtistsRequest
{
    /// <summary>
    /// Filter by artist name (partial match, case-insensitive). Example: ?Name=fourthwallzart
    /// </summary>
    [Description("Filter by artist name (partial match, case-insensitive). Example: ?Name=fourthwallzart")]
    public string? Name { get; set; }

    /// <summary>
    /// Filter by specific artist IDs (exact match). Example: ?IncludedIds=1
    /// </summary>
    [Description("Filter by specific artist IDs (exact match). Example: ?IncludedIds=1")]
    public List<long> IncludedIds { get; set; } = new();

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
    /// Filter by review status (Moderator/Admin only). Default: Accepted for public users.
    /// </summary>
    [Description("Filter by review status (Moderator/Admin only). Default: Accepted for public users.")]
    public ReviewStatus? ReviewStatus { get; set; }
}

/// <summary>
/// Request model for creating a new artist.
/// </summary>
public class CreateArtistRequest
{
    /// <summary>
    /// The artist's name or alias.
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// Link to the artist's Patreon page.
    /// </summary>
    /// <example>https://www.patreon.com/artist</example>
    public string? Patreon { get; set; }

    /// <summary>
    /// Link to the artist's Pixiv profile.
    /// </summary>
    /// <example>https://www.pixiv.net/users/12345</example>
    public string? Pixiv { get; set; }

    /// <summary>
    /// Link to the artist's Twitter/X profile.
    /// </summary>
    /// <example>https://twitter.com/artist</example>
    public string? Twitter { get; set; }

    /// <summary>
    /// Link to the artist's DeviantArt profile.
    /// </summary>
    /// <example>https://www.deviantart.com/artist</example>
    public string? DeviantArt { get; set; }
}

/// <summary>
/// Request model for updating an existing artist.
/// </summary>
public class UpdateArtistRequest
{
    /// <summary>
    /// The updated artist name.
    /// </summary>
    [Required]
    public string Name { get; set; } = string.Empty;

    /// <summary>
    /// Updated Patreon link.
    /// </summary>
    public string? Patreon { get; set; }

    /// <summary>
    /// Updated Pixiv link.
    /// </summary>
    public string? Pixiv { get; set; }

    /// <summary>
    /// Updated Twitter/X link.
    /// </summary>
    public string? Twitter { get; set; }

    /// <summary>
    /// Updated DeviantArt link.
    /// </summary>
    public string? DeviantArt { get; set; }

    /// <summary>
    /// The review status (Moderator only). Set to Accepted to approve a pending artist.
    /// </summary>
    public ReviewStatus? ReviewStatus { get; set; }

    /// <summary>
    /// The creator user ID. Set to reassign who created this artist.
    /// </summary>
    public long? CreatorId { get; set; }
}
