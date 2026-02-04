namespace WaifuApi.Domain.Entities;

public class ImageTag
{
    public long ImageId { get; set; }
    public Image Image { get; set; } = null!;

    public long TagId { get; set; }
    public Tag Tag { get; set; } = null!;
}
