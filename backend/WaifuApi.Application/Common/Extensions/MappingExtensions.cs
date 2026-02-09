using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Common.Extensions;

public static class MappingExtensions
{
    public static ArtistDto ToDto(this Artist artist)
    {
        return new ArtistDto
        {
            Id = artist.Id,
            Name = artist.Name,
            Patreon = artist.Patreon,
            Pixiv = artist.Pixiv,
            Twitter = artist.Twitter,
            DeviantArt = artist.DeviantArt,
            ReviewStatus = artist.ReviewStatus
        };
    }

    public static TagDto ToDto(this Tag tag)
    {
        return new TagDto
        {
            Id = tag.Id,
            Name = tag.Name,
            Slug = tag.Slug,
            Description = tag.Description,
            ReviewStatus = tag.ReviewStatus
        };
    }

    public static ImageDto ToDto(this Image image, string cdnBaseUrl, bool includeUploaderId = false)
    {
        return new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayHelper.ToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artists = image.Artists.Select(a => a.ToDto()).ToList(),
            UploaderId = includeUploaderId ? image.UploaderId : null,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(cdnBaseUrl, image.Id, image.Extension),
            Tags = image.Tags.Select(t => t.ToDto()).ToList(),
            ReviewStatus = image.ReviewStatus
        };
    }
}
