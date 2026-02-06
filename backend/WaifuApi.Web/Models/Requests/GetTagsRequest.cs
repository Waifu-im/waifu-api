using System.Collections.Generic;
using System.ComponentModel;

namespace WaifuApi.Web.Models.Requests;

/// <summary>
/// Request model for searching and filtering tags.
/// </summary>
public class GetTagsRequest
{
    /// <summary>
    /// Filter by tag name (partial match, case-insensitive). Example: ?name=blonde
    /// </summary>
    [Description("Filter by tag name (partial match, case-insensitive). Example: ?name=blonde")]
    public string? Name { get; set; }

    /// <summary>
    /// Filter by specific tag IDs (exact match). Example: ?includedIds=1&amp;includedIds=2
    /// </summary>
    [Description("Filter by specific tag IDs (exact match). Example: ?includedIds=1&includedIds=2")]
    public List<long> IncludedIds { get; set; } = new();

    /// <summary>
    /// Filter by specific tag slugs (exact match). Example: ?includedSlugs=blonde-hair&amp;includedSlugs=blue-eyes
    /// </summary>
    [Description("Filter by specific tag slugs (exact match). Example: ?includedSlugs=blonde-hair&includedSlugs=blue-eyes")]
    public List<string> IncludedSlugs { get; set; } = new();

    /// <summary>
    /// Page number for pagination. Default: 1.
    /// </summary>
    [Description("Page number for pagination. Default: 1.")]
    public int Page { get; set; } = 1;

    /// <summary>
    /// Number of results per page.
    /// </summary>
    [Description("Number of results per page.")]
    public int PageSize { get; set; }
}
