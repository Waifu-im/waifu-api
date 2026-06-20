using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Review.ApproveReviewTask;

public record ApproveReviewTaskCommand(long TaskId, long ReviewerId) : ICommand;

public class ApproveReviewTaskCommandHandler : ICommandHandler<ApproveReviewTaskCommand>
{
    private readonly IWaifuDbContext _context;
    private readonly IEditApplier _editApplier;
    private readonly IReviewTargetService _targetService;

    public ApproveReviewTaskCommandHandler(
        IWaifuDbContext context,
        IEditApplier editApplier,
        IReviewTargetService targetService)
    {
        _context = context;
        _editApplier = editApplier;
        _targetService = targetService;
    }

    public async ValueTask<Unit> Handle(ApproveReviewTaskCommand request, CancellationToken cancellationToken)
    {
        var task = await _context.ReviewTasks.FindAsync(new object[] { request.TaskId }, cancellationToken)
            ?? throw new KeyNotFoundException($"Review task with ID {request.TaskId} not found.");

        // Guard the transition so two moderators approving at once can't double-apply.
        if (task.Status != ReviewTaskStatus.Pending)
        {
            throw new ConflictException("This review task has already been resolved.");
        }

        // Apply and resolve atomically: if applying fails (target gone, validation), everything rolls back
        // and the task stays Pending so a moderator can retry or reject it.
        await using var transaction = await _context.BeginTransactionAsync(cancellationToken);

        if (task.Kind == ReviewTaskKind.Edit)
        {
            // Approving simply applies the proposed change over the current values (no drift/conflict check).
            await _editApplier.ApplyAsync(task, cancellationToken);
        }
        else // NewContent: publish the submitted entity
        {
            await _targetService.AcceptAsync(task.TargetType, task.TargetId, cancellationToken);
        }

        task.Status = ReviewTaskStatus.Approved;
        task.ReviewerId = request.ReviewerId;
        task.ResolvedAt = DateTime.UtcNow;

        await _context.SaveChangesAsync(cancellationToken);
        await transaction.CommitAsync(cancellationToken);

        return Unit.Value;
    }
}
