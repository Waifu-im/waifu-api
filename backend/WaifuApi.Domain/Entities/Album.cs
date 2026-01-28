using System.Collections.Generic;
using System.Text.Json.Serialization;

namespace WaifuApi.Domain.Entities;

public class Album
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public string Description { get; set; } = string.Empty;
    public bool IsDefault { get; set; }
    
    public long UserId { get; set; }
    
    [JsonIgnore]
    public User User { get; set; } = null!;
    
    public List<AlbumItem> Items { get; set; } = new();
}
