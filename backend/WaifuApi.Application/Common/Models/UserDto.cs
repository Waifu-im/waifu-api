using System.Collections.Generic;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Common.Models;

public class UserDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public string DiscordId { get; set; } = string.Empty;
    public string? AvatarUrl { get; set; }
    public Role Role { get; set; }
    public bool IsBlacklisted { get; set; }
    public long RequestCount { get; set; }
    public long ApiKeyRequestCount { get; set; }
    public long JwtRequestCount { get; set; }
}

public class UserMinimalDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public string? AvatarUrl { get; set; }
}
