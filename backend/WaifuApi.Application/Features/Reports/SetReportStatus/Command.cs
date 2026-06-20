using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Reports.SetReportStatus;

/// <summary>
/// Transitions a pending report to a terminal status. Resolved/Rejected are moderator actions; Cancelled is the
/// report author withdrawing their own report (or a moderator). Only pending reports can be transitioned.
/// </summary>
public record SetReportStatusCommand(long Id, long ActingUserId, bool IsModerator, ReportStatus Target, string? Reason = null) : ICommand;

public class SetReportStatusCommandHandler : ICommandHandler<SetReportStatusCommand>
{
    private readonly IWaifuDbContext _context;

    public SetReportStatusCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(SetReportStatusCommand request, CancellationToken cancellationToken)
    {
        var report = await _context.Reports.FindAsync(new object[] { request.Id }, cancellationToken)
            ?? throw new KeyNotFoundException($"Report with ID {request.Id} not found.");

        if (report.Status != ReportStatus.Pending)
        {
            throw new ConflictException("This report has already been handled.");
        }

        if (request.Target == ReportStatus.Cancelled)
        {
            // Authors may withdraw their own report; moderators may cancel any.
            if (!request.IsModerator && report.UserId != request.ActingUserId)
            {
                throw new ForbiddenException("You can only cancel your own reports.");
            }
        }
        else if (!request.IsModerator)
        {
            // Resolved / Rejected are moderator-only outcomes.
            throw new ForbiddenException("Only moderators can resolve or reject reports.");
        }

        report.Status = request.Target;
        // Record the moderator's reason on an answered report (validate/reject); cancellation has no note.
        if (request.Target != ReportStatus.Cancelled)
        {
            report.ReviewerNote = string.IsNullOrWhiteSpace(request.Reason) ? null : request.Reason.Trim();
        }
        await _context.SaveChangesAsync(cancellationToken);
        return Unit.Value;
    }
}
