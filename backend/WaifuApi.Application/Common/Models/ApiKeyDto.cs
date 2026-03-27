using System;
using System.Text.Json.Serialization;

namespace WaifuApi.Application.Common.Models;

public class ApiKeyDto
{
    public long Id { get; set; }

    /// <summary>
    /// The full raw API key. Only populated once, immediately after creation. Null on subsequent reads.
    /// </summary>
    [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
    public string? Key { get; set; }

    public string Description { get; set; } = string.Empty;
    public DateTime CreatedAt { get; set; }
    public DateTime? LastUsedAt { get; set; }
    public DateTime? ExpirationDate { get; set; }
    public long UserId { get; set; }
}
