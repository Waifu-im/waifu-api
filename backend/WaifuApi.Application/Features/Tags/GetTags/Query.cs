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
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Tags.GetTags;

public class GetTagsQuery : IQuery<PaginatedList<TagDto>>
{
    // Public filters (exposed via Request in Web layer)
    public string? Name { get; set; }
    public List<long> IncludedIds { get; set; } = new();
    public List<string> IncludedSlugs { get; set; } = new();
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    // Internal parameter (set by controller, never exposed via query string)
    public ReviewStatusFilter ReviewStatus { get; set; } = ReviewStatusFilter.Accepted;
    public Role? UserRole { get; set; }
    public long? UserId { get; set; }

    /// <summary>
    /// Adds the caller's own pending tags onto an otherwise-accepted result (no moderator access needed).
    /// Takes precedence over the default accepted-only view; redundant when an explicit Pending/All is requested.
    /// </summary>
    public bool IncludeMyPending { get; set; }
}

public class GetTagsQueryHandler : IQueryHandler<GetTagsQuery, PaginatedList<TagDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetTagsQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _defaultPageSize = int.Parse(configuration["Tag:DefaultPageSize"] ?? throw new InvalidOperationException("Tag:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Tag:MaxPageSize"] ?? throw new InvalidOperationException("Tag:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<TagDto>> Handle(GetTagsQuery request, CancellationToken cancellationToken)
    {
        // Non-accepted review status stays moderator/admin-only. IncludeMyPending is separate and always allowed:
        // it only unions the caller's OWN pending tags onto an otherwise-accepted result.
        if (!RoleUtils.CanAccessNonAcceptedReviewStatus(request.UserRole) &&
            request.ReviewStatus != ReviewStatusFilter.Accepted)
        {
            throw new ForbiddenException("Filtering by non-accepted review status is only available to moderators and admins.");
        }

        var isModeratorOrAdmin = RoleUtils.IsModeratorOrAdmin(request.UserRole);
        var query = _context.Tags.AsNoTracking();

        // When includedIds or includedSlugs is provided, fetch exactly those tags (ignore other filters and pagination)
        if (request.IncludedIds.Count > 0 || request.IncludedSlugs.Count > 0)
        {
            if (request.IncludedIds.Count > 0)
            {
                query = query.Where(t => request.IncludedIds.Contains(t.Id));
            }
            else
            {
                query = query.Where(t => request.IncludedSlugs.Contains(t.Slug));
            }

            var result = await query.Select(t => new TagDto
            {
                Id = t.Id,
                Name = t.Name,
                Slug = t.Slug,
                Description = t.Description,
                ReviewStatus = t.ReviewStatus,
                CreatorId = isModeratorOrAdmin ? t.CreatorId : null,
                ImageCount = t.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
            }).ToListAsync(cancellationToken);

            return new PaginatedList<TagDto>(result, result.Count, 1, result.Count, _maxPageSize, _defaultPageSize);
        }

        // Normal search/pagination flow
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        switch (request.ReviewStatus)
        {
            case ReviewStatusFilter.Pending:
                query = query.Where(t => t.ReviewStatus == ReviewStatus.Pending);
                break;
            case ReviewStatusFilter.All:
                break;
            default:
                // Accepted, plus the caller's own pending tags when IncludeMyPending is set.
                if (request.IncludeMyPending && request.UserId is long uid)
                {
                    query = query.Where(t => t.ReviewStatus == ReviewStatus.Accepted
                        || (t.ReviewStatus == ReviewStatus.Pending && t.CreatorId == uid));
                }
                else
                {
                    query = query.Where(t => t.ReviewStatus == ReviewStatus.Accepted);
                }
                break;
        }

        if (!string.IsNullOrWhiteSpace(request.Name))
        {
            query = query.Where(t => t.Name.ToLower().Contains(request.Name.ToLower()));
        }

        // Project to DTO with image count and order by image count descending
        var projectedQuery = query.Select(t => new TagDto
        {
            Id = t.Id,
            Name = t.Name,
            Slug = t.Slug,
            Description = t.Description,
            ReviewStatus = t.ReviewStatus,
            CreatorId = isModeratorOrAdmin ? t.CreatorId : null,
            ImageCount = t.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
        }).OrderByDescending(t => t.ImageCount);

        var count = await query.CountAsync(cancellationToken);
        var dtos = await projectedQuery.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

        return new PaginatedList<TagDto>(dtos, count, request.Page, pageSize, _maxPageSize, _defaultPageSize);
    }
}
