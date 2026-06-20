using System;
using System.Linq;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Services;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Artists.CreateArtist;

public record CreateArtistCommand(
    string Name,
    string? Patreon,
    string? Pixiv,
    string? Twitter,
    string? DeviantArt,
    long? CreatorId = null,
    Role? CreatorRole = null
) : ICommand<Artist>;

public class CreateArtistCommandHandler : ICommandHandler<CreateArtistCommand, Artist>
{
    private readonly IWaifuDbContext _context;
    private readonly IReviewPolicy _reviewPolicy;

    public CreateArtistCommandHandler(IWaifuDbContext context, IReviewPolicy reviewPolicy)
    {
        _context = context;
        _reviewPolicy = reviewPolicy;
    }

    public async ValueTask<Artist> Handle(CreateArtistCommand request, CancellationToken cancellationToken)
    {
        var trimmedName = request.Name.Trim();
        
        // Nettoyage des inputs
        var patreon = request.Patreon.ToNullIfEmpty()?.Trim();
        var pixiv = request.Pixiv.ToNullIfEmpty()?.Trim();
        var twitter = request.Twitter.ToNullIfEmpty()?.Trim();
        var deviantArt = request.DeviantArt.ToNullIfEmpty()?.Trim();

        // OPTIMISATION : Une seule requête pour tout vérifier
        // On cherche s'il existe DEJA un artiste qui a SOIT le même nom, SOIT le même lien X, SOIT le même lien Y...
        var existingConflict = await _context.Artists
            .AsNoTracking()
            .Where(a =>
                a.Name.ToLower() == trimmedName.ToLower() ||
                (patreon != null && a.Patreon == patreon) ||
                (pixiv != null && a.Pixiv == pixiv) ||
                (twitter != null && a.Twitter == twitter) ||
                (deviantArt != null && a.DeviantArt == deviantArt)
            )
            .Select(a => new { a.Name, a.Patreon, a.Pixiv, a.Twitter, a.DeviantArt }) // On ne récupère que les champs nécessaires
            .FirstOrDefaultAsync(cancellationToken);

        // Si on a trouvé un conflit, on identifie lequel pour l'erreur précise
        if (existingConflict != null)
        {
            if (string.Equals(existingConflict.Name, trimmedName, StringComparison.CurrentCultureIgnoreCase))
                throw new ConflictException($"Artist with name '{trimmedName}' already exists.");
            
            if (patreon != null && existingConflict.Patreon == patreon)
                throw new ConflictException($"Artist with Patreon '{patreon}' already exists.");
                
            if (pixiv != null && existingConflict.Pixiv == pixiv)
                throw new ConflictException($"Artist with Pixiv '{pixiv}' already exists.");
                
            if (twitter != null && existingConflict.Twitter == twitter)
                throw new ConflictException($"Artist with Twitter '{twitter}' already exists.");
                
            if (deviantArt != null && existingConflict.DeviantArt == deviantArt)
                throw new ConflictException($"Artist with DeviantArt '{deviantArt}' already exists.");
        }

        var requireReview = _reviewPolicy.RequiresReviewForNewContent(ReviewableContentType.Artist, request.CreatorRole);

        var artist = new Artist
        {
            Name = trimmedName,
            Patreon = patreon,
            Pixiv = pixiv,
            Twitter = twitter,
            DeviantArt = deviantArt,
            ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted,
            CreatorId = request.CreatorId
        };

        await using var transaction = await _context.BeginTransactionAsync(cancellationToken);
        _context.Artists.Add(artist);
        await _context.SaveChangesAsync(cancellationToken); // assigns artist.Id
        _context.ReviewTasks.Add(
            ReviewTaskFactory.NewContent(ReviewableContentType.Artist, artist.Id, request.CreatorId, requireReview));
        await _context.SaveChangesAsync(cancellationToken);
        await transaction.CommitAsync(cancellationToken);

        return artist;
    }
}