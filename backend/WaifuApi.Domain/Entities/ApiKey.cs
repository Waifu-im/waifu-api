using System;
using System.Text.Json.Serialization;

namespace WaifuApi.Domain.Entities;

public class ApiKey
{
    public long Id { get; set; }
    public required string Key { get; set; }
    public required string Description { get; set; }
    public DateTime CreatedAt { get; set; }
    public DateTime? LastUsedAt { get; set; }
    public DateTime? ExpirationDate { get; set; }
    public long UserId { get; set; }
    
    [JsonIgnore]
    public User User { get; set; } = null!;
}
