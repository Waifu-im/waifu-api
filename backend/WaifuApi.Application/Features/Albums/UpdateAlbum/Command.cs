using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.UpdateAlbum;

public record UpdateAlbumCommand(long UserId, long AlbumId, string Name, string Description) : ICommand<AlbumDto>;

public class UpdateAlbumCommandHandler : ICommandHandler<UpdateAlbumCommand, AlbumDto>
{
    private readonly IWaifuDbContext _context;

    public UpdateAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<AlbumDto> Handle(UpdateAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId && a.UserId == request.UserId, cancellationToken);

        if (album != null)
        {
            album.Name = request.Name;
            album.Description = request.Description;
            await _context.SaveChangesAsync(cancellationToken);
            
            return new AlbumDto
            {
                Id = album.Id,
                Name = album.Name,
                Description = album.Description,
                IsDefault = album.IsDefault,
                UserId = album.UserId
            };
        }

        return null!;
    }
}
