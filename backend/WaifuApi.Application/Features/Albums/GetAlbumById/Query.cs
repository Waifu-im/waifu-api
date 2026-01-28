using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
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
        var album = await _context.Albums.FindAsync(new object[] { request.AlbumId }, cancellationToken);

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
            UserId = album.UserId
        };
    }
}
