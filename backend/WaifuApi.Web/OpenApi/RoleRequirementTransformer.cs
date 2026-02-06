using Microsoft.AspNetCore.OpenApi;
using Microsoft.OpenApi;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Web.OpenApi;

/// <summary>
/// Adds role requirement information to operation descriptions for endpoints with configurable role restrictions.
/// </summary>
public class RoleRequirementTransformer : IOpenApiOperationTransformer
{
    private readonly IConfiguration _configuration;

    public RoleRequirementTransformer(IConfiguration configuration)
    {
        _configuration = configuration;
    }

    public Task TransformAsync(OpenApiOperation operation, OpenApiOperationTransformerContext context, CancellationToken cancellationToken)
    {
        var configKey = GetConfigKeyForOperation(context);
        if (configKey == null) return Task.CompletedTask;

        var roleRequirement = GetRoleRequirementDescription(configKey);
        if (roleRequirement == null) return Task.CompletedTask;

        // Append role requirement to the operation description
        if (string.IsNullOrEmpty(operation.Description))
        {
            operation.Description = roleRequirement;
        }
        else
        {
            operation.Description = $"{operation.Description}\n\n{roleRequirement}";
        }

        return Task.CompletedTask;
    }

    private string? GetConfigKeyForOperation(OpenApiOperationTransformerContext context)
    {
        var path = context.Description.RelativePath?.ToLowerInvariant() ?? "";
        var method = context.Description.HttpMethod?.ToUpperInvariant() ?? "";

        // Only check POST endpoints for creation/upload
        if (method != "POST") return null;

        // Match specific endpoints to their config keys
        if (path.Contains("tags") && !path.Contains("/"))
            return ConfigurationKeys.Permissions.TagCreationMinRole;

        if (path.Contains("artists") && !path.Contains("/"))
            return ConfigurationKeys.Permissions.ArtistCreationMinRole;

        if (path.Contains("images/upload") || path.EndsWith("images"))
            return ConfigurationKeys.Permissions.ImageUploadMinRole;

        return null;
    }

    private string? GetRoleRequirementDescription(string configKey)
    {
        var configValue = _configuration[configKey];

        if (string.IsNullOrEmpty(configValue))
        {
            return null;
        }

        if (Enum.TryParse<Role>(configValue, ignoreCase: true, out var role))
        {
            // No special description needed for User role (default)
            if (role == Role.User)
            {
                return null;
            }

            return $"**Requires:** {role} role or higher (configured by API administrator).";
        }

        return null;
    }
}
