using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.CreateAlbum;

public record CreateAlbumCommand(long UserId, string Name, string Description) : ICommand<Album>;

public class CreateAlbumCommandHandler : ICommandHandler<CreateAlbumCommand, Album>
{
    private readonly IWaifuDbContext _context;

    public CreateAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Album> Handle(CreateAlbumCommand request, CancellationToken cancellationToken)
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

        return album;
    }
}
