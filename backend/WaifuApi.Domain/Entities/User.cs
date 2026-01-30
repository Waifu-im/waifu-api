using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace WaifuApi.Domain.Entities;

public class User
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public required string DiscordId { get; set; }
    public string? AvatarUrl { get; set; }
    public Role Role { get; set; } = Role.User;
    public bool IsBlacklisted { get; set; }
    public long RequestCount { get; set; }
    public long ApiKeyRequestCount { get; set; }
    public long JwtRequestCount { get; set; }
    
    [JsonIgnore]
    public List<ApiKey> ApiKeys { get; set; } = new();
}
