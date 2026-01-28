using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Images.GetPendingImages;

public record GetPendingImagesQuery : IQuery<List<ImageDto>>;

public class GetPendingImagesQueryHandler : IQueryHandler<GetPendingImagesQuery, List<ImageDto>>
{
    private readonly IWaifuDbContext _context;

    public GetPendingImagesQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<ImageDto>> Handle(GetPendingImagesQuery request, CancellationToken cancellationToken)
    {
        return await _context.Images
            .AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artist)
            .Where(i => i.ReviewStatus == ReviewStatus.Pending)
            .OrderBy(i => i.UploadedAt)
            .Select(image => new ImageDto
            {
                Id = image.Id,
                Signature = image.PerceptualHash,
                Extension = image.Extension,
                DominantColor = image.DominantColor,
                Source = image.Source,
                Artist = image.Artist,
                UploadedAt = image.UploadedAt,
                IsNsfw = image.IsNsfw,
                IsAnimated = image.IsAnimated,
                Width = image.Width,
                Height = image.Height,
                ByteSize = image.ByteSize,
                Url = "",
                Tags = image.Tags
            })
            .ToListAsync(cancellationToken);
    }
}
