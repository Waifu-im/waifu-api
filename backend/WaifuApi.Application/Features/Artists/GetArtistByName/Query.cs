using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Artists.GetArtistByName;

public record GetArtistByNameQuery(string Name) : IQuery<Artist>;

public class GetArtistByNameQueryHandler : IQueryHandler<GetArtistByNameQuery, Artist>
{
    private readonly IWaifuDbContext _context;

    public GetArtistByNameQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Artist> Handle(GetArtistByNameQuery request, CancellationToken cancellationToken)
    {
        var artist = await _context.Artists
            .AsNoTracking()
            .FirstOrDefaultAsync(a => a.Name == request.Name, cancellationToken);

        if (artist == null)
        {
            throw new KeyNotFoundException($"Artist with name '{request.Name}' not found.");
        }

        return artist;
    }
}
