using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace WaifuApi.Domain.Entities;

public class User
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public string? Email { get; set; }
    public required string DiscordId { get; set; }
    public Role Role { get; set; } = Role.User;
    public bool IsBlacklisted { get; set; }
    
    [JsonIgnore]
    public List<ApiKey> ApiKeys { get; set; } = new();
}
