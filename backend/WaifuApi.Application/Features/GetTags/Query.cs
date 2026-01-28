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

namespace WaifuApi.Application.Features.GetTags;

public class GetTagsQuery : IQuery<PaginatedList<Tag>>
{
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
}

public class GetTagsQueryHandler : IQueryHandler<GetTagsQuery, PaginatedList<Tag>>
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

    public async ValueTask<PaginatedList<Tag>> Handle(GetTagsQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Tags
            .AsNoTracking()
            .Where(t => t.ReviewStatus == ReviewStatus.Accepted);

        return await PaginatedList<Tag>.CreateAsync(query, request.Page, pageSize, cancellationToken);
    }
}
