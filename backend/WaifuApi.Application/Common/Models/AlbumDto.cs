namespace WaifuApi.Application.Common.Models;

public class AlbumDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
    public bool IsDefault { get; set; }
    public long UserId { get; set; }
    public int ImageCount { get; set; }
}
