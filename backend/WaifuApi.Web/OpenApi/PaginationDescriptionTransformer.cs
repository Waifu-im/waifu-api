using Microsoft.AspNetCore.OpenApi;
using Microsoft.OpenApi;

namespace WaifuApi.Web.OpenApi;

/// <summary>
/// Dynamically updates PageSize parameter descriptions with actual configured values.
/// </summary>
public class PaginationDescriptionTransformer : IOpenApiOperationTransformer
{
    private readonly IConfiguration _configuration;

    public PaginationDescriptionTransformer(IConfiguration configuration)
    {
        _configuration = configuration;
    }

    public Task TransformAsync(OpenApiOperation operation, OpenApiOperationTransformerContext context, CancellationToken cancellationToken)
    {
        if (operation.Parameters == null) return Task.CompletedTask;

        var configSection = GetConfigSectionFromPath(context.Description.RelativePath);                                                                                                                                  
        if (configSection == null) return Task.CompletedTask;   
        
        var defaultPageSize = _configuration[$"{configSection}:DefaultPageSize"];                                                                                                                                        
        var maxPageSize = _configuration[$"{configSection}:MaxPageSize"];  
        
        foreach (var param in operation.Parameters)
        {
            if (string.Equals(param.Name, "pageSize", StringComparison.OrdinalIgnoreCase))
            {
                var maxInfo = FormatMaxPageSize(maxPageSize);
                param.Description = $"Number of results per page. Default: {defaultPageSize}. {maxInfo}";
            }
        }

        return Task.CompletedTask;
    }

    private static string? GetConfigSectionFromPath(string? path)
    {
        if (string.IsNullOrEmpty(path)) return null;

        // Match paths to config sections (path may or may not have leading /)
        if (path.Contains("images", StringComparison.OrdinalIgnoreCase)) return "Image";
        if (path.Contains("tags", StringComparison.OrdinalIgnoreCase)) return "Tag";
        if (path.Contains("artists", StringComparison.OrdinalIgnoreCase)) return "Artist";
        if (path.Contains("albums", StringComparison.OrdinalIgnoreCase)) return "Album";
        if (path.Contains("review", StringComparison.OrdinalIgnoreCase)) return "Review";
        if (path.Contains("users", StringComparison.OrdinalIgnoreCase)) return "User";
        if (path.Contains("storage-diff", StringComparison.OrdinalIgnoreCase)) return "StorageDiff";

        return null;
    }

    private static string FormatMaxPageSize(string? maxPageSize)
    {
        if (string.IsNullOrEmpty(maxPageSize)) return "";

        if (int.TryParse(maxPageSize, out var max))
        {
            if (max <= 0) return "Max: unlimited.";
            return $"Max: {max}.";
        }

        return "";
    }
}
