using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Constants;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Reports.GetReports;

public class GetReportsQuery : IQuery<PaginatedList<ReportDto>>
{
    public ReportStatus? Status { get; set; }

    /// <summary>When set, only reports submitted by this user are returned (scopes non-moderators to their own).</summary>
    public long? UserId { get; set; }

    /// <summary>Controls whether the reported image's uploaderId is exposed (moderators only).</summary>
    public Role? UserRole { get; set; }

    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetReportsQuery(ReportStatus? status, int page, int pageSize, long? userId = null, Role? userRole = null)
    {
        Status = status;
        Page = page;
        PageSize = pageSize;
        UserId = userId;
        UserRole = userRole;
    }

    public GetReportsQuery() { }
}

public class GetReportsQueryHandler : IQueryHandler<GetReportsQuery, PaginatedList<ReportDto>>
{
    private readonly IWaifuDbContext _context;
    private readonly string _cdnBaseUrl;
    private readonly int _defaultPageSize;
    private readonly int _maxPageSize;

    public GetReportsQueryHandler(IWaifuDbContext context, IConfiguration configuration)
    {
        _context = context;
        _cdnBaseUrl = configuration["Cdn:BaseUrl"] ?? throw new InvalidOperationException("Cdn:BaseUrl is required.");
        _defaultPageSize = int.Parse(configuration[ConfigurationKeys.Report.DefaultPageSize] ?? throw new InvalidOperationException($"{ConfigurationKeys.Report.DefaultPageSize} is required."));
        _maxPageSize = int.Parse(configuration[ConfigurationKeys.Report.MaxPageSize] ?? throw new InvalidOperationException($"{ConfigurationKeys.Report.MaxPageSize} is required."));
    }

    public async ValueTask<PaginatedList<ReportDto>> Handle(GetReportsQuery request, CancellationToken cancellationToken)
    {
        var (page, pageSize) = PaginationUtils.Normalize(request.Page, request.PageSize, _defaultPageSize, _maxPageSize);

        var query = _context.Reports
            .AsNoTracking()
            .Include(r => r.User)
            .Include(r => r.Image!)
            .ThenInclude(i => i.Artists)
            .Include(r => r.Image!)
            .ThenInclude(i => i.Tags)
            .AsQueryable();

        if (request.Status.HasValue)
        {
            query = query.Where(r => r.Status == request.Status.Value);
        }

        // Non-moderators are scoped to their own reports (set by the controller); moderators see all.
        if (request.UserId.HasValue)
        {
            query = query.Where(r => r.UserId == request.UserId.Value);
        }

        query = query.OrderByDescending(r => r.CreatedAt);

        var paginatedReports = await PaginatedList<Report>.CreateAsync(query, page, pageSize, _maxPageSize, _defaultPageSize, cancellationToken);

        var isModeratorOrAdmin = RoleUtils.IsModeratorOrAdmin(request.UserRole);
        var reportDtos = paginatedReports.Items
            .Select(report => ReportMapping.ToDto(report, _cdnBaseUrl, isModeratorOrAdmin))
            .ToList();

        return new PaginatedList<ReportDto>(reportDtos, paginatedReports.TotalCount, paginatedReports.PageNumber, pageSize, _maxPageSize, _defaultPageSize);
    }
}