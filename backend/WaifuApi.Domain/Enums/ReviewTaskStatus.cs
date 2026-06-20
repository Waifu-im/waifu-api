namespace WaifuApi.Domain.Enums;

/// <summary>
/// Lifecycle status of a review task. The task keeps a durable audit record even after the
/// content it concerns is accepted or deleted.
/// </summary>
public enum ReviewTaskStatus
{
    Pending = 0,
    Approved = 1,
    Rejected = 2,
    Cancelled = 3
}
