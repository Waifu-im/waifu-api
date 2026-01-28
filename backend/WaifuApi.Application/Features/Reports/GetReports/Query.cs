using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Reports.GetReports;

public record GetReportsQuery(bool? IsResolved = null) : IQuery<List<Report>>;

public class GetReportsQueryHandler : IQueryHandler<GetReportsQuery, List<Report>>
{
    private readonly IWaifuDbContext _context;

    public GetReportsQueryHandler(IWaifuDbContext context)
    {
        _context = context;
    }

    public async ValueTask<List<Report>> Handle(GetReportsQuery request, CancellationToken cancellationToken)
    {
        var query = _context.Reports.AsQueryable();

        if (request.IsResolved.HasValue)
        {
            query = query.Where(r => r.IsResolved == request.IsResolved.Value);
        }

        return await query
            .OrderByDescending(r => r.CreatedAt)
            .ToListAsync(cancellationToken);
    }
}
