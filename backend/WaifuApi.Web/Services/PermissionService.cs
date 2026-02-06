using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Web.Services;

public interface IPermissionService
{
    /// <summary>
    /// Check if the current user has the minimum role required for an action.
    /// Throws ForbiddenException with a friendly message if the user doesn't have permission.
    /// </summary>
    void EnsurePermission(string configKey, string actionDescription);

    /// <summary>
    /// Get the minimum role required for an action from configuration.
    /// Returns null if no restriction is configured (any authenticated user can perform the action).
    /// </summary>
    Role? GetMinimumRole(string configKey);

    /// <summary>
    /// Get a description of the role requirement for OpenAPI documentation.
    /// Returns null if no restriction is configured or if the role is User.
    /// </summary>
    string? GetRoleRequirementDescription(string configKey);
}

public class PermissionService : IPermissionService
{
    private readonly IConfiguration _configuration;
    private readonly ICurrentUserService _currentUser;

    public PermissionService(IConfiguration configuration, ICurrentUserService currentUser)
    {
        _configuration = configuration;
        _currentUser = currentUser;
    }

    public void EnsurePermission(string configKey, string actionDescription)
    {
        var minRole = GetMinimumRole(configKey);

        // No restriction configured - any authenticated user can perform the action
        if (minRole == null)
        {
            return;
        }

        var userRole = _currentUser.UserRole ?? Role.User;

        // User has sufficient role
        if (userRole >= minRole.Value)
        {
            return;
        }

        // Throw with a friendly message
        throw new ForbiddenException(
            $"This action requires the {minRole.Value} role or higher. " +
            $"This restriction is configured by the API administrator.");
    }

    public Role? GetMinimumRole(string configKey)
    {
        var configValue = _configuration[configKey];

        if (string.IsNullOrEmpty(configValue))
        {
            return null;
        }

        if (Enum.TryParse<Role>(configValue, ignoreCase: true, out var role))
        {
            return role;
        }

        return null;
    }

    public string? GetRoleRequirementDescription(string configKey)
    {
        var minRole = GetMinimumRole(configKey);

        // No restriction or User role (default) - no special description needed
        if (minRole == null || minRole == Role.User)
        {
            return null;
        }

        return $"**Requires:** {minRole.Value} role or higher (configured by API administrator).";
    }
}
