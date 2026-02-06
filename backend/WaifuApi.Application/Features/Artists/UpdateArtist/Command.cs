using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Artists.UpdateArtist;

public record UpdateArtistCommand(
    long Id, 
    string Name,
    string? Patreon,
    string? Pixiv,
    string? Twitter,
    string? DeviantArt,
    ReviewStatus? ReviewStatus
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

        var patreon = request.Patreon.ToNullIfEmpty();
        var pixiv = request.Pixiv.ToNullIfEmpty();
        var twitter = request.Twitter.ToNullIfEmpty();
        var deviantArt = request.DeviantArt.ToNullIfEmpty();

        var conflict = await _context.Artists
            .AsNoTracking()
            .Where(a => a.Id != request.Id && ( // Important : ignorer l'artiste qu'on modifie
                a.Name.ToLower() == request.Name.ToLower() ||
                (patreon != null && a.Patreon == patreon) ||
                (pixiv != null && a.Pixiv == pixiv) ||
                (twitter != null && a.Twitter == twitter) ||
                (deviantArt != null && a.DeviantArt == deviantArt)
            ))
            .Select(a => new { a.Name, a.Patreon, a.Pixiv, a.Twitter, a.DeviantArt })
            .FirstOrDefaultAsync(cancellationToken);

        if (conflict != null)
        {
            if (string.Equals(conflict.Name, request.Name, StringComparison.CurrentCultureIgnoreCase))
                throw new ConflictException($"Artist with name '{request.Name}' already exists.");
            
            if (patreon != null && conflict.Patreon == patreon)
                throw new ConflictException("Patreon link is already used by another artist.");
                
            if (pixiv != null && conflict.Pixiv == pixiv)
                throw new ConflictException("Pixiv link is already used by another artist.");
                
            if (twitter != null && conflict.Twitter == twitter)
                throw new ConflictException("Twitter link is already used by another artist.");
                
            if (deviantArt != null && conflict.DeviantArt == deviantArt)
                throw new ConflictException("DeviantArt link is already used by another artist.");
        }

        artist.Name = request.Name;
        artist.Patreon = patreon;
        artist.Pixiv = pixiv;
        artist.Twitter = twitter;
        artist.DeviantArt = deviantArt;

        if (request.ReviewStatus.HasValue)
        {
            artist.ReviewStatus = request.ReviewStatus.Value;
        }
        
        await _context.SaveChangesAsync(cancellationToken);

        return artist;
    }
}