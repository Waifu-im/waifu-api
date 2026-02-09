using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.GetImageById;

public record GetImageByIdQuery(long Id, long UserId, bool IsModeratorOrAdmin = false, ReviewStatusFilter ChildReviewStatus = ReviewStatusFilter.Accepted) : IQuery<ImageDto>;

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
        var query = _context.Images.AsNoTracking();
        query = request.ChildReviewStatus switch
        {
            ReviewStatusFilter.All => query.Include(i => i.Tags).Include(i => i.Artists),
            ReviewStatusFilter.Pending => query
                .Include(i => i.Tags.Where(t => t.ReviewStatus == ReviewStatus.Pending))
                .Include(i => i.Artists.Where(a => a.ReviewStatus == ReviewStatus.Pending)),
            _ => query
                .Include(i => i.Tags.Where(t => t.ReviewStatus == ReviewStatus.Accepted))
                .Include(i => i.Artists.Where(a => a.ReviewStatus == ReviewStatus.Accepted))
        };

        var image = await query.FirstOrDefaultAsync(i => i.Id == request.Id, cancellationToken);

        if (image == null) throw new KeyNotFoundException($"Image with ID {request.Id} not found.");

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
            Artists = image.Artists.Select(a => a.ToDto()).ToList(),
            UploaderId = request.IsModeratorOrAdmin ? image.UploaderId : null,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            Tags = image.Tags.Select(t => t.ToDto()).ToList(),
            ReviewStatus = image.ReviewStatus,
            Favorites = favorites,
            LikedAt = likedAt,
            Albums = albums
        };
    }
}