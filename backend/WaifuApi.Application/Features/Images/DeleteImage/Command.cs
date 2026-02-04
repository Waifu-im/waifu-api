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

    public DeleteImageCommandHandler(IWaifuDbContext context, IStorageService storageService)
    {
        _context = context;
        _storageService = storageService;
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

        // Delete from S3
        var s3FileName = $"{image.Id}{image.Extension}";
        await _storageService.DeleteAsync(s3FileName);

        // Delete from DB
        _context.Images.Remove(image);
        await _context.SaveChangesAsync(cancellationToken);
        return Unit.Value;
    }
}
