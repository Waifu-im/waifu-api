using System;
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

namespace WaifuApi.Application.Features.Artists.GetArtists;

public class GetArtistsQuery : IQuery<PaginatedList<Artist>>
{
    public string? Search { get; set; }
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
}

public class GetArtistsQueryHandler : IQueryHandler<GetArtistsQuery, PaginatedList<Artist>>
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

    public async ValueTask<PaginatedList<Artist>> Handle(GetArtistsQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Artists
            .AsNoTracking()
            .Where(a => a.ReviewStatus == ReviewStatus.Accepted);

        if (!string.IsNullOrWhiteSpace(request.Search))
        {
            query = query.Where(a => a.Name.Contains(request.Search));
        }

        query = query.OrderBy(a => a.Name);

        return await PaginatedList<Artist>.CreateAsync(query, request.Page, pageSize, cancellationToken);
    }
}