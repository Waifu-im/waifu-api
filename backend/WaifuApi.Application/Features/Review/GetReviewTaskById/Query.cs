using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.GetReviewTaskById;

/// <param name="RestrictToSubmitterId">When set, the task is only returned if it belongs to this user (scopes non-moderators to their own).</param>
public record GetReviewTaskByIdQuery(long Id, long? RestrictToSubmitterId) : IQuery<ReviewTaskDto>;

public class GetReviewTaskByIdQueryHandler : IQueryHandler<GetReviewTaskByIdQuery, ReviewTaskDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public GetReviewTaskByIdQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ReviewTaskDto> Handle(GetReviewTaskByIdQuery request, CancellationToken cancellationToken)
    {
        var task = await _context.ReviewTasks
            .AsNoTracking()
            .Include(t => t.Submitter)
            .Include(t => t.Reviewer)
            .FirstOrDefaultAsync(t => t.Id == request.Id, cancellationToken);

        // Treat "not yours" the same as "not found" so a user can't probe others' tasks.
        if (task == null ||
            (request.RestrictToSubmitterId.HasValue && task.SubmitterId != request.RestrictToSubmitterId.Value))
        {
            throw new KeyNotFoundException($"Review task with ID {request.Id} not found.");
        }

        ReviewTaskTargetDto? target = task.TargetType switch
        {
            ReviewableContentType.Image => await BuildImageTargetAsync(task.TargetId, cancellationToken),
            ReviewableContentType.Tag => await BuildTagTargetAsync(task.TargetId, cancellationToken),
            ReviewableContentType.Artist => await BuildArtistTargetAsync(task.TargetId, cancellationToken),
            _ => null
        };

        return ReviewTaskMapping.ToDto(task, target);
    }

    private async Task<ReviewTaskTargetDto?> BuildImageTargetAsync(long id, CancellationToken ct)
    {
        var image = await _context.Images.AsNoTracking()
            .Include(i => i.Tags)
            .Include(i => i.Artists)
            .FirstOrDefaultAsync(i => i.Id == id, ct);
        return image == null ? null : new ReviewTaskTargetDto { Image = image.ToDto(_cdnBaseUrl, includeUploaderId: true) };
    }

    private async Task<ReviewTaskTargetDto?> BuildTagTargetAsync(long id, CancellationToken ct)
    {
        var tag = await _context.Tags.AsNoTracking().FirstOrDefaultAsync(t => t.Id == id, ct);
        return tag == null ? null : new ReviewTaskTargetDto { Tag = tag.ToDto() };
    }

    private async Task<ReviewTaskTargetDto?> BuildArtistTargetAsync(long id, CancellationToken ct)
    {
        var artist = await _context.Artists.AsNoTracking().FirstOrDefaultAsync(a => a.Id == id, ct);
        return artist == null ? null : new ReviewTaskTargetDto { Artist = artist.ToDto() };
    }
}
