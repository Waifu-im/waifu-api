using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Artists.CreateArtist;

public record CreateArtistCommand(
    string Name,
    string? Patreon,
    string? Pixiv,
    string? Twitter,
    string? DeviantArt
) : ICommand<Artist>;

public class CreateArtistCommandHandler : ICommandHandler<CreateArtistCommand, Artist>
{
    private readonly IWaifuDbContext _context;
    private readonly IConfiguration _configuration;

    public CreateArtistCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _configuration = configuration;
    }

    public async ValueTask<Artist> Handle(CreateArtistCommand request, CancellationToken cancellationToken)
    {
        var exists = await _context.Artists.AnyAsync(a => a.Name == request.Name, cancellationToken);
        if (exists)
        {
            throw new ConflictException($"Artist with name '{request.Name}' already exists.");
        }

        // Check unique constraints for social links if provided
        if (!string.IsNullOrEmpty(request.Patreon) && await _context.Artists.AnyAsync(a => a.Patreon == request.Patreon, cancellationToken))
            throw new ConflictException($"Artist with Patreon '{request.Patreon}' already exists.");
            
        if (!string.IsNullOrEmpty(request.Pixiv) && await _context.Artists.AnyAsync(a => a.Pixiv == request.Pixiv, cancellationToken))
            throw new ConflictException($"Artist with Pixiv '{request.Pixiv}' already exists.");
            
        if (!string.IsNullOrEmpty(request.Twitter) && await _context.Artists.AnyAsync(a => a.Twitter == request.Twitter, cancellationToken))
            throw new ConflictException($"Artist with Twitter '{request.Twitter}' already exists.");
            
        if (!string.IsNullOrEmpty(request.DeviantArt) && await _context.Artists.AnyAsync(a => a.DeviantArt == request.DeviantArt, cancellationToken))
            throw new ConflictException($"Artist with DeviantArt '{request.DeviantArt}' already exists.");

        var requireReview = bool.Parse(_configuration["Moderation:RequireArtistReview"] ?? "true");
        
        var artist = new Artist
        {
            Name = request.Name,
            Patreon = request.Patreon,
            Pixiv = request.Pixiv,
            Twitter = request.Twitter,
            DeviantArt = request.DeviantArt,
            ReviewStatus = requireReview ? ReviewStatus.Pending : ReviewStatus.Accepted
        };

        _context.Artists.Add(artist);
        await _context.SaveChangesAsync(cancellationToken);

        return artist;
    }
}
