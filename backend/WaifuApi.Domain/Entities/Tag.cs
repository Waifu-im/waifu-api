using System.Collections.Generic;
using System.Text.Json.Serialization;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Domain.Entities;

public class Tag
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public required string Slug { get; set; }
    public required string Description { get; set; }
    public ReviewStatus ReviewStatus { get; set; } = ReviewStatus.Pending;

    public long? CreatorId { get; set; }
    public User? Creator { get; set; }

    [JsonIgnore]
    public List<Image> Images { get; set; } = new();
}