namespace WaifuApi.Domain.Entities;

/// <summary>
/// WARNING: This entity is manipulated via RAW SQL in RequestLoggingMiddleware.
/// Any changes to table name or column names must be reflected in those raw SQL queries.
/// </summary>
public class GlobalStat
{
    public string Key { get; set; } = string.Empty; // Ex: "TotalRequests"
    public long Value { get; set; }
}