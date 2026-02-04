using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Albums.RemoveImageFromAlbum;

public record RemoveImageFromAlbumCommand(long UserId, long AlbumId, long ImageId) : ICommand;

public class RemoveImageFromAlbumCommandHandler : ICommandHandler<RemoveImageFromAlbumCommand>
{
    private readonly IWaifuDbContext _context;

    public RemoveImageFromAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(RemoveImageFromAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId && a.UserId == request.UserId, cancellationToken);

        if (album == null) return Unit.Value;

        var item = await _context.AlbumItems
            .FirstOrDefaultAsync(ai => ai.AlbumId == request.AlbumId && ai.ImageId == request.ImageId, cancellationToken);

        if (item != null)
        {
            _context.AlbumItems.Remove(item);
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
