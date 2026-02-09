using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Utilities;

/// <summary>
/// Utility class for role-based authorization checks.
/// </summary>
public static class RoleUtils
{
    /// <summary>
    /// Checks if the role is Moderator or Admin.
    /// </summary>
    public static bool IsModeratorOrAdmin(Role? role)
    {
        return role is Role.Admin or Role.Moderator;
    }

    /// <summary>
    /// Checks if the role is Admin.
    /// </summary>
    public static bool IsAdmin(Role? role)
    {
        return role == Role.Admin;
    }

    /// <summary>
    /// Checks if the user has permission to access non-accepted review statuses.
    /// Regular users can only view Accepted content.
    /// </summary>
    public static bool CanAccessNonAcceptedReviewStatus(Role? role)
    {
        return IsModeratorOrAdmin(role);
    }
}
