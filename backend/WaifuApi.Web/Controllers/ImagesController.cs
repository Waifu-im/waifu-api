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
using WaifuApi.Domain.Enums;

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

    [HttpGet]
    public async Task<ActionResult<PaginatedList<ImageDto>>> Get([FromQuery] GetImagesQuery query)
    {
        var userIdClaim = User.FindFirst(ClaimTypes.NameIdentifier);
        if (userIdClaim != null && long.TryParse(userIdClaim.Value, out var userId))
        {
            query.UserId = userId;
        }

        var images = await _mediator.Send(query);
        return Ok(images);
    }

    [HttpGet("{id:long}")]
    public async Task<ActionResult<ImageDto>> GetById([FromRoute] long id)
    {
        var userIdClaim = User.FindFirst(ClaimTypes.NameIdentifier);
        var userId = userIdClaim != null ? long.Parse(userIdClaim.Value) : 0;
        
        var image = await _mediator.Send(new GetImageByIdQuery(id, userId));
        return Ok(image);
    }

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
            request.Artists ?? new List<long>(),
            request.Tags ?? new List<string>(), // Changed to list of slugs
            request.Source,
            request.IsNsfw
        );

        var image = await _mediator.Send(command);
        return CreatedAtAction(nameof(GetById), new { id = image.Id }, image);
    }

    [Authorize(Policy = "Moderator")]
    [HttpPut("{id:long}")]
    public async Task<ActionResult<ImageDto>> Update([FromRoute] long id, [FromBody] UpdateImageRequest request)
    {
        var command = new UpdateImageCommand(id, request.Source, request.IsNsfw, request.UserId, request.Tags, request.Artists, request.ReviewStatus);
        var image = await _mediator.Send(command);
        return Ok(image);
    }

    [Authorize(Policy = "Admin")]
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
    public List<long>? Artists { get; set; }
    public List<string>? Tags { get; set; } // Slugs
    public string? Source { get; set; }
    public bool IsNsfw { get; set; }
}

public class UpdateImageRequest
{
    public string? Source { get; set; }
    public bool IsNsfw { get; set; }
    public long? UserId { get; set; }
    public List<string>? Tags { get; set; } // Slugs
    public List<long>? Artists { get; set; } // IDs
    public ReviewStatus? ReviewStatus { get; set; }
}