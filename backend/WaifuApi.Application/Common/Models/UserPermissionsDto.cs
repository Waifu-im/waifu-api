namespace WaifuApi.Application.Common.Models;

/// <summary>Configurable permissions of the current user, resolved against the server configuration.</summary>
public class UserPermissionsDto
{
    /// <summary>Whether the user can view and edit the site banner settings.</summary>
    public bool CanManageBanner { get; set; }
}
