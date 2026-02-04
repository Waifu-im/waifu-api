using System;

namespace WaifuApi.Domain.Entities;

/// <summary>
/// WARNING: This entity is manipulated via RAW SQL in RequestLoggingMiddleware and StatsCleanupService.
/// Any changes to table name or column names must be reflected in those raw SQL queries.
/// </summary>
public class DailyStat
{
    public int Id { get; set; }
    public DateOnly Date { get; set; }
    public int RequestCount { get; set; }
}