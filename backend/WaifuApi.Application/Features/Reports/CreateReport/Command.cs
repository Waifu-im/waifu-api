using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Reports.CreateReport;

public record CreateReportCommand(long UserId, long ImageId, string? Description) : ICommand<Report>;

public class CreateReportCommandHandler : ICommandHandler<CreateReportCommand, Report>
{
    private readonly IWaifuDbContext _context;

    public CreateReportCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Report> Handle(CreateReportCommand request, CancellationToken cancellationToken)
    {
        // One open report per (user, image): a user can't pile multiple pending reports on the same image.
        var hasOpenReport = await _context.Reports.AnyAsync(r =>
            r.UserId == request.UserId &&
            r.ImageId == request.ImageId &&
            r.Status == ReportStatus.Pending, cancellationToken);

        if (hasOpenReport)
        {
            throw new ConflictException("You already have a pending report for this image.");
        }

        var report = new Report
        {
            UserId = request.UserId,
            ImageId = request.ImageId,
            Description = request.Description!, // Validated by FluentValidation
            CreatedAt = DateTime.UtcNow,
            Status = ReportStatus.Pending
        };

        _context.Reports.Add(report);
        await _context.SaveChangesAsync(cancellationToken);

        return report;
    }
}
