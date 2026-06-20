using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Exceptions;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Reports.UpdateReport;

/// <summary>Edits a report's description. Allowed for the report's author (or a moderator) while it is still open.</summary>
public record UpdateReportCommand(long Id, long UserId, bool IsModerator, string Description) : ICommand<ReportDto>;

public class UpdateReportCommandHandler : ICommandHandler<UpdateReportCommand, ReportDto>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;

    public UpdateReportCommandHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
    }

    public async ValueTask<ReportDto> Handle(UpdateReportCommand request, CancellationToken cancellationToken)
    {
        var report = await _context.Reports
            .Include(r => r.User)
            .Include(r => r.Image!).ThenInclude(i => i.Artists)
            .Include(r => r.Image!).ThenInclude(i => i.Tags)
            .FirstOrDefaultAsync(r => r.Id == request.Id, cancellationToken)
            ?? throw new KeyNotFoundException($"Report with ID {request.Id} not found.");

        if (!request.IsModerator && report.UserId != request.UserId)
        {
            throw new ForbiddenException("You can only edit your own reports.");
        }

        if (report.Status != ReportStatus.Pending)
        {
            throw new ConflictException("This report has already been handled and can no longer be edited.");
        }

        report.Description = request.Description.Trim();
        await _context.SaveChangesAsync(cancellationToken);

        return ReportMapping.ToDto(report, _cdnBaseUrl, request.IsModerator);
    }
}
