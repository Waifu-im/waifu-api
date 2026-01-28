using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.CreateAlbum;

public record CreateAlbumCommand(long UserId, string Name, string Description) : ICommand<AlbumDto>;

public class CreateAlbumCommandHandler : ICommandHandler<CreateAlbumCommand, AlbumDto>
{
    private readonly IWaifuDbContext _context;

    public CreateAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<AlbumDto> Handle(CreateAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = new Album
        {
            UserId = request.UserId,
            Name = request.Name,
            Description = request.Description,
            IsDefault = false
        };

        _context.Albums.Add(album);
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
}
