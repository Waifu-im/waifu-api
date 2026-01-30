using System.Collections.Generic;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.GetTags;
using WaifuApi.Application.Features.Tags.CreateTag;
using WaifuApi.Application.Features.Tags.DeleteTag;
using WaifuApi.Application.Features.Tags.GetTagById;
using WaifuApi.Application.Features.Tags.GetTagBySlug;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Domain.Enums;

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

    [HttpGet]
    public async Task<ActionResult<PaginatedList<TagDto>>> Get([FromQuery] GetTagsQuery query)
    {
        var tags = await _mediator.Send(query);
        return Ok(tags);
    }

    [HttpGet("{id:long}")]
    public async Task<ActionResult<TagDto>> GetById([FromRoute] long id)
    {
        var tag = await _mediator.Send(new GetTagByIdQuery(id));
        return Ok(tag);
    }

    [HttpGet("by-slug/{slug}")]
    public async Task<ActionResult<TagDto>> GetBySlug([FromRoute] string slug)
    {
        var tag = await _mediator.Send(new GetTagBySlugQuery(slug));
        return Ok(tag);
    }

    [Authorize]
    [HttpPost]
    public async Task<ActionResult<TagDto>> Create([FromBody] CreateTagRequest request)
    {
        var tag = await _mediator.Send(new CreateTagCommand(request.Name, request.Description, request.Slug));
        return CreatedAtAction(nameof(Get), new { id = tag.Id }, tag);
    }

    [Authorize(Policy = "Moderator")]
    [HttpPut("{id:long}")]
    public async Task<ActionResult<TagDto>> Update([FromRoute] long id, [FromBody] UpdateTagRequest request)
    {
        var command = new UpdateTagCommand(id, request.Name, request.Description, request.Slug, request.ReviewStatus);
        var tag = await _mediator.Send(command);
        return Ok(tag);
    }

    [Authorize(Policy = "Admin")]
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
    public string? Slug { get; set; }
}

public class UpdateTagRequest
{
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
    public string? Slug { get; set; }
    public ReviewStatus? ReviewStatus { get; set; }
}