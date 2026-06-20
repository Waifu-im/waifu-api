using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.RejectReviewTask;

public record RejectReviewTaskCommand(long TaskId, long ReviewerId, string? Reason) : ICommand;

public class RejectReviewTaskCommandHandler : ICommandHandler<RejectReviewTaskCommand>
{
    private readonly IWaifuDbContext _context;
    private readonly IReviewTargetService _targetService;

    public RejectReviewTaskCommandHandler(IWaifuDbContext context, IReviewTargetService targetService)
    {
        _context = context;
        _targetService = targetService;
    }

    public async ValueTask<Unit> Handle(RejectReviewTaskCommand request, CancellationToken cancellationToken)
    {
        var task = await _context.ReviewTasks.FindAsync(new object[] { request.TaskId }, cancellationToken)
            ?? throw new KeyNotFoundException($"Review task with ID {request.TaskId} not found.");

        if (task.Status != ReviewTaskStatus.Pending)
        {
            throw new ConflictException("This review task has already been resolved.");
        }

        await using var transaction = await _context.BeginTransactionAsync(cancellationToken);

        // Rejecting a new submission removes the not-yet-public entity (and its file). The task itself
        // survives as the durable record that the submission was made and rejected.
        if (task.Kind == ReviewTaskKind.NewContent)
        {
            try
            {
                await _targetService.DeleteAsync(task.TargetType, task.TargetId, request.ReviewerId, actorIsModerator: true, cancellationToken);
            }
            catch (KeyNotFoundException)
            {
                // Target already gone — the end state (no entity) is what we want; just close the task.
            }
        }

        task.Status = ReviewTaskStatus.Rejected;
        task.ReviewerId = request.ReviewerId;
        task.ReviewerNote = request.Reason.ToNullIfEmpty()?.Trim();
        task.ResolvedAt = DateTime.UtcNow;

        await _context.SaveChangesAsync(cancellationToken);
        await transaction.CommitAsync(cancellationToken);

        return Unit.Value;
    }
}
