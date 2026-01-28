using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Artists.ReviewArtist;

public record ReviewArtistCommand(long ArtistId, bool Accepted) : ICommand;

public class ReviewArtistCommandHandler : ICommandHandler<ReviewArtistCommand>
{
    private readonly IWaifuDbContext _context;

    public ReviewArtistCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(ReviewArtistCommand request, CancellationToken cancellationToken)
    {
        var artist = await _context.Artists.FindAsync(new object[] { request.ArtistId }, cancellationToken);
        if (artist == null) return Unit.Value;

        if (request.Accepted)
        {
            artist.ReviewStatus = ReviewStatus.Accepted;
        }
        else
        {
            // Delete on rejection
            _context.Artists.Remove(artist);
        }
        
        await _context.SaveChangesAsync(cancellationToken);
        return Unit.Value;
    }
}
