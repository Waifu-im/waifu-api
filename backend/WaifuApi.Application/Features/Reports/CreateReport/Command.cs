using System;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

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
        var report = new Report
        {
            UserId = request.UserId,
            ImageId = request.ImageId,
            Description = request.Description,
            CreatedAt = DateTime.UtcNow,
            IsResolved = false
        };

        _context.Reports.Add(report);
        await _context.SaveChangesAsync(cancellationToken);

        return report;
    }
}
