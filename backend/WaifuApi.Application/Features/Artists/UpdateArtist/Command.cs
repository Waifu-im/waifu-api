using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Artists.UpdateArtist;

public record UpdateArtistCommand(
    long Id, 
    string Name,
    string? Patreon,
    string? Pixiv,
    string? Twitter,
    string? DeviantArt
) : ICommand<Artist>;

public class UpdateArtistCommandHandler : ICommandHandler<UpdateArtistCommand, Artist>
{
    private readonly IWaifuDbContext _context;

    public UpdateArtistCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Artist> Handle(UpdateArtistCommand request, CancellationToken cancellationToken)
    {
        var artist = await _context.Artists.FindAsync(new object[] { request.Id }, cancellationToken);

        if (artist == null)
        {
            throw new KeyNotFoundException($"Artist with ID {request.Id} not found.");
        }

        artist.Name = request.Name;
        artist.Patreon = request.Patreon;
        artist.Pixiv = request.Pixiv;
        artist.Twitter = request.Twitter;
        artist.DeviantArt = request.DeviantArt;
        
        await _context.SaveChangesAsync(cancellationToken);

        return artist;
    }
}
