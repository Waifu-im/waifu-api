using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using FluentValidation;
using FluentValidation.Results;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.GetImages;

public class GetImagesQuery : IQuery<PaginatedList<ImageDto>>
{
    // Public filters (exposed via Request in Web layer)
    public NsfwMode IsNsfw { get; set; } = NsfwMode.False;
    public List<string> IncludedTags { get; set; } = new();
    public List<string> ExcludedTags { get; set; } = new();
    public List<string> IncludedArtists { get; set; } = new();
    public List<string> ExcludedArtists { get; set; } = new();
    public List<string> IncludedIds { get; set; } = new();
    public List<string> ExcludedIds { get; set; } = new();
    public AnimatedMode IsAnimated { get; set; } = AnimatedMode.All;
    public ImageOrderBy OrderBy { get; set; } = ImageOrderBy.Random;
    public Orientation Orientation { get; set; } = Orientation.All;
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
    public string Width { get; set; } = string.Empty;
    public string Height { get; set; } = string.Empty;
    public string ByteSize { get; set; } = string.Empty;

    // Internal parameters (set by controller, never exposed via query string)
    public long? UserId { get; set; }
    public long? AlbumId { get; set; }
    public ReviewStatus? ReviewStatus { get; set; }

    // Moderator/Admin only parameters
    public long? UploaderId { get; set; }
    public bool IsModeratorOrAdmin { get; set; }
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
        if (request.OrderBy == ImageOrderBy.AddedToAlbum && !request.AlbumId.HasValue)
        {
            throw new ValidationException(new[]
            {
                new ValidationFailure("orderBy", "AddedToAlbum ordering is only available when viewing an album.")
            });
        }

        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var filters = new ImageFilters
        {
            IsNsfw = request.IsNsfw,
            IncludedTags = request.IncludedTags,
            ExcludedTags = request.ExcludedTags,
            IncludedArtists = request.IncludedArtists,
            ExcludedArtists = request.ExcludedArtists,
            IncludedIds = request.IncludedIds,
            ExcludedIds = request.ExcludedIds,
            IsAnimated = request.IsAnimated,
            OrderBy = request.OrderBy,
            Orientation = request.Orientation,
            Width = request.Width,
            Height = request.Height,
            ByteSize = request.ByteSize,
            UserId = request.UserId,
            AlbumId = request.AlbumId,
            ReviewStatus = request.ReviewStatus,
            UploaderId = request.UploaderId
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
             joined = request.OrderBy switch
             {
                 ImageOrderBy.AddedToAlbum => joined.OrderByDescending(x => x.ai.AddedAt),
                 ImageOrderBy.UploadedAt => joined.OrderByDescending(x => x.i.UploadedAt),
                 ImageOrderBy.Favorites => joined.OrderByDescending(x => _context.AlbumItems.Count(ai => ai.ImageId == x.i.Id && ai.Album.IsDefault)),
                 _ => joined.OrderBy(x => EF.Functions.Random())
             };

             totalCount = await joined.CountAsync(cancellationToken);
             var pagedItems = await joined.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);
             var imageIds = pagedItems.Select(x => x.i.Id).ToList();
             addedToAlbumMap = pagedItems.ToDictionary(x => x.i.Id, x => x.ai.AddedAt);

             var fetchedImages = await _context.Images.AsNoTracking().Include(i => i.Tags).Include(i => i.Artists).Where(i => imageIds.Contains(i.Id)).ToListAsync(cancellationToken);
             images = imageIds.Select(id => fetchedImages.First(img => img.Id == id)).ToList();
        }
        else
        {
            query = request.OrderBy switch
            {
                ImageOrderBy.UploadedAt => query.OrderByDescending(i => i.UploadedAt),
                ImageOrderBy.Favorites => query.OrderByDescending(i => _context.AlbumItems.Count(ai => ai.ImageId == i.Id && ai.Album.IsDefault)),
                _ => query.OrderBy(i => EF.Functions.Random())
            };

            totalCount = await query.CountAsync(cancellationToken);
            images = await query.Include(i => i.Tags).Include(i => i.Artists).Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);
        }

        var imageIdsForStats = images.Select(i => i.Id).ToList();
        var favoritesCounts = await _context.AlbumItems.Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.IsDefault).GroupBy(ai => ai.ImageId).Select(g => new { ImageId = g.Key, Count = g.Count() }).ToDictionaryAsync(x => x.ImageId, x => x.Count, cancellationToken);
        var userId = request.UserId ?? 0;
        var likedStatus = userId > 0 ? await _context.AlbumItems.Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.UserId == userId && ai.Album.IsDefault).Select(ai => new { ai.ImageId, ai.AddedAt }).ToDictionaryAsync(x => x.ImageId, x => x.AddedAt, cancellationToken) : new Dictionary<long, DateTime>();
        var userAlbumsMap = new Dictionary<long, List<AlbumDto>>();
        if (userId > 0) {
             var userAlbums = await _context.AlbumItems.Where(ai => imageIdsForStats.Contains(ai.ImageId) && ai.Album.UserId == userId).Select(ai => new { ai.ImageId, Album = new AlbumDto { Id = ai.Album.Id, Name = ai.Album.Name, UserId = ai.Album.UserId, IsDefault = ai.Album.IsDefault } }).ToListAsync(cancellationToken);
             foreach (var item in userAlbums) { if (!userAlbumsMap.ContainsKey(item.ImageId)) userAlbumsMap[item.ImageId] = new List<AlbumDto>(); userAlbumsMap[item.ImageId].Add(item.Album); }
        }

        var imageDtos = images.Select(image =>
        {
            var dto = image.ToDto(_cdnBaseUrl, request.IsModeratorOrAdmin);
            dto.Favorites = favoritesCounts.TryGetValue(image.Id, out var count) ? count : 0;
            dto.LikedAt = likedStatus.TryGetValue(image.Id, out var date) ? (DateTime?)date : null;
            dto.AddedToAlbumAt = addedToAlbumMap.TryGetValue(image.Id, out var addedAt) ? addedAt : null;
            dto.Albums = userAlbumsMap.TryGetValue(image.Id, out var albums) ? albums : new List<AlbumDto>();
            return dto;
        }).ToList();

        return new PaginatedList<ImageDto>(imageDtos, totalCount, request.Page, pageSize, _maxPageSize, _defaultPageSize);
    }
}
