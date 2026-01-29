using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.GetImageById;

public record GetImageByIdQuery(long Id, long UserId) : IQuery<ImageDto>;

public class GetImageByIdQueryHandler : IQueryHandler<GetImageByIdQuery, ImageDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly string _frontendBaseUrl;

    public GetImageByIdQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? "https://cdn.waifu.im";
        _frontendBaseUrl = configuration["Frontend:BaseUrl"] ?? "https://waifu.im";
    }

    public async ValueTask<ImageDto> Handle(GetImageByIdQuery request, CancellationToken cancellationToken)
    {
        var image = await _context.Images
            .AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == request.Id, cancellationToken);

        if (image == null) throw new KeyNotFoundException($"Image with ID {request.Id} not found.");

        // ... (Stats Logic inchangée)
        var favorites = await _context.AlbumItems.CountAsync(ai => ai.ImageId == image.Id && ai.Album.IsDefault, cancellationToken);
        var likedAt = request.UserId > 0 ? await _context.AlbumItems.Where(ai => ai.ImageId == image.Id && ai.Album.UserId == request.UserId && ai.Album.IsDefault).Select(ai => (DateTime?)ai.AddedAt).FirstOrDefaultAsync(cancellationToken) : null;
        var albums = request.UserId > 0 ? await _context.AlbumItems.Where(ai => ai.ImageId == image.Id && ai.Album.UserId == request.UserId).Select(ai => new AlbumDto { Id = ai.Album.Id, Name = ai.Album.Name, UserId = ai.Album.UserId, IsDefault = ai.Album.IsDefault }).ToListAsync(cancellationToken) : new List<AlbumDto>();

        return new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayHelper.ToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artists = image.Artists.Where(a => a.ReviewStatus == ReviewStatus.Accepted).ToList(),
            UploaderId = image.UploaderId,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            // Mapping Tag -> TagDto
            Tags = image.Tags.Where(t => t.ReviewStatus == ReviewStatus.Accepted).Select(t => new TagDto 
            { 
                Id = t.Id, 
                Name = t.Name, 
                Slug = t.Slug, 
                Description = t.Description, 
                ReviewStatus = t.ReviewStatus 
            }).ToList(),
            Favorites = favorites,
            LikedAt = likedAt,
            Albums = albums
        };
    }
}