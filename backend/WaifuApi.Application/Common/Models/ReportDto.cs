using System;

namespace WaifuApi.Application.Common.Models;

public class ReportDto
{
    public long Id { get; set; }
    public long UserId { get; set; }
    public UserMinimalDto? User { get; set; }
    public long ImageId { get; set; }
    public ImageDto? Image { get; set; }
    public string? Description { get; set; }
    public bool IsResolved { get; set; }
    public DateTime CreatedAt { get; set; }
}
