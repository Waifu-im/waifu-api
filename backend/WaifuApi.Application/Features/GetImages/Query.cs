using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.GetImages;

public class GetImagesQuery : IQuery<List<ImageDto>>
{
    public NsfwMode IsNsfw { get; set; } = NsfwMode.Safe;
    public List<string> IncludedTags { get; set; } = new();
    public List<string> ExcludedTags { get; set; } = new();
    public List<string> IncludedFiles { get; set; } = new();
    public List<string> ExcludedFiles { get; set; } = new();
    public bool? IsAnimated { get; set; }
    public string OrderBy { get; set; } = string.Empty;
    public string Orientation { get; set; } = string.Empty;
    public int Limit { get; set; }
    public bool Full { get; set; }
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;
    public long UserId { get; set; }
}

public class GetImagesQueryHandler : IQueryHandler<GetImagesQuery, List<ImageDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public GetImagesQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? "https://cdn.waifu.im";
    }

    public async ValueTask<List<ImageDto>> Handle(GetImagesQuery request, CancellationToken cancellationToken)
    {
        var filters = new ImageFilters
        {
            IsNsfw = request.IsNsfw,
            IncludedTags = request.IncludedTags,
            ExcludedTags = request.ExcludedTags,
            IncludedFiles = request.IncludedFiles,
            ExcludedFiles = request.ExcludedFiles,
            IsAnimated = request.IsAnimated,
            OrderBy = request.OrderBy,
            Orientation = request.Orientation,
            Limit = request.Limit,
            Full = request.Full,
            Width = request.Width,
            Height = request.Height,
            ByteSize = request.ByteSize,
            UserId = request.UserId
        };

        var query = _context.Images
            .AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artist)
            .AsQueryable();

        query = query.ApplyFilters(filters);

        if (filters.Limit > 0)
        {
            query = query.Take(filters.Limit);
        }

        return await query.Select(image => new ImageDto
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
            Url = $"{_cdnBaseUrl}/{image.Id}{image.Extension}",
            Tags = image.Tags,
            Favorites = _context.AlbumItems.Count(ai => ai.ImageId == image.Id && ai.Album.IsDefault),
            LikedAt = request.UserId > 0 
                ? _context.AlbumItems
                    .Where(ai => ai.ImageId == image.Id && ai.Album.UserId == request.UserId && ai.Album.IsDefault)
                    .Select(ai => (DateTime?)ai.AddedAt)
                    .FirstOrDefault()
                : null
        }).ToListAsync(cancellationToken);
    }
}
