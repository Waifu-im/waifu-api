using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Images.ReviewImage;

public record ReviewImageCommand(long ImageId, bool Accepted) : ICommand;

public class ReviewImageCommandHandler : ICommandHandler<ReviewImageCommand>
{
    private readonly IWaifuDbContext _context;
    private readonly IStorageService _storageService;

    public ReviewImageCommandHandler(IWaifuDbContext context, IStorageService storageService)
    {
        _context = context;
        _storageService = storageService;
    }

    public async ValueTask<Unit> Handle(ReviewImageCommand request, CancellationToken cancellationToken)
    {
        var image = await _context.Images.FindAsync(new object[] { request.ImageId }, cancellationToken);
        if (image == null) return Unit.Value;

        if (request.Accepted)
        {
            image.ReviewStatus = ReviewStatus.Accepted;
        }
        else
        {
            // Delete from S3
            var s3FileName = $"{image.Id}{image.Extension}";
            await _storageService.DeleteAsync(s3FileName);

            // Delete from DB
            _context.Images.Remove(image);
        }

        await _context.SaveChangesAsync(cancellationToken);
        return Unit.Value;
    }
}
