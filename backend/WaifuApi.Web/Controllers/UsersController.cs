using System.Collections.Generic;
using System.ComponentModel.DataAnnotations;
using System.Security.Claims;
using System.Threading.Tasks;
using Mediator;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Mvc;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Users.BanUser;
using WaifuApi.Application.Features.Users.GetMe;
using WaifuApi.Application.Features.Users.GetUserById;
using WaifuApi.Application.Features.Users.GetUsers;
using WaifuApi.Application.Features.Users.UpdateUserRole;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;
using WaifuApi.Web.Constants;
using WaifuApi.Web.Models;

namespace WaifuApi.Web.Controllers;

/// <summary>
/// User profile and administration.
/// </summary>
/// <remarks>
/// Endpoints for viewing user profiles and administrative user management.
///
/// **Roles:**
/// - **User**: Basic authenticated user
/// - **TrustedUser**: Can upload without review
/// - **Moderator**: Can review and approve content
/// - **Admin**: Full administrative access
/// </remarks>
[ApiController]
[Route("users")]
[Authorize]
[Produces("application/json")]
[Tags("Users")]
[ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status401Unauthorized)]
public class UsersController : ControllerBase
{
    private readonly IMediator _mediator;

    public UsersController(IMediator mediator)
    {
        _mediator = mediator;
    }

    /// <summary>
    /// Get your own profile.
    /// </summary>
    /// <remarks>
    /// Returns the profile information for the currently authenticated user.
    /// </remarks>
    /// <returns>Your user profile.</returns>
    /// <response code="200">Returns your profile.</response>
    /// <response code="401">Authentication required.</response>
    [HttpGet("me")]
    [ProducesResponseType(typeof(UserDto), StatusCodes.Status200OK)]
    public async Task<ActionResult<UserDto>> GetMe()
    {
        var userId = long.Parse(User.FindFirst(ClaimTypes.NameIdentifier)!.Value);
        var user = await _mediator.Send(new GetMeQuery(userId));
        return Ok(user);
    }

    /// <summary>
    /// Get a user by ID.
    /// </summary>
    /// <remarks>
    /// Returns minimal user information. Used by moderators for content review.
    ///
    /// **Requires:** Moderator role or higher.
    /// </remarks>
    /// <param name="id">The user ID.</param>
    /// <returns>The user's minimal profile.</returns>
    /// <response code="200">Returns the user.</response>
    /// <response code="403">Insufficient permissions.</response>
    /// <response code="404">User not found.</response>
    [HttpGet("{id:long}")]
    [Authorize(Policy = AuthorizationPolicies.Moderator)]
    [ProducesResponseType(typeof(UserMinimalDto), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<UserMinimalDto>> GetUserById(long id)
    {
        var user = await _mediator.Send(new GetUserByIdQuery(id));
        return Ok(user);
    }

    /// <summary>
    /// List all users.
    /// </summary>
    /// <remarks>
    /// Returns a paginated list of all users with their full profiles.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="query">Pagination and filter parameters.</param>
    /// <returns>A paginated list of users.</returns>
    /// <response code="200">Returns the list of users.</response>
    /// <response code="400">Invalid query parameters.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    [HttpGet]
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [ProducesResponseType(typeof(PaginatedList<UserDto>), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    public async Task<ActionResult<PaginatedList<UserDto>>> Get([FromQuery] GetUsersQuery query)
    {
        var users = await _mediator.Send(query);
        return Ok(users);
    }

    /// <summary>
    /// Change a user's role.
    /// </summary>
    /// <remarks>
    /// Update a user's role to grant or revoke permissions.
    /// You cannot change your own role.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="id">The user ID to update.</param>
    /// <param name="request">The new role.</param>
    /// <returns>The updated user.</returns>
    /// <response code="200">Role updated successfully.</response>
    /// <response code="400">Invalid role or trying to change own role.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    /// <response code="404">User not found.</response>
    [HttpPut("{id:long}/role")]
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [ProducesResponseType(typeof(User), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ValidationErrorResponse), StatusCodes.Status400BadRequest)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<User>> UpdateRole(long id, [FromBody] UpdateRoleRequest request)
    {
        var user = await _mediator.Send(new UpdateUserRoleCommand(id, request.Role));
        return Ok(user);
    }

    /// <summary>
    /// Ban or unban a user.
    /// </summary>
    /// <remarks>
    /// Banned users cannot authenticate or use the API.
    /// Their existing API keys will be rejected.
    ///
    /// **Requires:** Admin role.
    /// </remarks>
    /// <param name="id">The user ID to ban/unban.</param>
    /// <param name="request">The ban status.</param>
    /// <returns>The updated user.</returns>
    /// <response code="200">Ban status updated successfully.</response>
    /// <response code="403">Insufficient permissions (Admin required).</response>
    /// <response code="404">User not found.</response>
    [HttpPut("{id:long}/ban")]
    [Authorize(Policy = AuthorizationPolicies.Admin)]
    [ProducesResponseType(typeof(User), StatusCodes.Status200OK)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status403Forbidden)]
    [ProducesResponseType(typeof(ErrorResponse), StatusCodes.Status404NotFound)]
    public async Task<ActionResult<User>> Ban(long id, [FromBody] BanUserRequest request)
    {
        var user = await _mediator.Send(new BanUserCommand(id, request.IsBlacklisted));
        return Ok(user);
    }
}

/// <summary>
/// Request model for updating a user's role.
/// </summary>
public class UpdateRoleRequest
{
    /// <summary>
    /// The new role to assign to the user.
    /// </summary>
    [Required]
    public Role Role { get; set; }
}

/// <summary>
/// Request model for banning/unbanning a user.
/// </summary>
public class BanUserRequest
{
    /// <summary>
    /// Set to true to ban the user, false to unban.
    /// </summary>
    public bool IsBlacklisted { get; set; }
}
