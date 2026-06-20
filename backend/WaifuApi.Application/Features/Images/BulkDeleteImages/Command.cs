using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

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

        // Resolve pending reports on the deleted images (their image link is nulled by the FK on delete).
        var pendingReports = await _context.Reports
            .Where(r => r.ImageId != null && request.ImageIds.Contains(r.ImageId.Value) && r.Status == ReportStatus.Pending)
            .ToListAsync(cancellationToken);
        foreach (var report in pendingReports)
        {
            report.Status = ReportStatus.Resolved;
            report.ReviewerNote = "The reported image was deleted by a moderator.";
        }

        _context.Images.RemoveRange(images);
        await _context.SaveChangesAsync(cancellationToken);

        return Unit.Value;
    }
}
