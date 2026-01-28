using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.Auth.CreateApiKey;
using WaifuApi.Application.Features.Auth.GetApiKeys;
using WaifuApi.Application.Features.Auth.LoginWithDiscord;
using WaifuApi.Application.Features.Auth.RevokeApiKey;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("auth")]
public class AuthController : ControllerBase
{
    private readonly IMediator _mediator;

    public AuthController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Authenticates a user via Discord OAuth2 code.
    /// </summary>
    /// <param name="command">The login command containing the Discord code.</param>
    /// <returns>A JWT token.</returns>
    [HttpPost("discord")]
    public async Task<ActionResult<string>> LoginWithDiscord([FromBody] LoginWithDiscordCommand command)
    {
        var token = await _mediator.Send(command);
        return Ok(new { token });
    }

    /// <summary>
    /// Retrieves the API keys for the current user.
    /// </summary>
    /// <returns>A list of API keys.</returns>
    [Authorize]
    [HttpGet("api-keys")]
    public async Task<ActionResult<List<ApiKey>>> GetApiKeys()
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var keys = await _mediator.Send(new GetApiKeysQuery(userId));
        return Ok(keys);
    }

    /// <summary>
    /// Creates a new API key for the current user.
    /// </summary>
    /// <param name="request">The request containing the description for the new key.</param>
    /// <returns>The created API key.</returns>
    [Authorize]
    [HttpPost("api-keys")]
    public async Task<ActionResult<ApiKey>> CreateApiKey([FromBody] CreateApiKeyRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var key = await _mediator.Send(new CreateApiKeyCommand(userId, request.Description));
        return CreatedAtAction(nameof(GetApiKeys), null, key);
    }

    /// <summary>
    /// Revokes (deletes) an API key.
    /// </summary>
    /// <param name="id">The ID of the API key to revoke.</param>
    [Authorize]
    [HttpDelete("api-keys/{id:long}")]
    public async Task<IActionResult> RevokeApiKey([FromRoute] long id)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        await _mediator.Send(new RevokeApiKeyCommand(userId, id));
        return NoContent();
    }
}

public class CreateApiKeyRequest
{
    public string Description { get; set; } = string.Empty;
}
