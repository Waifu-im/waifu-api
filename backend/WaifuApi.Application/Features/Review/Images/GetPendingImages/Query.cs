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

namespace WaifuApi.Application.Features.Review.Images.GetPendingImages;

public class GetPendingImagesQuery : IQuery<PaginatedList<ImageDto>>
{
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetPendingImagesQuery(int page, int pageSize)
    {
        Page = page;
        PageSize = pageSize;
    }
    
    public GetPendingImagesQuery() { }
}

public class GetPendingImagesQueryHandler : IQueryHandler<GetPendingImagesQuery, PaginatedList<ImageDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetPendingImagesQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        _defaultPageSize = int.Parse(configuration["Review:DefaultPageSize"] ?? throw new InvalidOperationException("Review:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Review:MaxPageSize"] ?? throw new InvalidOperationException("Review:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<ImageDto>> Handle(GetPendingImagesQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Images
            .AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artist)
            .Where(i => i.ReviewStatus == ReviewStatus.Pending)
            .OrderBy(i => i.UploadedAt);

        var paginatedImages = await PaginatedList<Domain.Entities.Image>.CreateAsync(query, request.Page, pageSize, cancellationToken);

        var imageDtos = paginatedImages.Items.Select(image => new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artist = image.Artist,
            UploaderId = image.UploaderId,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            Tags = image.Tags
        }).ToList();

        return new PaginatedList<ImageDto>(
            imageDtos,
            paginatedImages.TotalCount,
            paginatedImages.PageNumber,
            pageSize // Pass the actual pageSize used for calculation
        );
    }

    private static string BitArrayToHex(BitArray bits)
    {
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
