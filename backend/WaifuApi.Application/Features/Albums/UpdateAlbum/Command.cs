using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.UpdateAlbum;

public record UpdateAlbumCommand(long UserId, long AlbumId, string Name, string Description) : ICommand<Album>;

public class UpdateAlbumCommandHandler : ICommandHandler<UpdateAlbumCommand, Album>
{
    private readonly IWaifuDbContext _context;

    public UpdateAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Album> Handle(UpdateAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId && a.UserId == request.UserId, cancellationToken);

        if (album != null)
        {
            album.Name = request.Name;
            album.Description = request.Description;
            await _context.SaveChangesAsync(cancellationToken);
        }

        return album!;
    }
}
