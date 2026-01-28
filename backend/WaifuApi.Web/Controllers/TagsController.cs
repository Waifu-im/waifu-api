using System.Collections.Generic;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.GetTags;
using WaifuApi.Application.Features.Tags.CreateTag;
using WaifuApi.Application.Features.Tags.DeleteTag;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("tags")]
public class TagsController : ControllerBase
{
    private readonly IMediator _mediator;

    public TagsController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Retrieves all available tags.
    /// </summary>
    /// <returns>A list of tags.</returns>
    [HttpGet]
    public async Task<ActionResult<List<Tag>>> Get()
    {
        var tags = await _mediator.Send(new GetTagsQuery());
        return Ok(tags);
    }

    /// <summary>
    /// Creates a new tag.
    /// </summary>
    /// <param name="request">The tag details.</param>
    /// <returns>The created tag.</returns>
    [Authorize]
    [HttpPost]
    public async Task<ActionResult<Tag>> Create([FromBody] CreateTagRequest request)
    {
        var tag = await _mediator.Send(new CreateTagCommand(request.Name, request.Description));
        return CreatedAtAction(nameof(Get), new { id = tag.Id }, tag);
    }

    /// <summary>
    /// Updates a tag.
    /// </summary>
    /// <param name="id">The tag ID.</param>
    /// <param name="request">The update request.</param>
    /// <returns>The updated tag.</returns>
    [Authorize(Policy = "Moderator")]
    [HttpPut("{id:long}")]
    public async Task<ActionResult<Tag>> Update([FromRoute] long id, [FromBody] UpdateTagRequest request)
    {
        var command = new UpdateTagCommand(id, request.Name, request.Description);
        var tag = await _mediator.Send(command);
        return Ok(tag);
    }

    /// <summary>
    /// Deletes a tag.
    /// </summary>
    /// <param name="id">The tag ID.</param>
    [Authorize(Policy = "Moderator")]
    [HttpDelete("{id:long}")]
    public async Task<IActionResult> Delete([FromRoute] long id)
    {
        await _mediator.Send(new DeleteTagCommand(id));
        return NoContent();
    }
}

public class CreateTagRequest
{
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
}

public class UpdateTagRequest
{
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
}
