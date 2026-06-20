using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.SubmitEdit;

/// <summary>Submits a proposed change to an existing image/tag/artist — creates an Edit review task.</summary>
public record SubmitEditCommand(
    long SubmitterId,
    Role? SubmitterRole,
    ReviewableContentType TargetType,
    long TargetId,
    string? Reason,
    string PayloadJson
) : ICommand<ReviewTaskDto>;

public class SubmitEditCommandHandler : ICommandHandler<SubmitEditCommand, ReviewTaskDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IReviewPolicy _policy;
    private readonly IEditApplier _applier;
    private readonly IEditPreparer _preparer;
    private readonly int _maxOpenPerUser;

    public SubmitEditCommandHandler(
        IWaifuDbContext context,
        IReviewPolicy policy,
        IEditApplier applier,
        IEditPreparer preparer,
        IConfiguration configuration)
    {
        _context = context;
        _policy = policy;
        _applier = applier;
        _preparer = preparer;
        _maxOpenPerUser = int.TryParse(configuration[ConfigurationKeys.Review.MaxOpenEditsPerUser], out var max) ? max : 0;
    }

    public async ValueTask<ReviewTaskDto> Handle(SubmitEditCommand request, CancellationToken cancellationToken)
    {
        // One open edit per (user, target): keeps a single user from spamming the same item.
        var hasOpenForTarget = await _context.ReviewTasks.AnyAsync(t =>
            t.Kind == ReviewTaskKind.Edit &&
            t.SubmitterId == request.SubmitterId &&
            t.TargetType == request.TargetType &&
            t.TargetId == request.TargetId &&
            t.Status == ReviewTaskStatus.Pending, cancellationToken);

        if (hasOpenForTarget)
        {
            throw new ConflictException("You already have a pending edit for this item. Edit or cancel it first.");
        }

        // Optional global cap on simultaneously-open edits per user.
        if (_maxOpenPerUser > 0)
        {
            var openCount = await _context.ReviewTasks.CountAsync(t =>
                t.Kind == ReviewTaskKind.Edit &&
                t.SubmitterId == request.SubmitterId &&
                t.Status == ReviewTaskStatus.Pending, cancellationToken);
            if (openCount >= _maxOpenPerUser)
            {
                throw new ConflictException($"You have too many pending edits (limit {_maxOpenPerUser}). Please wait for some to be reviewed.");
            }
        }

        var prepared = await _preparer.PrepareAsync(request.TargetType, request.TargetId, request.PayloadJson, cancellationToken);

        var task = new ReviewTask
        {
            Kind = ReviewTaskKind.Edit,
            TargetType = request.TargetType,
            TargetId = request.TargetId,
            SubmitterId = request.SubmitterId,
            Payload = prepared.NormalizedPayload,
            Reason = request.Reason.ToNullIfEmpty()?.Trim(),
            Status = ReviewTaskStatus.Pending,
            CreatedAt = DateTime.UtcNow
        };

        _context.ReviewTasks.Add(task);

        // Trusted users (per the shared review policy) have their change applied immediately.
        var requiresReview = _policy.RequiresReviewForEditRequest(request.TargetType, request.SubmitterRole);
        if (!requiresReview)
        {
            await using var transaction = await _context.BeginTransactionAsync(cancellationToken);
            task.Status = ReviewTaskStatus.Approved;
            task.ResolvedAt = DateTime.UtcNow;
            task.ReviewerNote = "Auto-applied (trusted role).";
            await _applier.ApplyAsync(task, cancellationToken);
            await _context.SaveChangesAsync(cancellationToken);
            await transaction.CommitAsync(cancellationToken);
        }
        else
        {
            await _context.SaveChangesAsync(cancellationToken);
        }

        return ReviewTaskMapping.ToDto(task, prepared.Target);
    }
}
