using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Text.Json.Serialization;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.GetImages;

public class GetImagesQuery : IQuery<PaginatedList<ImageDto>>
{
    public NsfwMode IsNsfw { get; set; } = NsfwMode.Safe;
    public List<string> IncludedTags { get; set; } = new();
    public List<string> ExcludedTags { get; set; } = new();
    public List<string> IncludedFiles { get; set; } = new();
    public List<string> ExcludedFiles { get; set; } = new();
    public bool? IsAnimated { get; set; }
    public string OrderBy { get; set; } = string.Empty;
    public string Orientation { get; set; } = string.Empty;
    
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
    
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;
    
    [JsonIgnore]
    public long UserId { get; set; }
    
    [JsonIgnore]
    public long? AlbumId { get; set; }
    
    public long? ArtistId { get; set; }
}

public class GetImagesQueryHandler : IQueryHandler<GetImagesQuery, PaginatedList<ImageDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;
    private readonly ICurrentUserService _currentUserService;

    public GetImagesQueryHandler(IWaifuDbContext context, IConfiguration configuration, ICurrentUserService currentUserService)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        
        // Use new config keys
        _defaultPageSize = int.Parse(configuration["Image:DefaultPageSize"] ?? throw new InvalidOperationException("Image:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Image:MaxPageSize"] ?? throw new InvalidOperationException("Image:MaxPageSize is required."));
        
        _currentUserService = currentUserService;
    }

    public async ValueTask<PaginatedList<ImageDto>> Handle(GetImagesQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        
        if (_maxPageSize > 0 && pageSize > _maxPageSize)
        {
            pageSize = _maxPageSize;
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
            Width = request.Width,
            Height = request.Height,
            ByteSize = request.ByteSize,
            UserId = request.UserId,
            AlbumId = request.AlbumId,
            ArtistId = request.ArtistId
        };

        IQueryable<Image> query = _context.Images.AsNoTracking();
        query = query.ApplyFilters(filters);

        List<Image> images;
        int totalCount;
        Dictionary<long, DateTime> addedToAlbumMap = new();

        if (filters.AlbumId.HasValue)
        {
            var joined = query.Join(_context.AlbumItems, i => i.Id, ai => ai.ImageId, (i, ai) => new { i, ai })
                .Where(x => x.ai.AlbumId == filters.AlbumId.Value);

            // Sorting
            if (request.OrderBy == "ADDED_TO_ALBUM")
            {
                joined = joined.OrderByDescending(x => x.ai.AddedAt);
            }
            else if (request.OrderBy == "UPLOADED_AT")
            {
                joined = joined.OrderByDescending(x => x.i.UploadedAt);
            }
            else if (request.OrderBy == "FAVORITES")
            {
                joined = joined.OrderByDescending(x => _context.AlbumItems.Count(ai => ai.ImageId == x.i.Id && ai.Album.IsDefault));
            }
            else
            {
                // Default to RANDOM if not specified or unknown
                joined = joined.OrderBy(x => EF.Functions.Random());
            }

            totalCount = await joined.CountAsync(cancellationToken);
            var pagedItems = await joined.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

            var imageIds = pagedItems.Select(x => x.i.Id).ToList();
            addedToAlbumMap = pagedItems.ToDictionary(x => x.i.Id, x => x.ai.AddedAt);

            var fetchedImages = await _context.Images
                .AsNoTracking()
                .Include(i => i.Tags)
                .Include(i => i.Artist)
                .Where(i => imageIds.Contains(i.Id))
                .ToListAsync(cancellationToken);

            // Re-order in memory
            images = imageIds.Select(id => fetchedImages.First(img => img.Id == id)).ToList();
        }
        else
        {
            // Standard path
            if (request.OrderBy == "UPLOADED_AT")
            {
                query = query.OrderByDescending(i => i.UploadedAt);
            }
            else if (request.OrderBy == "FAVORITES")
            {
                 query = query.OrderByDescending(i => _context.AlbumItems.Count(ai => ai.ImageId == i.Id && ai.Album.IsDefault));
            }
            else
            {
                // Default to RANDOM
                query = query.OrderBy(i => EF.Functions.Random());
            }

            totalCount = await query.CountAsync(cancellationToken);
            images = await query
                .Include(i => i.Tags)
                .Include(i => i.Artist)
                .Skip((request.Page - 1) * pageSize)
                .Take(pageSize)
                .ToListAsync(cancellationToken);
        }

        var imageIdsForStats = images.Select(i => i.Id).ToList();
        
        var favoritesCounts = await _context.AlbumItems
            .Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.IsDefault)
            .GroupBy(ai => ai.ImageId)
            .Select(g => new { ImageId = g.Key, Count = g.Count() })
            .ToDictionaryAsync(x => x.ImageId, x => x.Count, cancellationToken);

        var likedStatus = request.UserId > 0
            ? await _context.AlbumItems
                .Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.UserId == request.UserId && ai.Album.IsDefault)
                .Select(ai => new { ai.ImageId, ai.AddedAt })
                .ToDictionaryAsync(x => x.ImageId, x => x.AddedAt, cancellationToken)
            : new Dictionary<long, DateTime>();

        var userAlbumsMap = new Dictionary<long, List<AlbumDto>>();
        if (request.UserId > 0)
        {
            var userAlbums = await _context.AlbumItems
                .Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.UserId == request.UserId)
                .Select(ai => new { ai.ImageId, Album = new AlbumDto { Id = ai.Album.Id, Name = ai.Album.Name, UserId = ai.Album.UserId, IsDefault = ai.Album.IsDefault } })
                .ToListAsync(cancellationToken);

             foreach (var item in userAlbums)
             {
                 if (!userAlbumsMap.ContainsKey(item.ImageId))
                 {
                     userAlbumsMap[item.ImageId] = new List<AlbumDto>();
                 }
                 userAlbumsMap[item.ImageId].Add(item.Album);
             }
        }

        var imageDtos = images.Select(image => new ImageDto
        {
            Id = image.Id,
            PerceptualHash = BitArrayToHex(image.PerceptualHash),
            Extension = image.Extension,
            DominantColor = image.DominantColor,
            Source = image.Source,
            Artist = (image.Artist != null && image.Artist.ReviewStatus == ReviewStatus.Accepted) ? image.Artist : null,
            UploaderId = image.UploaderId,
            UploadedAt = image.UploadedAt,
            IsNsfw = image.IsNsfw,
            IsAnimated = image.IsAnimated,
            Width = image.Width,
            Height = image.Height,
            ByteSize = image.ByteSize,
            Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, image.Id, image.Extension),
            Tags = image.Tags.Where(t => t.ReviewStatus == ReviewStatus.Accepted).ToList(),
            Favorites = favoritesCounts.TryGetValue(image.Id, out var count) ? count : 0,
            LikedAt = likedStatus.TryGetValue(image.Id, out var date) ? (DateTime?)date : null,
            AddedToAlbumAt = addedToAlbumMap.TryGetValue(image.Id, out var addedAt) ? addedAt : null,
            Albums = userAlbumsMap.TryGetValue(image.Id, out var albums) ? albums : new List<AlbumDto>()
        }).ToList();

        return new PaginatedList<ImageDto>(imageDtos, totalCount, request.Page, pageSize);
    }

    private static string BitArrayToHex(BitArray bits)
    {
        var bytes = new byte[(bits.Length + 7) / 8];
        bits.CopyTo(bytes, 0);
        return Convert.ToHexString(bytes).ToLowerInvariant();
    }
}
