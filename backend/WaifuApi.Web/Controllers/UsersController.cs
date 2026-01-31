using System.Collections.Generic;
using System.Security.Claims;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Users.BanUser;
using WaifuApi.Application.Features.Users.GetMe;
using WaifuApi.Application.Features.Users.GetUserById;
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
    public async Task<ActionResult<UserDto>> GetMe()
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var user = await _mediator.Send(new GetMeQuery(userId));
        return Ok(user);
    }

    /// <summary>
    /// Retrieves a user by ID (Moderator only).
    /// Returns a minimal DTO safe for moderation tools.
    /// </summary>
    /// <param name="id">The ID of the user.</param>
    /// <returns>The user profile.</returns>
    [HttpGet("{id:long}")]
    [Authorize(Policy = "Moderator")]
    public async Task<ActionResult<UserMinimalDto>> GetUserById(long id)
    {
        var user = await _mediator.Send(new GetUserByIdQuery(id));
        return Ok(user);
    }

    /// <summary>
    /// Retrieves a list of users (Admin only).
    /// </summary>
    /// <param name="query">Pagination parameters.</param>
    /// <returns>A paginated list of users.</returns>
    [HttpGet]
    [Authorize(Policy = "Admin")]
    public async Task<ActionResult<PaginatedList<UserDto>>> Get([FromQuery] GetUsersQuery query)
    {
        var users = await _mediator.Send(query);
        return Ok(users);
    }

    /// <summary>
    /// Updates a user's role (Admin only).
    /// </summary>
    /// <param name="id">The ID of the user to update.</param>
    /// <param name="request">The new role.</param>
    /// <returns>The updated user.</returns>
    [HttpPut("{id:long}/role")]
    [Authorize(Policy = "Admin")]
    public async Task<ActionResult<User>> UpdateRole(long id, [FromBody] UpdateRoleRequest request)
    {
        var user = await _mediator.Send(new UpdateUserRoleCommand(id, request.Role));
        return Ok(user);
    }

    /// <summary>
    /// Bans or unbans a user (Admin only).
    /// </summary>
    /// <param name="id">The ID of the user.</param>
    /// <param name="request">The ban status.</param>
    /// <returns>The updated user.</returns>
    [HttpPut("{id:long}/ban")]
    [Authorize(Policy = "Admin")]
    public async Task<ActionResult<User>> Ban(long id, [FromBody] BanUserRequest request)
    {
        var user = await _mediator.Send(new BanUserCommand(id, request.IsBlacklisted));
        return Ok(user);
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
