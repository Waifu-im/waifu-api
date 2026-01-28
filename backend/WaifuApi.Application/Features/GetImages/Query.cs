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
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
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
    public string Limit { get; set; } = "1";
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;
    public long UserId { get; set; }
}

public class GetImagesQueryHandler : IQueryHandler<GetImagesQuery, List<ImageDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly int _maxLimit;
    private readonly ICurrentUserService _currentUserService;

    public GetImagesQueryHandler(IWaifuDbContext context, IConfiguration configuration, ICurrentUserService currentUserService)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        _maxLimit = int.Parse(configuration["Image:MaxLimit"] ?? throw new InvalidOperationException("Image:MaxLimit is required."));
        _currentUserService = currentUserService;
    }

    public async ValueTask<List<ImageDto>> Handle(GetImagesQuery request, CancellationToken cancellationToken)
    {
        // Parse Limit
        int limit = 1;
        if (request.Limit.Equals("all", StringComparison.OrdinalIgnoreCase))
        {
            var userId = _currentUserService.UserId;
            if (userId.HasValue)
            {
                var user = await _context.Users.FindAsync(new object[] { userId.Value }, cancellationToken);
                if (user != null && user.Role == Role.Admin)
                {
                    limit = int.MaxValue;
                }
                else
                {
                    limit = _maxLimit;
                }
            }
            else
            {
                limit = _maxLimit;
            }
        }
        else if (int.TryParse(request.Limit, out var parsedLimit))
        {
            limit = parsedLimit;
            if (limit > _maxLimit)
            {
                limit = _maxLimit;
            }
        }
        
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
            Limit = limit,
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

        if (limit < int.MaxValue)
        {
            query = query.Take(limit);
        }

        // Fetch data first, then map to DTO in memory
        var images = await query.ToListAsync(cancellationToken);

        var imageIds = images.Select(i => i.Id).ToList();
        
        var favoritesCounts = await _context.AlbumItems
            .Where(ai => imageIds.Contains(ai.ImageId) && ai.Album.IsDefault)
            .GroupBy(ai => ai.ImageId)
            .Select(g => new { ImageId = g.Key, Count = g.Count() })
            .ToDictionaryAsync(x => x.ImageId, x => x.Count, cancellationToken);

        var likedStatus = request.UserId > 0
            ? await _context.AlbumItems
                .Where(ai => imageIds.Contains(ai.ImageId) && ai.Album.UserId == request.UserId && ai.Album.IsDefault)
                .Select(ai => new { ai.ImageId, ai.AddedAt })
                .ToDictionaryAsync(x => x.ImageId, x => x.AddedAt, cancellationToken)
            : new Dictionary<long, DateTime>();

        return images.Select(image => new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artist = (image.Artist != null && image.Artist.ReviewStatus == ReviewStatus.Accepted) ? image.Artist : null,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = $"{_cdnBaseUrl}/{image.Id}{image.Extension}",
            Tags = image.Tags.Where(t => t.ReviewStatus == ReviewStatus.Accepted).ToList(),
            Favorites = favoritesCounts.TryGetValue(image.Id, out var count) ? count : 0,
            LikedAt = likedStatus.TryGetValue(image.Id, out var date) ? (DateTime?)date : null
        }).ToList();
    }

    private static string BitArrayToHex(BitArray bits)
    {
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
