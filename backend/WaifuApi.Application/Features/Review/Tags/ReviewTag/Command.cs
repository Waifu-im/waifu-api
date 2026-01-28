using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.Tags.ReviewTag;

public record ReviewTagCommand(long TagId, bool Accepted) : ICommand;

public class ReviewTagCommandHandler : ICommandHandler<ReviewTagCommand>
{
    private readonly IWaifuDbContext _context;

    public ReviewTagCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(ReviewTagCommand request, CancellationToken cancellationToken)
    {
        var tag = await _context.Tags.FindAsync(new object[] { request.TagId }, cancellationToken);
        if (tag == null) return Unit.Value;

        if (request.Accepted)
        {
            tag.ReviewStatus = ReviewStatus.Accepted;
        }
        else
        {
            // Delete on rejection
            _context.Tags.Remove(tag);
        }
        
        await _context.SaveChangesAsync(cancellationToken);
        return Unit.Value;
    }
}
