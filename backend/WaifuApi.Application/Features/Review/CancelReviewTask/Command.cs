using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.CancelReviewTask;

public record CancelReviewTaskCommand(long TaskId, long ActorId, bool ActorIsModerator) : ICommand;

public class CancelReviewTaskCommandHandler : ICommandHandler<CancelReviewTaskCommand>
{
    private readonly IWaifuDbContext _context;
    private readonly IReviewTargetService _targetService;

    public CancelReviewTaskCommandHandler(IWaifuDbContext context, IReviewTargetService targetService)
    {
        _context = context;
        _targetService = targetService;
    }

    public async ValueTask<Unit> Handle(CancelReviewTaskCommand request, CancellationToken cancellationToken)
    {
        var task = await _context.ReviewTasks.FindAsync(new object[] { request.TaskId }, cancellationToken)
            ?? throw new KeyNotFoundException($"Review task with ID {request.TaskId} not found.");

        // Only the original submitter (or a moderator) may cancel, and only while it is still pending.
        if (task.SubmitterId != request.ActorId && !request.ActorIsModerator)
        {
            throw new ForbiddenException("You can only cancel your own submissions.");
        }

        if (task.Status != ReviewTaskStatus.Pending)
        {
            throw new ConflictException("This review task has already been resolved.");
        }

        await using var transaction = await _context.BeginTransactionAsync(cancellationToken);

        // Cancelling a new submission withdraws it — the not-yet-public entity (and its file) is removed.
        if (task.Kind == ReviewTaskKind.NewContent)
        {
            try
            {
                await _targetService.DeleteAsync(task.TargetType, task.TargetId, request.ActorId, request.ActorIsModerator, cancellationToken);
            }
            catch (KeyNotFoundException)
            {
                // Target already gone — just close the task.
            }
        }

        task.Status = ReviewTaskStatus.Cancelled;
        task.ResolvedAt = DateTime.UtcNow;
        if (request.ActorIsModerator && task.SubmitterId != request.ActorId)
        {
            task.ReviewerId = request.ActorId;
        }

        await _context.SaveChangesAsync(cancellationToken);
        await transaction.CommitAsync(cancellationToken);

        return Unit.Value;
    }
}
