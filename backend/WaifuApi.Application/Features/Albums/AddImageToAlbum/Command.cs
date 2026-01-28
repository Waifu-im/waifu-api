using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Albums.AddImageToAlbum;

public record AddImageToAlbumCommand(long UserId, long AlbumId, long ImageId) : ICommand;

public class AddImageToAlbumCommandHandler : ICommandHandler<AddImageToAlbumCommand>
{
    private readonly IWaifuDbContext _context;

    public AddImageToAlbumCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(AddImageToAlbumCommand request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId && a.UserId == request.UserId, cancellationToken);

        if (album == null) return Unit.Value;

        if (!await _context.AlbumItems.AnyAsync(ai => ai.AlbumId == request.AlbumId && ai.ImageId == request.ImageId, cancellationToken))
        {
            _context.AlbumItems.Add(new AlbumItem
            {
                AlbumId = request.AlbumId,
                ImageId = request.ImageId,
                AddedAt = DateTime.UtcNow
            });
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
