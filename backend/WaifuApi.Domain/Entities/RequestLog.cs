using System;

namespace WaifuApi.Domain.Entities;

public class RequestLog
{
    public int Id { get; set; }
    public string Endpoint { get; set; } = string.Empty;
    public string Method { get; set; } = string.Empty;
    public int StatusCode { get; set; }
    public DateTime Timestamp { get; set; }
    public string? IpAddress { get; set; }
    public string? UserAgent { get; set; }
    public int? UserId { get; set; }
}