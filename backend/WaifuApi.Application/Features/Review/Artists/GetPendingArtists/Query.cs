using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Artists.GetPendingArtists;

public record GetPendingArtistsQuery : IQuery<List<Artist>>;

public class GetPendingArtistsQueryHandler : IQueryHandler<GetPendingArtistsQuery, List<Artist>>
{
    private readonly IWaifuDbContext _context;

    public GetPendingArtistsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<Artist>> Handle(GetPendingArtistsQuery request, CancellationToken cancellationToken)
    {
        return await _context.Artists
            .Where(a => a.ReviewStatus == ReviewStatus.Pending)
            .ToListAsync(cancellationToken);
    }
}
