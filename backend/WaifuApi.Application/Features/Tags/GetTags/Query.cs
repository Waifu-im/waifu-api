using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Models;
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
    public ReviewStatus? ReviewStatus { get; set; }
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
                ImageCount = t.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
            }).ToListAsync(cancellationToken);

            return new PaginatedList<TagDto>(result, result.Count, 1, result.Count);
        }

        // Normal search/pagination flow
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        if (request.ReviewStatus.HasValue)
        {
            query = query.Where(t => t.ReviewStatus == request.ReviewStatus.Value);
        }
        else
        {
            query = query.Where(t => t.ReviewStatus == ReviewStatus.Accepted);
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
            ImageCount = t.Images.Count(i => i.ReviewStatus == ReviewStatus.Accepted)
        }).OrderByDescending(t => t.ImageCount);

        var count = await query.CountAsync(cancellationToken);
        var dtos = await projectedQuery.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

        return new PaginatedList<TagDto>(dtos, count, request.Page, pageSize);
    }
}
