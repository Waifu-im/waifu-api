using System;

namespace WaifuApi.Application.Common.Models;

public class ApiKeyDto
{
    public long Id { get; set; }
    public string Key { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
    public DateTime CreatedAt { get; set; }
    public DateTime? LastUsedAt { get; set; }
    public DateTime? ExpirationDate { get; set; }
    public long UserId { get; set; }
}
