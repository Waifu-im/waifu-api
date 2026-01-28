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

namespace WaifuApi.Application.Features.Users.GetUsers;

public class GetUsersQuery : IQuery<PaginatedList<User>>
{
    public string? Search { get; set; }
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetUsersQuery(string? search, int page, int pageSize)
    {
        Search = search;
        Page = page;
        PageSize = pageSize;
    }
    
    public GetUsersQuery() { }
}

public class GetUsersQueryHandler : IQueryHandler<GetUsersQuery, PaginatedList<User>>
{
    private readonly IWaifuDbContext _context;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetUsersQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _defaultPageSize = int.Parse(configuration["User:DefaultPageSize"] ?? throw new InvalidOperationException("User:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["User:MaxPageSize"] ?? throw new InvalidOperationException("User:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<User>> Handle(GetUsersQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Users.AsQueryable();

        if (!string.IsNullOrEmpty(request.Search))
        {
            query = query.Where(u => u.Name.Contains(request.Search) || u.DiscordId.Contains(request.Search));
        }
        
        query = query.OrderBy(u => u.Id);

        return await PaginatedList<User>.CreateAsync(query, request.Page, pageSize, cancellationToken);
    }
}
