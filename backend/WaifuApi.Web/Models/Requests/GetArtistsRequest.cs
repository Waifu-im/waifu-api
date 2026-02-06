using System.Collections.Generic;
using System.ComponentModel;

namespace WaifuApi.Web.Models.Requests;

/// <summary>
/// Request model for searching and filtering artists.
/// </summary>
public class GetArtistsRequest
{
    /// <summary>
    /// Filter by artist name (partial match, case-insensitive). Example: ?name=sakimichan
    /// </summary>
    [Description("Filter by artist name (partial match, case-insensitive). Example: ?name=sakimichan")]
    public string? Name { get; set; }

    /// <summary>
    /// Filter by specific artist IDs (exact match). Example: ?includedIds=1&amp;includedIds=2
    /// </summary>
    [Description("Filter by specific artist IDs (exact match). Example: ?includedIds=1&includedIds=2")]
    public List<long> IncludedIds { get; set; } = new();

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
