using System.ComponentModel;

namespace WaifuApi.Domain.Enums;

public enum ImageOrderBy
{
    [Description("Random order")]
    Random = 0,

    [Description("Most recent uploads first")]
    UploadedAt = 1,

    [Description("Most favorited first")]
    Favorites = 2,

    [Description("Most recently added to album (album endpoints only)")]
    AddedToAlbum = 3
}
