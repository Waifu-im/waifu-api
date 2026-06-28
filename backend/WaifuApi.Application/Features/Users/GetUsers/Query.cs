using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Users.GetUsers;

public class GetUsersQuery : IQuery<PaginatedList<UserDto>>
{
    public string? Name { get; set; }
    public List<long> IncludedIds { get; set; } = new();
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }
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
        var query = _context.Users.AsQueryable();

        // When includedIds is provided, fetch exactly those users (ignore other filters and pagination)
        if (request.IncludedIds.Count > 0)
        {
            query = query.Where(u => request.IncludedIds.Contains(u.Id));

            var result = await query.Select(u => new UserDto
            {
                Id = u.Id,
                Name = u.Name,
                DiscordId = u.DiscordId,
                AvatarUrl = u.AvatarUrl,
                Role = u.Role,
                IsBlacklisted = u.IsBlacklisted,
                BlacklistReason = u.BlacklistReason,
                RequestCount = u.RequestCount,
                ApiKeyRequestCount = u.ApiKeyRequestCount,
                JwtRequestCount = u.JwtRequestCount,
                UploadedImageCount = _context.Images.Count(i => i.UploaderId == u.Id && i.ReviewStatus == ReviewStatus.Accepted),
                AlbumImageCount = _context.AlbumItems.Count(ai => _context.Albums.Any(a => a.Id == ai.AlbumId && a.UserId == u.Id))
            }).ToListAsync(cancellationToken);

            return new PaginatedList<UserDto>(result, result.Count, 1, result.Count, _maxPageSize, _defaultPageSize);
        }

        // Normal search/pagination flow
        var (page, pageSize) = PaginationUtils.Normalize(request.Page, request.PageSize, _defaultPageSize, _maxPageSize);

        if (!string.IsNullOrEmpty(request.Name))
        {
            query = query.Where(u => u.Name.ToLower().Contains(request.Name.ToLower()) || u.DiscordId.Contains(request.Name));
        }

        // Project to DTO with uploaded image count and order by it descending
        var projectedQuery = query.Select(u => new UserDto
        {
            Id = u.Id,
            Name = u.Name,
            DiscordId = u.DiscordId,
            AvatarUrl = u.AvatarUrl,
            Role = u.Role,
            IsBlacklisted = u.IsBlacklisted,
            BlacklistReason = u.BlacklistReason,
            RequestCount = u.RequestCount,
            ApiKeyRequestCount = u.ApiKeyRequestCount,
            JwtRequestCount = u.JwtRequestCount,
            UploadedImageCount = _context.Images.Count(i => i.UploaderId == u.Id && i.ReviewStatus == ReviewStatus.Accepted),
            AlbumImageCount = _context.AlbumItems.Count(ai => _context.Albums.Any(a => a.Id == ai.AlbumId && a.UserId == u.Id))
        }).OrderByDescending(u => u.UploadedImageCount);

        var count = await query.CountAsync(cancellationToken);
        var dtos = await projectedQuery.Skip((page - 1) * pageSize).Take(pageSize).ToListAsync(cancellationToken);

        return new PaginatedList<UserDto>(dtos, count, page, pageSize, _maxPageSize, _defaultPageSize);
    }
}