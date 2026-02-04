using System.Collections.Generic;
using System.ComponentModel;
using System.ComponentModel.DataAnnotations;
using System.IO;
using System.Security.Claims;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Images.GetImageById;
using WaifuApi.Application.Features.Images.GetImages;
using WaifuApi.Application.Features.Images.DeleteImage;
using WaifuApi.Application.Features.Images.UpdateImage;
using WaifuApi.Application.Features.Images.UploadImage;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;
using WaifuApi.Web.Models.Requests;
using WaifuApi.Web.Services;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Manage anime/waifu images in the gallery.
/// </summary>
/// <remarks>
/// This controller provides endpoints to browse, search, upload, update, and delete images.
/// Most read operations are public, while write operations require authentication.
/// </remarks>
[ApiController]
[Route("images")]
[Produces("application/json")]
[Tags("Images")]
public class ImagesController : ControllerBase
{
    private readonly IMediator _mediator;
    private readonly ICurrentUserService _currentUser;
    private readonly IPermissionService _permissionService;

    public ImagesController(IMediator mediator, ICurrentUserService currentUser, IPermissionService permissionService)
    {
        _mediator = mediator;
        _currentUser = currentUser;
        _permissionService = permissionService;
    }

    /// <summary>
    /// Search and browse images with filters.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of images. By default, only SFW and approved images are returned.
    /// See parameter descriptions for filtering options.
    /// </remarks>
    /// <param name="request">Search and filter parameters.</param>
    /// <returns>A paginated list of images.</returns>
    /// <response code="200">Returns the matching images.</response>
    /// <response code="400">Invalid filter parameters.</response>
    [HttpGet]
    [ProducesResponseType(typeof(PaginatedList<ImageDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    public async Task<ActionResult<PaginatedList<ImageDto>>> Get([FromQuery] GetImagesRequest request)
    {
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
            UserId = _currentUser.UserId,
            IsModeratorOrAdmin = _currentUser.IsModeratorOrAdmin,
            UploaderId = _currentUser.IsModeratorOrAdmin ? request.UploaderId : null
        };

        var images = await _mediator.Send(query);
        return Ok(images);
    }

    /// <summary>
    /// Get a specific image by ID.
    /// </summary>
    /// <remarks>
    /// Returns detailed information about a single image including tags, artists, and metadata.
    /// If authenticated, also includes user-specific data like favorite status.
    /// </remarks>
    /// <param name="id">The unique identifier of the image.</param>
    /// <returns>The image details.</returns>
    /// <response code="200">Returns the image.</response>
    /// <response code="404">Image not found.</response>
    [HttpGet("{id:long}")]
    [ProducesResponseType(typeof(ImageDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<ImageDto>> GetById([FromRoute] long id)
    {
        var image = await _mediator.Send(new GetImageByIdQuery(id, _currentUser.UserId ?? 0, _currentUser.IsModeratorOrAdmin));
        return Ok(image);
    }

    /// <summary>
    /// Upload a new image.
    /// </summary>
    /// <remarks>
    /// Upload an image file to the gallery. Supported formats: JPEG, PNG, GIF, WebP.
    ///
    /// The image will be processed to extract metadata (dimensions, dominant color, perceptual hash).
    /// New images are set to "Pending" review status and must be approved by a moderator before appearing in public searches.
    ///
    /// **Size limits:**
    /// - Maximum file size: 10MB
    /// - Minimum dimensions: configured per server
    /// </remarks>
    /// <param name="request">The image file and metadata.</param>
    /// <returns>The created image.</returns>
    /// <response code="201">Image uploaded successfully.</response>
    /// <response code="400">Invalid file or metadata.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (role restriction configured by API administrator).</response>
    [Authorize]
    [HttpPost("upload")]
    [ProducesResponseType(typeof(ImageDto), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<ImageDto>> Upload([FromForm] UploadImageRequest request)
    {
        _permissionService.EnsurePermission(ConfigurationKeys.Permissions.ImageUploadMinRole, "upload images");
        var userId = _currentUser.UserId!.Value;

        using var stream = request.File.OpenReadStream();
        var command = new UploadImageCommand(
            userId,
            stream,
            request.File.FileName,
            request.File.ContentType,
            request.Artists ?? new List<long>(),
            request.Tags ?? new List<string>(),
            request.Source,
            request.IsNsfw
        );

        var image = await _mediator.Send(command);
        return CreatedAtAction(nameof(GetById), new { id = image.Id }, image);
    }

    /// <summary>
    /// Update an existing image.
    /// </summary>
    /// <remarks>
    /// Update image metadata such as tags, artists, source URL, or NSFW status.
    /// Moderators can also update the review status and reassign the uploader.
    /// Optionally, the image file itself can be replaced.
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="id">The image ID to update.</param>
    /// <param name="request">The fields to update.</param>
    /// <returns>The updated image.</returns>
    /// <response code="200">Image updated successfully.</response>
    /// <response code="400">Invalid update data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions.</response>
    /// <response code="404">Image not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [HttpPatch("{id:long}")]
    [ProducesResponseType(typeof(ImageDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<ImageDto>> Update([FromRoute] long id, [FromForm] UpdateImageRequest request)
    {
        Stream? fileStream = null;
        string? fileName = null;
        string? contentType = null;

        if (request.File != null)
        {
            fileStream = request.File.OpenReadStream();
            fileName = request.File.FileName;
            contentType = request.File.ContentType;
        }

        try
        {
            var command = new UpdateImageCommand(
                id,
                request.Source,
                request.IsNsfw,
                request.UserId,
                request.Tags,
                request.Artists,
                request.ReviewStatus,
                fileStream,
                fileName,
                contentType
            );
            var image = await _mediator.Send(command);
            return Ok(image);
        }
        finally
        {
            fileStream?.Dispose();
        }
    }

    /// <summary>
    /// Delete an image permanently.
    /// </summary>
    /// <remarks>
    /// Permanently removes an image from the gallery and storage.
    /// This action cannot be undone.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="id">The image ID to delete.</param>
    /// <response code="204">Image deleted successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    /// <response code="404">Image not found.</response>
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [HttpDelete("{id:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        await _mediator.Send(new DeleteImageCommand(id, _currentUser.UserId!.Value, _currentUser.IsModeratorOrAdmin));
        return NoContent();
    }
}

/// <summary>
/// Request model for uploading a new image.
/// </summary>
public class UploadImageRequest
{
    /// <summary>
    /// The image file to upload. Supported formats: JPEG, PNG, GIF, WebP.
    /// </summary>
    [Required]
    public IFormFile File { get; set; } = null!;

    /// <summary>
    /// List of artist IDs to associate with this image.
    /// </summary>
    public List<long>? Artists { get; set; }

    /// <summary>
    /// List of tag slugs to associate with this image (e.g., "waifu", "blonde-hair").
    /// </summary>
    public List<string>? Tags { get; set; }

    /// <summary>
    /// Source URL where this image was found (e.g., Pixiv, Twitter link).
    /// </summary>
    public string? Source { get; set; }

    /// <summary>
    /// Whether this image contains NSFW (Not Safe For Work) content.
    /// </summary>
    [DefaultValue(false)]
    public bool IsNsfw { get; set; }
}

/// <summary>
/// Request model for updating an existing image.
/// </summary>
public class UpdateImageRequest
{
    /// <summary>
    /// Optional replacement image file.
    /// </summary>
    public IFormFile? File { get; set; }

    /// <summary>
    /// Updated source URL.
    /// </summary>
    public string? Source { get; set; }

    /// <summary>
    /// Updated NSFW status.
    /// </summary>
    public bool IsNsfw { get; set; }

    /// <summary>
    /// Reassign the image to a different uploader (Moderator only).
    /// </summary>
    public long? UserId { get; set; }

    /// <summary>
    /// Updated list of tag slugs.
    /// </summary>
    public List<string>? Tags { get; set; }

    /// <summary>
    /// Updated list of artist IDs.
    /// </summary>
    public List<long>? Artists { get; set; }

    /// <summary>
    /// Updated review status (Moderator only).
    /// </summary>
    public ReviewStatus? ReviewStatus { get; set; }
}
