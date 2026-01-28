using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.GetAlbums;

public record GetAlbumsQuery(long UserId) : IQuery<List<Album>>;

public class GetAlbumsQueryHandler : IQueryHandler<GetAlbumsQuery, List<Album>>
{
    private readonly IWaifuDbContext _context;

    public GetAlbumsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<Album>> Handle(GetAlbumsQuery request, CancellationToken cancellationToken)
    {
        return await _context.Albums
            .Where(a => a.UserId == request.UserId)
            .OrderByDescending(a => a.IsDefault) // Default first
            .ThenBy(a => a.Name)
            .ToListAsync(cancellationToken);
    }
}
