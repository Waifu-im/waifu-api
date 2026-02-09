using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Artists.GetArtists;

public class GetArtistsQuery : IQuery<PaginatedList<ArtistDto>>
{
    // Public filters (exposed via Request in Web layer)
    public string? Name { get; set; }
    public List<long> IncludedIds { get; set; } = new();
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    // Internal parameter (set by controller, never exposed via query string)
    public ReviewStatusFilter ReviewStatus { get; set; } = ReviewStatusFilter.Accepted;
    public Role? UserRole { get; set; }
}

public class GetArtistsQueryHandler : IQueryHandler<GetArtistsQuery, PaginatedList<ArtistDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetArtistsQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _defaultPageSize = int.Parse(configuration["Artist:DefaultPageSize"] ?? throw new InvalidOperationException("Artist:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Artist:MaxPageSize"] ?? throw new InvalidOperationException("Artist:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<ArtistDto>> Handle(GetArtistsQuery request, CancellationToken cancellationToken)
    {
        // Validate permission for non-Accepted review status filtering
        if (!RoleUtils.CanAccessNonAcceptedReviewStatus(request.UserRole) &&
            request.ReviewStatus != ReviewStatusFilter.Accepted)
        {
            throw new ForbiddenException("Filtering by non-accepted review status is only available to moderators and admins.");
        }

        var isModeratorOrAdmin = RoleUtils.IsModeratorOrAdmin(request.UserRole);
        var query = _context.Artists.AsNoTracking();

        // When includedIds is provided, fetch exactly those artists (ignore other filters and pagination)
        if (request.IncludedIds.Count > 0)
        {
            query = query.Where(a => request.IncludedIds.Contains(a.Id));

            var result = await query.Select(a => new ArtistDto
            {
                Id = a.Id,
                Name = a.Name,
                Patreon = a.Patreon,
                Pixiv = a.Pixiv,
                Twitter = a.Twitter,
                DeviantArt = a.DeviantArt,
                ReviewStatus = a.ReviewStatus,
                CreatorId = isModeratorOrAdmin ? a.CreatorId : null,
                ImageCount = a.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
            }).ToListAsync(cancellationToken);

            return new PaginatedList<ArtistDto>(result, result.Count, 1, result.Count, _maxPageSize, _defaultPageSize);
        }

        // Normal search/pagination flow
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        switch (request.ReviewStatus)
        {
            case ReviewStatusFilter.Pending:
                query = query.Where(a => a.ReviewStatus == ReviewStatus.Pending);
                break;
            case ReviewStatusFilter.All:
                break;
            default:
                query = query.Where(a => a.ReviewStatus == ReviewStatus.Accepted);
                break;
        }

        if (!string.IsNullOrWhiteSpace(request.Name))
        {
            query = query.Where(a => a.Name.ToLower().Contains(request.Name.ToLower()));
        }

        // Project to DTO with image count and order by image count descending
        var projectedQuery = query.Select(a => new ArtistDto
        {
            Id = a.Id,
            Name = a.Name,
            Patreon = a.Patreon,
            Pixiv = a.Pixiv,
            Twitter = a.Twitter,
            DeviantArt = a.DeviantArt,
            ReviewStatus = a.ReviewStatus,
            CreatorId = isModeratorOrAdmin ? a.CreatorId : null,
            ImageCount = a.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
        }).OrderByDescending(a => a.ImageCount);

        var count = await query.CountAsync(cancellationToken);
        var dtos = await projectedQuery.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

        return new PaginatedList<ArtistDto>(dtos, count, request.Page, pageSize, _maxPageSize, _defaultPageSize);
    }
}
