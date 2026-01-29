using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Models;

public class ArtistDto
{
    public long Id { get; set; }
    public required string Name { get; set; }
    public string? Patreon { get; set; }
    public string? Pixiv { get; set; }
    public string? Twitter { get; set; }
    public string? DeviantArt { get; set; }
    public ReviewStatus ReviewStatus { get; set; }
}