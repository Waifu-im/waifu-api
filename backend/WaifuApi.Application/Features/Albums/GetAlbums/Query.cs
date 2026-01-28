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

namespace WaifuApi.Application.Features.Albums.GetAlbums;

public class GetAlbumsQuery : IQuery<PaginatedList<AlbumDto>>
{
    public long UserId { get; set; }
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetAlbumsQuery(long userId)
    {
        UserId = userId;
    }
    
    // Parameterless constructor for binding if needed, though usually not for records/classes with constructor
    public GetAlbumsQuery() { }
}

public class GetAlbumsQueryHandler : IQueryHandler<GetAlbumsQuery, PaginatedList<AlbumDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetAlbumsQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _defaultPageSize = int.Parse(configuration["Album:DefaultPageSize"] ?? throw new InvalidOperationException("Album:DefaultPageSize is required."));
        _maxPageSize = int.Parse(configuration["Album:MaxPageSize"] ?? throw new InvalidOperationException("Album:MaxPageSize is required."));
    }

    public async ValueTask<PaginatedList<AlbumDto>> Handle(GetAlbumsQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Albums
            .Where(a => a.UserId == request.UserId)
            .OrderByDescending(a => a.IsDefault) // Default first
            .ThenBy(a => a.Name)
            .Select(a => new AlbumDto
            {
                Id = a.Id,
                Name = a.Name,
                Description = a.Description,
                IsDefault = a.IsDefault,
                UserId = a.UserId
            });

        return await PaginatedList<AlbumDto>.CreateAsync(query, request.Page, pageSize, cancellationToken);
    }
}
