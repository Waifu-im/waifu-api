using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Review.Artists.GetPendingArtists;
using WaifuApi.Application.Features.Review.Artists.ReviewArtist;
using WaifuApi.Application.Features.Review.Images.GetPendingImages;
using WaifuApi.Application.Features.Review.Images.ReviewImage;
using WaifuApi.Application.Features.Review.Tags.GetPendingTags;
using WaifuApi.Application.Features.Review.Tags.ReviewTag;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("review")]
[Authorize(Policy = "Moderator")]
public class ReviewController : ControllerBase
{
    private readonly IMediator _mediator;

    public ReviewController(IMediator mediator)
    {
        _mediator = mediator;
    }

    // Images
    /// <summary>
    /// Retrieves a list of images pending review.
    /// </summary>
    /// <returns>A list of pending images.</returns>
    [HttpGet("images")]
    public async Task<ActionResult<List<ImageDto>>> GetPendingImages()
    {
        var images = await _mediator.Send(new GetPendingImagesQuery());
        return Ok(images);
    }

    /// <summary>
    /// Reviews (accepts or rejects) an image.
    /// </summary>
    /// <param name="id">The ID of the image to review.</param>
    /// <param name="request">The review decision (accepted/rejected).</param>
    [HttpPost("images/{id:long}")]
    public async Task<IActionResult> ReviewImage([FromRoute] long id, [FromBody] ReviewRequest request)
    {
        await _mediator.Send(new ReviewImageCommand(id, request.Accepted));
        return NoContent();
    }

    // Artists
    /// <summary>
    /// Retrieves a list of artists pending review.
    /// </summary>
    /// <returns>A list of pending artists.</returns>
    [HttpGet("artists")]
    public async Task<ActionResult<List<Artist>>> GetPendingArtists()
    {
        var artists = await _mediator.Send(new GetPendingArtistsQuery());
        return Ok(artists);
    }

    /// <summary>
    /// Reviews (accepts or rejects) an artist.
    /// </summary>
    /// <param name="id">The ID of the artist to review.</param>
    /// <param name="request">The review decision.</param>
    [HttpPost("artists/{id:long}")]
    public async Task<IActionResult> ReviewArtist([FromRoute] long id, [FromBody] ReviewRequest request)
    {
        await _mediator.Send(new ReviewArtistCommand(id, request.Accepted));
        return NoContent();
    }

    // Tags
    /// <summary>
    /// Retrieves a list of tags pending review.
    /// </summary>
    /// <returns>A list of pending tags.</returns>
    [HttpGet("tags")]
    public async Task<ActionResult<List<Tag>>> GetPendingTags()
    {
        var tags = await _mediator.Send(new GetPendingTagsQuery());
        return Ok(tags);
    }

    /// <summary>
    /// Reviews (accepts or rejects) a tag.
    /// </summary>
    /// <param name="id">The ID of the tag to review.</param>
    /// <param name="request">The review decision.</param>
    [HttpPost("tags/{id:long}")]
    public async Task<IActionResult> ReviewTag([FromRoute] long id, [FromBody] ReviewRequest request)
    {
        await _mediator.Send(new ReviewTagCommand(id, request.Accepted));
        return NoContent();
    }
}

public class ReviewRequest
{
    public bool Accepted { get; set; }
}
