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
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Artists.GetPendingArtists;

public class GetPendingArtistsQuery : IQuery<PaginatedList<Artist>>
{
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetPendingArtistsQuery(int page, int pageSize)
    {
        Page = page;
        PageSize = pageSize;
    }
    
    public GetPendingArtistsQuery() { }
}

public class GetPendingArtistsQueryHandler : IQueryHandler<GetPendingArtistsQuery, PaginatedList<Artist>>
{
    private readonly IWaifuDbContext _context;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetPendingArtistsQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _defaultPageSize = int.Parse(configuration["Review:DefaultPageSize"] ?? throw new InvalidOperationException("Review:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Review:MaxPageSize"] ?? throw new InvalidOperationException("Review:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<Artist>> Handle(GetPendingArtistsQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Artists
            .Where(a => a.ReviewStatus == ReviewStatus.Pending)
            .OrderBy(a => a.Id);

        return await PaginatedList<Artist>.CreateAsync(query, request.Page, pageSize, cancellationToken);
    }
}
