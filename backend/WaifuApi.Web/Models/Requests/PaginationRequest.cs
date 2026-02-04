using System.ComponentModel;

namespace WaifuApi.Web.Models.Requests;

/// <summary>
/// Base request model for pagination.
/// </summary>
public class PaginationRequest
{
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
