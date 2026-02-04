using System;

namespace WaifuApi.Domain.Entities;

public class Report
{
    public long Id { get; set; }
    public long UserId { get; set; }
    public User User { get; set; } = null!;
    public long ImageId { get; set; }
    public Image Image { get; set; } = null!;
    public required string Description { get; set; }
    public DateTime CreatedAt { get; set; }
    public bool IsResolved { get; set; }
}
