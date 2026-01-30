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

public class GetUsersQuery : IQuery<PaginatedList<UserDto>>
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

public class GetUsersQueryHandler : IQueryHandler<GetUsersQuery, PaginatedList<UserDto>>
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

    public async ValueTask<PaginatedList<UserDto>> Handle(GetUsersQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Users.AsQueryable();

        if (!string.IsNullOrEmpty(request.Search))
        {
            query = query.Where(u => u.Name.ToLower().Contains(request.Search.ToLower()) || u.DiscordId.Contains(request.Search));
        }
        
        query = query.OrderBy(u => u.Id);

        var count = await query.CountAsync(cancellationToken);
        var items = await query.Skip((request.Page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

        var dtos = items.Select(u => new UserDto
        {
            Id = u.Id,
            Name = u.Name,
            DiscordId = u.DiscordId,
            AvatarUrl = u.AvatarUrl,
            Role = u.Role,
            IsBlacklisted = u.IsBlacklisted,
            RequestCount = u.RequestCount,
            ApiKeyRequestCount = u.ApiKeyRequestCount,
            JwtRequestCount = u.JwtRequestCount
        }).ToList();

        return new PaginatedList<UserDto>(dtos, count, request.Page, pageSize);
    }
}
