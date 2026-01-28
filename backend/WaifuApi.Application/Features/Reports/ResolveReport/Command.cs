using System.Threading;
using System.Threading.Tasks;
using Mediator;
using WaifuApi.Application.Interfaces;

namespace WaifuApi.Application.Features.Reports.ResolveReport;

public record ResolveReportCommand(long ReportId) : ICommand;

public class ResolveReportCommandHandler : ICommandHandler<ResolveReportCommand>
{
    private readonly IWaifuDbContext _context;

    public ResolveReportCommandHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<Unit> Handle(ResolveReportCommand request, CancellationToken cancellationToken)
    {
        var report = await _context.Reports.FindAsync(new object[] { request.ReportId }, cancellationToken);
        if (report != null)
        {
            report.IsResolved = true;
            await _context.SaveChangesAsync(cancellationToken);
        }
        return Unit.Value;
    }
}
