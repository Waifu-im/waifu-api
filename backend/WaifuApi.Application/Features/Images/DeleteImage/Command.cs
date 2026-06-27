using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Images.DeleteImage;

public record DeleteImageCommand(long ImageId, long RequesterId, bool IsAdminOrModerator) : ICommand;

public class DeleteImageCommandHandler : ICommandHandler<DeleteImageCommand>
{
    private readonly IWaifuDbContext _context;
    private readonly IStorageService _storageService;
    private readonly ICdnCacheService _cdnCache;

    public DeleteImageCommandHandler(IWaifuDbContext context, IStorageService storageService, ICdnCacheService cdnCache)
    {
        _context = context;
        _storageService = storageService;
        _cdnCache = cdnCache;
    }

    public async ValueTask<Unit> Handle(DeleteImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images.FindAsync(new object[] { request.ImageId }, cancellationToken);
        
        if (image == null)
        {
            throw new KeyNotFoundException($"Image with ID {request.ImageId} not found.");
        }

        // Check permissions
        if (image.UploaderId != request.RequesterId && !request.IsAdminOrModerator)
        {
            throw new UnauthorizedAccessException("You are not authorized to delete this image.");
        }

        // Resolve any pending reports on this image. The image link is set to null by the FK on delete, so the
        // report survives as a resolved record for the reporter rather than being cascade-deleted.
        var pendingReports = await _context.Reports
            .Where(r => r.ImageId == request.ImageId && r.Status == ReportStatus.Pending)
            .ToListAsync(cancellationToken);
        foreach (var report in pendingReports)
        {
            report.Status = ReportStatus.Resolved;
            report.ReviewerNote = "The reported image was deleted by a moderator.";
        }

        // Delete from S3
        var s3FileName = $"{image.Id}{image.Extension}";
        await _storageService.DeleteAsync(s3FileName);

        // Delete from DB
        _context.Images.Remove(image);
        await _context.SaveChangesAsync(cancellationToken);

        // Clear the CDN cache for just this image so the edge doesn't keep serving the deleted file.
        await _cdnCache.PurgeImageAsync(image.Id, new[] { image.Extension }, cancellationToken);
        return Unit.Value;
    }
}
