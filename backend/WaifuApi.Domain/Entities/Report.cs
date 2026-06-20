using System;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

public class Report
{
    public long Id { get; set; }
    public long UserId { get; set; }
    public User User { get; set; } = null!;

    /// <summary>Nullable so a report survives its image being deleted (the link is set to null instead of cascading).</summary>
    public long? ImageId { get; set; }
    public Image? Image { get; set; }

    public required string Description { get; set; }
    public DateTime CreatedAt { get; set; }
    public ReportStatus Status { get; set; } = ReportStatus.Pending;

    /// <summary>Optional moderator note recorded when the report is answered (validated/rejected), shown to the reporter.</summary>
    public string? ReviewerNote { get; set; }
}
