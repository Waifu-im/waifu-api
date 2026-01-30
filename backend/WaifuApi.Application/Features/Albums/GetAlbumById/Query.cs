using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Albums.GetAlbumById;

public record GetAlbumByIdQuery(long AlbumId) : IQuery<AlbumDto>;

public class GetAlbumByIdQueryHandler : IQueryHandler<GetAlbumByIdQuery, AlbumDto>
{
    private readonly IWaifuDbContext _context;

    public GetAlbumByIdQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<AlbumDto> Handle(GetAlbumByIdQuery request, CancellationToken cancellationToken)
    {
        var album = await _context.Albums
            .Include(a => a.Items) // Changed from AlbumItems to Items
            .FirstOrDefaultAsync(a => a.Id == request.AlbumId, cancellationToken);

        if (album == null)
        {
            throw new KeyNotFoundException($"Album with ID {request.AlbumId} not found.");
        }

        return new AlbumDto
        {
            Id = album.Id,
            Name = album.Name,
            Description = album.Description,
            IsDefault = album.IsDefault,
            UserId = album.UserId,
            ImageCount = album.Items.Count // Changed from AlbumItems to Items
        };
    }
}
