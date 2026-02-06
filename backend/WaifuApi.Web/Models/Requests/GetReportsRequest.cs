using System.ComponentModel;

namespace WaifuApi.Web.Models.Requests;

/// <summary>
/// Request model for listing reports.
/// </summary>
public class GetReportsRequest
{
    /// <summary>
    /// Filter by resolution status.
    /// </summary>
    [Description("Filter by resolution status.")]
    public bool? IsResolved { get; set; }

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
