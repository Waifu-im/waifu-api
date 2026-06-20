using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Features.Artists.DeleteArtist;
using WaifuApi.Application.Features.Images.DeleteImage;
using WaifuApi.Application.Features.Tags.DeleteTag;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Common.Services;

/// <inheritdoc />
public class ReviewTargetService : IReviewTargetService
{
    private readonly IWaifuDbContext _context;
    private readonly IMediator _mediator;

    public ReviewTargetService(IWaifuDbContext context, IMediator mediator)
    {
        _context = context;
        _mediator = mediator;
    }

    public async Task AcceptAsync(ReviewableContentType type, long targetId, CancellationToken cancellationToken)
    {
        switch (type)
        {
            case ReviewableContentType.Image:
            {
                var image = await _context.Images.FindAsync(new object[] { targetId }, cancellationToken)
                            ?? throw new KeyNotFoundException($"Image with ID {targetId} not found.");
                image.ReviewStatus = ReviewStatus.Accepted;
                break;
            }
            case ReviewableContentType.Tag:
            {
                var tag = await _context.Tags.FindAsync(new object[] { targetId }, cancellationToken)
                          ?? throw new KeyNotFoundException($"Tag with ID {targetId} not found.");
                tag.ReviewStatus = ReviewStatus.Accepted;
                break;
            }
            case ReviewableContentType.Artist:
            {
                var artist = await _context.Artists.FindAsync(new object[] { targetId }, cancellationToken)
                             ?? throw new KeyNotFoundException($"Artist with ID {targetId} not found.");
                artist.ReviewStatus = ReviewStatus.Accepted;
                break;
            }
            default:
                throw new ArgumentOutOfRangeException(nameof(type));
        }
    }

    public async Task DeleteAsync(ReviewableContentType type, long targetId, long actorId, bool actorIsModerator, CancellationToken cancellationToken)
    {
        switch (type)
        {
            case ReviewableContentType.Image:
                await _mediator.Send(new DeleteImageCommand(targetId, actorId, actorIsModerator), cancellationToken);
                break;
            case ReviewableContentType.Tag:
                await _mediator.Send(new DeleteTagCommand(targetId), cancellationToken);
                break;
            case ReviewableContentType.Artist:
                await _mediator.Send(new DeleteArtistCommand(targetId), cancellationToken);
                break;
            default:
                throw new ArgumentOutOfRangeException(nameof(type));
        }
    }
}
