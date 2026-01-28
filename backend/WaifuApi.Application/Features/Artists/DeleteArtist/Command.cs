using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Artists.DeleteArtist;

public record DeleteArtistCommand(long Id) : ICommand<Unit>;

public class DeleteArtistCommandHandler : ICommandHandler<DeleteArtistCommand, Unit>
{
    private readonly IWaifuDbContext _context;

    public DeleteArtistCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(DeleteArtistCommand request, CancellationToken cancellationToken)
    {
        var artist = await _context.Artists.FindAsync(new object[] { request.Id }, cancellationToken);

        if (artist == null)
        {
            throw new KeyNotFoundException($"Artist with ID {request.Id} not found.");
        }

        _context.Artists.Remove(artist);
        await _context.SaveChangesAsync(cancellationToken);

        return Unit.Value;
    }
}
