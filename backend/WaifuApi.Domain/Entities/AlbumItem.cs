using System;

namespace WaifuApi.Domain.Entities;

public class AlbumItem
{
    public long AlbumId { get; set; }
    public Album Album { get; set; } = null!;
    
    public long ImageId { get; set; }
    public Image Image { get; set; } = null!;
    
    public DateTime AddedAt { get; set; }
}
