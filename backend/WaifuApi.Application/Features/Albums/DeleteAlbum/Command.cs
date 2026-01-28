using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Albums.DeleteAlbum;

public record DeleteAlbumCommand(long UserId, long AlbumId) : ICommand;

public class DeleteAlbumCommandHandler : ICommandHandler<DeleteAlbumCommand>
{
    private readonly IWaifuDbContext _context;

    public DeleteAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(DeleteAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId && a.UserId == request.UserId, cancellationToken);

        if (album != null)
        {
            if (album.IsDefault)
            {
                throw new InvalidOperationException("Cannot delete the default album.");
            }

            _context.Albums.Remove(album);
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
