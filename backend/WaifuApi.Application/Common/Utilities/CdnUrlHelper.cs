namespace WaifuApi.Application.Common.Utilities;

public static class CdnUrlHelper
{
    public static string GetImageUrl(string cdnBaseUrl, long imageId, string extension)
    {
        return $"{cdnBaseUrl}/{imageId}{extension}";
    }
}
