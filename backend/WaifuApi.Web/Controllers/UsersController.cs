using System.Security.Claims;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Features.Users.BanUser;
using WaifuApi.Application.Features.Users.GetMe;
using WaifuApi.Application.Features.Users.GetUsers;
using WaifuApi.Application.Features.Users.UpdateUserRole;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Web.Controllers;

[ApiController]
[Route("users")]
[Authorize]
public class UsersController : ControllerBase
{
    private readonly IMediator _mediator;

    public UsersController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Retrieves the current authenticated user's profile.
    /// </summary>
    /// <returns>The user profile.</returns>
    [HttpGet("me")]
    public async Task<ActionResult<User>> GetMe()
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var user = await _mediator.Send(new GetMeQuery(userId));
        return Ok(user);
    }

    /// <summary>
    /// Retrieves a list of users (Admin only).
    /// </summary>
    /// <param name="search">Optional search term for name or Discord ID.</param>
    /// <param name="page">Page number (default 1).</param>
    /// <param name="pageSize">Page size (default 20).</param>
    /// <returns>A paginated list of users.</returns>
    [HttpGet]
    [Authorize(Policy = "Admin")]
    public async Task<ActionResult<List<User>>> Get([FromQuery] string? search, [FromQuery] int page = 1, [FromQuery] int pageSize = 20)
    {
        var users = await _mediator.Send(new GetUsersQuery(search, page, pageSize));
        return Ok(users);
    }

    /// <summary>
    /// Updates a user's role (Admin only).
    /// </summary>
    /// <param name="id">The ID of the user to update.</param>
    /// <param name="request">The new role.</param>
    [HttpPut("{id:long}/role")]
    [Authorize(Policy = "Admin")]
    public async Task<IActionResult> UpdateRole(long id, [FromBody] UpdateRoleRequest request)
    {
        await _mediator.Send(new UpdateUserRoleCommand(id, request.Role));
        return NoContent();
    }

    /// <summary>
    /// Bans or unbans a user (Admin only).
    /// </summary>
    /// <param name="id">The ID of the user.</param>
    /// <param name="request">The ban status.</param>
    [HttpPut("{id:long}/ban")]
    [Authorize(Policy = "Admin")]
    public async Task<IActionResult> Ban(long id, [FromBody] BanUserRequest request)
    {
        await _mediator.Send(new BanUserCommand(id, request.IsBlacklisted));
        return NoContent();
    }
}

public class UpdateRoleRequest
{
    public Role Role { get; set; }
}

public class BanUserRequest
{
    public bool IsBlacklisted { get; set; }
}
