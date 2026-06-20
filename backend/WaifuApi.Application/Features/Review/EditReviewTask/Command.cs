using System;
using System.Collections.Generic;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Extensions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Features.Artists.UpdateArtist;
using WaifuApi.Application.Features.Images.UpdateImage;
using WaifuApi.Application.Features.Tags.UpdateTag;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.EditReviewTask;

/// <summary>
/// Edits the content of a Pending review task — the live entity for NewContent, or the proposed payload for
/// Edit. Allowed for the submitter (their own) or any moderator; content-only (never changes review status).
/// </summary>
public record EditReviewTaskCommand(long TaskId, long ActorId, bool ActorIsModerator, string ContentJson) : ICommand<ReviewTaskDto>;

public class EditReviewTaskCommandHandler : ICommandHandler<EditReviewTaskCommand, ReviewTaskDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IEditPreparer _preparer;
    private readonly IMediator _mediator;
    private readonly string _cdnBaseUrl;

    public EditReviewTaskCommandHandler(IWaifuDbContext context, IEditPreparer preparer, IMediator mediator, IConfiguration configuration)
    {
        _context = context;
        _preparer = preparer;
        _mediator = mediator;
        _cdnBaseUrl = configuration[ConfigurationKeys.Cdn.BaseUrl] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ReviewTaskDto> Handle(EditReviewTaskCommand request, CancellationToken cancellationToken)
    {
        var task = await _context.ReviewTasks.FindAsync(new object[] { request.TaskId }, cancellationToken)
            ?? throw new KeyNotFoundException($"Review task with ID {request.TaskId} not found.");

        // Only the submitter or a moderator may edit, and only while still pending.
        if (task.SubmitterId != request.ActorId && !request.ActorIsModerator)
        {
            throw new ForbiddenException("You can only edit your own submissions.");
        }
        if (task.Status != ReviewTaskStatus.Pending)
        {
            throw new ConflictException("This review task has already been resolved.");
        }

        await using var transaction = await _context.BeginTransactionAsync(cancellationToken);

        ReviewTaskTargetDto target;
        if (task.Kind == ReviewTaskKind.Edit)
        {
            // Re-validate/normalize the proposed change against the current target.
            var prepared = await _preparer.PrepareAsync(task.TargetType, task.TargetId, request.ContentJson, cancellationToken);
            task.Payload = prepared.NormalizedPayload;
            target = prepared.Target;
        }
        else
        {
            // Apply the full content to the still-pending entity, then re-read it.
            target = await ApplyNewContentAsync(task.TargetType, task.TargetId, request.ContentJson, cancellationToken);
        }

        // Flag a moderator editing someone else's submission so the submitter sees a cue.
        if (request.ActorIsModerator && task.SubmitterId != request.ActorId)
        {
            task.ModeratorEditedAt = DateTime.UtcNow;
        }

        await _context.SaveChangesAsync(cancellationToken);
        await transaction.CommitAsync(cancellationToken);

        return ReviewTaskMapping.ToDto(task, target);
    }

    private async Task<ReviewTaskTargetDto> ApplyNewContentAsync(ReviewableContentType type, long targetId, string contentJson, CancellationToken ct)
    {
        switch (type)
        {
            case ReviewableContentType.Image:
            {
                var content = Deserialize<NewImageContent>(contentJson);
                var image = await _context.Images.Include(i => i.Tags).Include(i => i.Artists)
                    .FirstOrDefaultAsync(i => i.Id == targetId, ct)
                    ?? throw new KeyNotFoundException($"Image with ID {targetId} not found.");
                // Content-only: preserve uploader + review status (stays Pending), no file replacement.
                await _mediator.Send(new UpdateImageCommand(
                    image.Id, content.Source, content.IsNsfw ?? image.IsNsfw, image.UploaderId,
                    content.Tags, content.Artists, null), ct);
                var updated = await _context.Images.AsNoTracking().Include(i => i.Tags).Include(i => i.Artists)
                    .FirstAsync(i => i.Id == targetId, ct);
                return new ReviewTaskTargetDto { Image = updated.ToDto(_cdnBaseUrl, includeUploaderId: true) };
            }
            case ReviewableContentType.Tag:
            {
                var content = Deserialize<NewTagContent>(contentJson);
                var tag = await _context.Tags.FindAsync(new object[] { targetId }, ct)
                    ?? throw new KeyNotFoundException($"Tag with ID {targetId} not found.");
                await _mediator.Send(new UpdateTagCommand(
                    tag.Id, content.Name ?? tag.Name, content.Description ?? tag.Description, null, null, null), ct);
                var updated = await _context.Tags.AsNoTracking().FirstAsync(t => t.Id == targetId, ct);
                return new ReviewTaskTargetDto { Tag = updated.ToDto() };
            }
            case ReviewableContentType.Artist:
            {
                var content = Deserialize<NewArtistContent>(contentJson);
                var artist = await _context.Artists.FindAsync(new object[] { targetId }, ct)
                    ?? throw new KeyNotFoundException($"Artist with ID {targetId} not found.");
                await _mediator.Send(new UpdateArtistCommand(
                    artist.Id, content.Name ?? artist.Name, content.Patreon ?? artist.Patreon, content.Pixiv ?? artist.Pixiv,
                    content.Twitter ?? artist.Twitter, content.DeviantArt ?? artist.DeviantArt, null, null), ct);
                var updated = await _context.Artists.AsNoTracking().FirstAsync(a => a.Id == targetId, ct);
                return new ReviewTaskTargetDto { Artist = updated.ToDto() };
            }
            default:
                throw new ArgumentOutOfRangeException(nameof(type));
        }
    }

    private static T Deserialize<T>(string json) where T : new()
    {
        try { return JsonSerializer.Deserialize<T>(json, EditPayloadOptions.Strict) ?? new T(); }
        catch (JsonException ex) { throw new ArgumentException($"Invalid content: {ex.Message}"); }
    }
}
