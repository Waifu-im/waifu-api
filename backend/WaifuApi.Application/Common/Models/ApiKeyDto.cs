using System;

namespace WaifuApi.Application.Common.Models;

public class ApiKeyDto
{
    public long Id { get; set; }
    public string Description { get; set; } = string.Empty;
    public DateTime CreatedAt { get; set; }
    public DateTime? LastUsedAt { get; set; }
    public DateTime? ExpirationDate { get; set; }
    public long UserId { get; set; }
}

/// <summary>
/// Returned only once at creation time. Contains the raw API key.
/// </summary>
public class CreatedApiKeyDto : ApiKeyDto
{
    /// <summary>
    /// The full raw API key. Store it securely — it will not be shown again.
    /// </summary>
    public string Key { get; set; } = string.Empty;
}
