using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Images.BulkDeleteImages;

public record BulkDeleteImagesCommand(List<long> ImageIds) : ICommand;

public class BulkDeleteImagesCommandHandler : ICommandHandler<BulkDeleteImagesCommand>
{
    private readonly IWaifuDbContext _context;

    public BulkDeleteImagesCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(BulkDeleteImagesCommand request, CancellationToken cancellationToken)
    {
        var images = await _context.Images
            .Where(i => request.ImageIds.Contains(i.Id))
            .ToListAsync(cancellationToken);

        _context.Images.RemoveRange(images);
        await _context.SaveChangesAsync(cancellationToken);

        return Unit.Value;
    }
}
