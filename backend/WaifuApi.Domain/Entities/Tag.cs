using System.Collections.Generic;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

public class Tag
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public required string Description { get; set; }
    public bool IsNsfw { get; set; }
    public ReviewStatus ReviewStatus { get; set; } = ReviewStatus.Pending;
    
    public List<Image> Images { get; set; } = new();
}
