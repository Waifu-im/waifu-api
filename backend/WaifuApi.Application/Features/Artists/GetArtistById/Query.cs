using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Artists.GetArtistById;

public record GetArtistByIdQuery(long ArtistId) : IQuery<ArtistDto>;

public class GetArtistByIdQueryHandler : IQueryHandler<GetArtistByIdQuery, ArtistDto>
{
    private readonly IWaifuDbContext _context;

    public GetArtistByIdQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<ArtistDto> Handle(GetArtistByIdQuery request, CancellationToken cancellationToken)
    {
        var artist = await _context.Artists.FindAsync(new object[] { request.ArtistId }, cancellationToken);
        if (artist == null)
        {
            throw new KeyNotFoundException($"Artist with ID {request.ArtistId} not found.");
        }
        
        return new ArtistDto
        {
            Id = artist.Id,
            Name = artist.Name,
            Patreon = artist.Patreon,
            Pixiv = artist.Pixiv,
            Twitter = artist.Twitter,
            DeviantArt = artist.DeviantArt,
            ReviewStatus = artist.ReviewStatus
        };
    }
}
