namespace WaifuApi.Domain.Enums;

/// <summary>
/// Lifecycle status of an image report. Mirrors <see cref="ReviewTaskStatus"/>: a report starts Pending and is
/// resolved by a moderator (Resolved = acted on/valid, Rejected = dismissed) or withdrawn by its author (Cancelled).
/// </summary>
public enum ReportStatus
{
    Pending = 0,
    Resolved = 1,
    Rejected = 2,
    Cancelled = 3
}
