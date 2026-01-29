using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

public class TagDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public string Slug { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
    public ReviewStatus? ReviewStatus { get; set; }
}