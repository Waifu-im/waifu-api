using System.ComponentModel.DataAnnotations;
using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Auth.CreateApiKey;
using WaifuApi.Application.Features.Auth.GetApiKeys;
using WaifuApi.Application.Features.Auth.LoginWithDiscord;
using WaifuApi.Application.Features.Auth.RevokeApiKey;
using WaifuApi.Application.Features.Auth.UpdateApiKey;
using WaifuApi.Domain.Entities;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// Authentication and API key management.
/// </summary>
/// <remarks>
/// This controller handles user authentication via Discord OAuth2 and API key management.
///
/// **Authentication methods:**
/// - **JWT Bearer Token**: For web application sessions (short-lived)
/// - **API Key**: For programmatic access (long-lived, can have expiration)
///
/// Use the Discord login endpoint to obtain a JWT token, then use that token to manage your API keys.
/// </remarks>
[ApiController]
[Route("auth")]
[Produces("application/json")]
[Tags("Authentication")]
public class AuthController : ControllerBase
{
    private readonly IMediator _mediator;

    public AuthController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Login with Discord OAuth2.
    /// </summary>
    /// <remarks>
    /// Exchange a Discord OAuth2 authorization code for a JWT token.
    ///
    /// **Flow:**
    /// 1. Redirect user to Discord OAuth2 authorization URL
    /// 2. User authorizes the application
    /// 3. Discord redirects back with a `code` parameter
    /// 4. Send the code to this endpoint to get a JWT token
    ///
    /// The JWT token should be included in subsequent requests as: `Authorization: Bearer {token}`
    /// </remarks>
    /// <param name="command">The Discord authorization code.</param>
    /// <returns>A JWT token for authentication.</returns>
    /// <response code="200">Returns the JWT token.</response>
    /// <response code="400">Invalid or expired authorization code.</response>
    [HttpPost("discord")]
    [ProducesResponseType(StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    public async Task<ActionResult<string>> LoginWithDiscord([FromBody] LoginWithDiscordCommand command)
    {
        var token = await _mediator.Send(command);
        return Ok(new { token });
    }

    /// <summary>
    /// List your API keys.
    /// </summary>
    /// <remarks>
    /// Returns all API keys associated with your account.
    /// For security, only the key prefix is returned (not the full key).
    /// </remarks>
    /// <returns>A list of your API keys.</returns>
    /// <response code="200">Returns the list of API keys.</response>
    /// <response code="401">Authentication required.</response>
    [Authorize]
    [HttpGet("api-keys")]
    [ProducesResponseType(typeof(List<ApiKeyDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    public async Task<ActionResult<List<ApiKeyDto>>> GetApiKeys()
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var keys = await _mediator.Send(new GetApiKeysQuery(userId));
        return Ok(keys);
    }

    /// <summary>
    /// Create a new API key.
    /// </summary>
    /// <remarks>
    /// Generate a new API key for programmatic access to the API.
    ///
    /// **Important:** The full API key is only shown once in the response. Store it securely.
    ///
    /// Use the API key in requests as: `Authorization: ApiKey {your_key}`
    /// </remarks>
    /// <param name="request">The API key details.</param>
    /// <returns>The created API key (full key shown only once).</returns>
    /// <response code="201">API key created successfully.</response>
    /// <response code="400">Invalid request data.</response>
    /// <response code="401">Authentication required.</response>
    [Authorize]
    [HttpPost("api-keys")]
    [ProducesResponseType(typeof(ApiKeyDto), StatusCodes.Status201Created)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    public async Task<ActionResult<ApiKeyDto>> CreateApiKey([FromBody] CreateApiKeyRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var key = await _mediator.Send(new CreateApiKeyCommand(userId, request.Description, request.ExpirationDate));
        return CreatedAtAction(nameof(GetApiKeys), null, key);
    }

    /// <summary>
    /// Update an API key.
    /// </summary>
    /// <remarks>
    /// Update the description or expiration date of an existing API key.
    /// You cannot change the key itself - create a new one if needed.
    /// </remarks>
    /// <param name="id">The API key ID to update.</param>
    /// <param name="request">The fields to update.</param>
    /// <returns>The updated API key.</returns>
    /// <response code="200">API key updated successfully.</response>
    /// <response code="400">Invalid update data.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">API key not found or doesn't belong to you.</response>
    [Authorize]
    [HttpPatch("api-keys/{id:long}")]
    [ProducesResponseType(typeof(ApiKeyDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<ApiKeyDto>> UpdateApiKey([FromRoute] long id, [FromBody] UpdateApiKeyRequest request)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var key = await _mediator.Send(new UpdateApiKeyCommand(userId, id, request.Description, request.ExpirationDate));
        return Ok(key);
    }

    /// <summary>
    /// Revoke (delete) an API key.
    /// </summary>
    /// <remarks>
    /// Permanently deletes an API key. Any requests using this key will be rejected immediately.
    /// This action cannot be undone.
    /// </remarks>
    /// <param name="id">The API key ID to revoke.</param>
    /// <response code="204">API key revoked successfully.</response>
    /// <response code="401">Authentication required.</response>
    /// <response code="404">API key not found or doesn't belong to you.</response>
    [Authorize]
    [HttpDelete("api-keys/{id:long}")]
    [ProducesResponseType(StatusCodes.Status204NoContent)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<IActionResult> RevokeApiKey([FromRoute] long id)
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        await _mediator.Send(new RevokeApiKeyCommand(userId, id));
        return NoContent();
    }
}

/// <summary>
/// Request model for creating a new API key.
/// </summary>
public class CreateApiKeyRequest
{
    /// <summary>
    /// A description to help identify this API key (e.g., "My Discord Bot", "Personal Script").
    /// </summary>
    [Required]
    public string Description { get; set; } = string.Empty;

    /// <summary>
    /// Optional expiration date. If not set, the key never expires.
    /// </summary>
    public DateTime? ExpirationDate { get; set; }
}

/// <summary>
/// Request model for updating an existing API key.
/// </summary>
public class UpdateApiKeyRequest
{
    /// <summary>
    /// Updated description.
    /// </summary>
    [Required]
    public string Description { get; set; } = string.Empty;

    /// <summary>
    /// Updated expiration date. Set to null to remove expiration.
    /// </summary>
    public DateTime? ExpirationDate { get; set; }
}
