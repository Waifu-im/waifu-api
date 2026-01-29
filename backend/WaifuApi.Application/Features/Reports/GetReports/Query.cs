using Mediator;
using Microsoft.EntityFrameworkCore;
using Microsoft.Extensions.Configuration;
using WaifuApi.Application.Common.Models;
using WaifuApi.Application.Common.Utilities;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Entities;

namespace WaifuApi.Application.Features.Reports.GetReports;

public class GetReportsQuery : IQuery<PaginatedList<ReportDto>>
{
    public bool? IsResolved { get; set; }
    public int Page { get; set; } = 1;
    public int PageSize { get; set; }

    public GetReportsQuery(bool? isResolved, int page, int pageSize)
    {
        IsResolved = isResolved;
        Page = page;
        PageSize = pageSize;
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
        _defaultPageSize = int.Parse(configuration["Review:DefaultPageSize"] ?? "30");
        _maxPageSize = int.Parse(configuration["Review:MaxPageSize"] ?? "-1");
    }

    public async ValueTask<PaginatedList<ReportDto>> Handle(GetReportsQuery request, CancellationToken cancellationToken)
    {
        var pageSize = request.PageSize == 0 ? _defaultPageSize : request.PageSize;
        if (_maxPageSize > 0 && pageSize > _maxPageSize) pageSize = _maxPageSize;

        var query = _context.Reports
            .AsNoTracking()
            .Include(r => r.User)
            .Include(r => r.Image)
            .ThenInclude(i => i.Artists)
            .Include(r => r.Image)
            .ThenInclude(i => i.Tags)
            .AsQueryable();

        if (request.IsResolved.HasValue)
        {
            query = query.Where(r => r.IsResolved == request.IsResolved.Value);
        }
        
        query = query.OrderByDescending(r => r.CreatedAt);

        var paginatedReports = await PaginatedList<Report>.CreateAsync(query, request.Page, pageSize, cancellationToken);

        var reportDtos = paginatedReports.Items.Select(report => new ReportDto
        {
            Id = report.Id,
            UserId = report.UserId,
            User = report.User != null ? new UserDto { Id = report.User.Id, Name = report.User.Name } : null,
            ImageId = report.ImageId,
            Image = report.Image != null ? new ImageDto
            {
                Id = report.Image.Id,
                PerceptualHash = BitArrayHelper.ToHex(report.Image.PerceptualHash), // Utilisation du Helper
                Extension = report.Image.Extension,
                DominantColor = report.Image.DominantColor,
                Source = report.Image.Source,
                Artists = report.Image.Artists.Select(a => new ArtistDto
                {
                    Id = a.Id,
                    Name = a.Name,
                    Patreon = a.Patreon,
                    Pixiv = a.Pixiv,
                    Twitter = a.Twitter,
                    DeviantArt = a.DeviantArt,
                    ReviewStatus = a.ReviewStatus
                }).ToList(),
                UploaderId = report.Image.UploaderId,
                UploadedAt = report.Image.UploadedAt,
                IsNsfw = report.Image.IsNsfw,
                IsAnimated = report.Image.IsAnimated,
                Width = report.Image.Width,
                Height = report.Image.Height,
                ByteSize = report.Image.ByteSize,
                Url = CdnUrlHelper.GetImageUrl(_cdnBaseUrl, report.Image.Id, report.Image.Extension),
                Tags = report.Image.Tags.Select(t => new TagDto
                {
                    Id = t.Id,
                    Name = t.Name,
                    Slug = t.Slug,
                    Description = t.Description,
                    ReviewStatus = t.ReviewStatus
                }).ToList()
            } : null,
            Description = report.Description,
            IsResolved = report.IsResolved,
            CreatedAt = report.CreatedAt
        }).ToList();

        return new PaginatedList<ReportDto>(reportDtos, paginatedReports.TotalCount, paginatedReports.PageNumber, pageSize);
    }
}