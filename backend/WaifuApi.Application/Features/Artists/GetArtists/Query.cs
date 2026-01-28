using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Artists.GetArtists;

public class GetArtistsQuery : IQuery<PaginatedList<Artist>>
{
    public int PageNumber { get; set; } = 1;
    public int PageSize { get; set; } = 10;
}

public class GetArtistsQueryHandler : IQueryHandler<GetArtistsQuery, PaginatedList<Artist>>
{
    private readonly IWaifuDbContext _context;

    public GetArtistsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<PaginatedList<Artist>> Handle(GetArtistsQuery request, CancellationToken cancellationToken)
    {
        var query = _context.Artists
            .AsNoTracking()
            .Where(a => a.ReviewStatus == ReviewStatus.Accepted)
            .OrderBy(a => a.Name);

        return await PaginatedList<Artist>.CreateAsync(query, request.PageNumber, request.PageSize, cancellationToken);
    }
}
