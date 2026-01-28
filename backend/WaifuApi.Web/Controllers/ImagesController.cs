using System.Collections.Generic;
using System.IO;
using System.Security.Claims;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.GetImageById;
using WaifuApi.Application.Features.GetImages;
using WaifuApi.Application.Features.Images.DeleteImage;
using WaifuApi.Application.Features.Images.UpdateImage;
using WaifuApi.Application.Features.Images.UploadImage;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("images")]
public class ImagesController : ControllerBase
{
    private readonly IMediator _mediator;

    public ImagesController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Searches for images based on filters.
    /// </summary>
    /// <param name="query">The search filters.</param>
    /// <remarks>
    /// **IsNsfw**: 
    /// - Safe (0): Returns only Safe images (Default).
    /// - Nsfw (1): Returns only NSFW images.
    /// - All (2): Returns both Safe and NSFW images.
    /// 
    /// **Limit**:
    /// - Number (e.g., "30"): Returns up to that many images (Default 1).
    /// - "all": Returns all images (Admin only).
    /// </remarks>
    /// <returns>A list of images matching the criteria.</returns>
    [HttpGet]
    public async Task<ActionResult<List<ImageDto>>> Get([FromQuery] GetImagesQuery query)
    {
        var images = await _mediator.Send(query);
        return Ok(images);
    }

    /// <summary>
    /// Gets a single image by ID.
    /// </summary>
    /// <param name="id">The image ID.</param>
    /// <returns>The image details.</returns>
    [HttpGet("{id:long}")]
    public async Task<ActionResult<ImageDto>> GetById([FromRoute] long id)
    {
        var userIdClaim = User.FindFirst(ClaimTypes.NameIdentifier);
        var userId = userIdClaim != null ? long.Parse(userIdClaim.Value) : 0;
        
        var image = await _mediator.Send(new GetImageByIdQuery(id, userId));
        return Ok(image);
    }

    /// <summary>
    /// Uploads a new image.
    /// </summary>
    /// <param name="request">The upload request containing the file and metadata.</param>
    /// <returns>The uploaded image details.</returns>
    [Authorize]
    [HttpPost("upload")]
    public async Task<ActionResult<ImageDto>> Upload([FromForm] UploadImageRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);

        using var stream = request.File.OpenReadStream();
        var command = new UploadImageCommand(
            userId,
            stream,
            request.File.FileName,
            request.File.ContentType,
            request.ArtistName,
            request.Tags ?? new List<string>(),
            request.Source,
            request.IsNsfw
        );

        var image = await _mediator.Send(command);
        return CreatedAtAction(nameof(GetById), new { id = image.Id }, image);
    }

    /// <summary>
    /// Updates an image's metadata.
    /// </summary>
    /// <param name="id">The image ID.</param>
    /// <param name="request">The update request.</param>
    /// <returns>The updated image details.</returns>
    [Authorize(Policy = "Moderator")]
    [HttpPatch("{id:long}")]
    public async Task<ActionResult<ImageDto>> Update([FromRoute] long id, [FromBody] UpdateImageRequest request)
    {
        var command = new UpdateImageCommand(id, request.Source, request.IsNsfw, request.UserId);
        var image = await _mediator.Send(command);
        return Ok(image);
    }

    /// <summary>
    /// Deletes an image.
    /// </summary>
    /// <param name="id">The ID of the image to delete.</param>
    [Authorize]
    [HttpDelete("{id:long}")]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var userRole = User.FindFirst(ClaimTypes.Role)?.Value;
        var isAdminOrModerator = userRole == Role.Admin.ToString() || userRole == Role.Moderator.ToString();

        await _mediator.Send(new DeleteImageCommand(id, userId, isAdminOrModerator));
        return NoContent();
    }
}

public class UploadImageRequest
{
    public IFormFile File { get; set; } = null!;
    public string? ArtistName { get; set; }
    public List<string>? Tags { get; set; }
    public string? Source { get; set; }
    public bool IsNsfw { get; set; }
}

public class UpdateImageRequest
{
    public string? Source { get; set; }
    public bool? IsNsfw { get; set; }
    public long? UserId { get; set; }
}
