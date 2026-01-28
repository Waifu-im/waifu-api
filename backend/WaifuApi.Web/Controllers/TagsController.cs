using Mediator;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.GetTags;
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
}
