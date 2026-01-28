using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Artists.CreateArtist;
using WaifuApi.Application.Features.Artists.DeleteArtist;
using WaifuApi.Application.Features.Artists.GetArtistByName;
using WaifuApi.Application.Features.Artists.GetArtists;
using WaifuApi.Application.Features.Artists.UpdateArtist;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("artists")]
public class ArtistsController : ControllerBase
{
    private readonly IMediator _mediator;

    public ArtistsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Gets a paginated list of artists.
    /// </summary>
    /// <param name="query">Pagination parameters.</param>
    /// <returns>A paginated list of artists.</returns>
    [HttpGet]
    public async Task<ActionResult<PaginatedList<Artist>>> Get([FromQuery] GetArtistsQuery query)
    {
        var artists = await _mediator.Send(query);
        return Ok(artists);
    }

    /// <summary>
    /// Retrieves an artist by name.
    /// </summary>
    /// <param name="name">The name of the artist.</param>
    /// <returns>The artist details.</returns>
    [HttpGet("by-name/{name}")]
    public async Task<ActionResult<Artist>> GetByName([FromRoute] string name)
    {
        var artist = await _mediator.Send(new GetArtistByNameQuery(name));
        return Ok(artist);
    }

    /// <summary>
    /// Creates a new artist.
    /// </summary>
    /// <param name="request">The artist details.</param>
    /// <returns>The created artist.</returns>
    [Authorize]
    [HttpPost]
    public async Task<ActionResult<Artist>> Create([FromBody] CreateArtistRequest request)
    {
        var command = new CreateArtistCommand(
            request.Name,
            request.Patreon,
            request.Pixiv,
            request.Twitter,
            request.DeviantArt
        );
        var artist = await _mediator.Send(command);
        return CreatedAtAction(nameof(Get), new { id = artist.Id }, artist);
    }

    /// <summary>
    /// Updates an artist.
    /// </summary>
    /// <param name="id">The artist ID.</param>
    /// <param name="request">The update request.</param>
    /// <returns>The updated artist.</returns>
    [Authorize(Policy = "Moderator")]
    [HttpPut("{id:long}")]
    public async Task<ActionResult<Artist>> Update([FromRoute] long id, [FromBody] UpdateArtistRequest request)
    {
        var command = new UpdateArtistCommand(
            id, 
            request.Name,
            request.Patreon,
            request.Pixiv,
            request.Twitter,
            request.DeviantArt
        );
        var artist = await _mediator.Send(command);
        return Ok(artist);
    }

    /// <summary>
    /// Deletes an artist.
    /// </summary>
    /// <param name="id">The artist ID.</param>
    [Authorize(Policy = "Moderator")]
    [HttpDelete("{id:long}")]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        await _mediator.Send(new DeleteArtistCommand(id));
        return NoContent();
    }
}

public class CreateArtistRequest
{
    public string Name { get; set; } = string.Empty;
    public string? Patreon { get; set; }
    public string? Pixiv { get; set; }
    public string? Twitter { get; set; }
    public string? DeviantArt { get; set; }
}

public class UpdateArtistRequest
{
    public string Name { get; set; } = string.Empty;
    public string? Patreon { get; set; }
    public string? Pixiv { get; set; }
    public string? Twitter { get; set; }
    public string? DeviantArt { get; set; }
}
