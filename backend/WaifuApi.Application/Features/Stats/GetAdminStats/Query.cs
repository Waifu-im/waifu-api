using Mediator;
using Microsoft.EntityFrameworkCore;
using WaifuApi.Application.Features.Stats.GetPublicStats;
using WaifuApi.Application.Interfaces;
using WaifuApi.Domain.Enums;

namespace WaifuApi.Application.Features.Stats.GetAdminStats;

public record GetAdminStatsQuery : IRequest<AdminStatsDto>;

public class GetAdminStatsQueryHandler : IRequestHandler<GetAdminStatsQuery, AdminStatsDto>
{
    private readonly IWaifuDbContext _context;
    private readonly IMediator _mediator;

    public GetAdminStatsQueryHandler(IWaifuDbContext context, IMediator mediator)
    {
        _context = context;
        _mediator = mediator;
    }

    public async ValueTask<AdminStatsDto> Handle(GetAdminStatsQuery request, CancellationToken cancellationToken)
    {
        var today = DateOnly.FromDateTime(DateTime.UtcNow);
        var oneYearAgo = today.AddYears(-1);
        
        var dailyStat = await _context.DailyStats
            .FirstOrDefaultAsync(s => s.Date == today, cancellationToken);

        var history = await _context.DailyStats
            .Where(s => s.Date >= oneYearAgo)
            .OrderBy(s => s.Date)
            .Select(s => new DailyStatDto 
            { 
                Date = s.Date, 
                Count = s.RequestCount 
            })
            .ToListAsync(cancellationToken);

        var topUsers = await _context.Users
            .OrderByDescending(u => u.RequestCount)
            .Take(10)
            .Select(u => new UserStatDto
            {
                Id = u.Id,
                Name = u.Name,
                RequestCount = u.RequestCount,
                ApiKeyRequestCount = u.ApiKeyRequestCount,
                JwtRequestCount = u.JwtRequestCount
            })
            .ToListAsync(cancellationToken);

        var topApiKeyUsers = await _context.Users
            .OrderByDescending(u => u.ApiKeyRequestCount)
            .Take(5)
            .Select(u => new UserStatDto
            {
                Id = u.Id,
                Name = u.Name,
                RequestCount = u.RequestCount,
                ApiKeyRequestCount = u.ApiKeyRequestCount,
                JwtRequestCount = u.JwtRequestCount
            })
            .ToListAsync(cancellationToken);

        var topJwtUsers = await _context.Users
            .OrderByDescending(u => u.JwtRequestCount)
            .Take(5)
            .Select(u => new UserStatDto
            {
                Id = u.Id,
                Name = u.Name,
                RequestCount = u.RequestCount,
                ApiKeyRequestCount = u.ApiKeyRequestCount,
                JwtRequestCount = u.JwtRequestCount
            })
            .ToListAsync(cancellationToken);

        var topUploaders = await _context.Users
            .Select(u => new UploaderStatDto
            {
                Id = u.Id,
                Name = u.Name,
                UploadedImageCount = _context.Images.Count(i => i.UploaderId == u.Id && i.ReviewStatus == ReviewStatus.Accepted)
            })
            .Where(u => u.UploadedImageCount > 0)
            .OrderByDescending(u => u.UploadedImageCount)
            .Take(10)
            .ToListAsync(cancellationToken);

        var topAlbumUsers = await _context.Users
            .Select(u => new AlbumUserStatDto
            {
                Id = u.Id,
                Name = u.Name,
                AlbumImageCount = _context.AlbumItems.Count(ai => _context.Albums.Any(a => a.Id == ai.AlbumId && a.UserId == u.Id))
            })
            .Where(u => u.AlbumImageCount > 0)
            .OrderByDescending(u => u.AlbumImageCount)
            .Take(10)
            .ToListAsync(cancellationToken);

        // Calculate authenticated vs unauthenticated
        var totalAuthenticatedRequests = await _context.Users.SumAsync(u => u.RequestCount, cancellationToken);
        var totalApiKeyRequests = await _context.Users.SumAsync(u => u.ApiKeyRequestCount, cancellationToken);
        var totalJwtRequests = await _context.Users.SumAsync(u => u.JwtRequestCount, cancellationToken);
        
        var publicStats = await _mediator.Send(new GetPublicStatsQuery(), cancellationToken);
        var totalUnauthenticatedRequests = publicStats.TotalRequests - totalAuthenticatedRequests;

        return new AdminStatsDto
        {
            RequestsToday = dailyStat?.RequestCount ?? 0,
            History = history,
            TopUsers = topUsers,
            TopApiKeyUsers = topApiKeyUsers,
            TopJwtUsers = topJwtUsers,
            TopUploaders = topUploaders,
            TopAlbumUsers = topAlbumUsers,
            TotalRequests = publicStats.TotalRequests,
            TotalAuthenticatedRequests = totalAuthenticatedRequests,
            TotalUnauthenticatedRequests = totalUnauthenticatedRequests,
            TotalApiKeyRequests = totalApiKeyRequests,
            TotalJwtRequests = totalJwtRequests,
            TotalImages = publicStats.TotalImages,
            TotalTags = publicStats.TotalTags,
            TotalArtists = publicStats.TotalArtists
        };
    }
}

public class AdminStatsDto
{
    public int RequestsToday { get; set; }
    public List<DailyStatDto> History { get; set; } = new();
    public List<UserStatDto> TopUsers { get; set; } = new();
    public List<UserStatDto> TopApiKeyUsers { get; set; } = new();
    public List<UserStatDto> TopJwtUsers { get; set; } = new();
    public List<UploaderStatDto> TopUploaders { get; set; } = new();
    public List<AlbumUserStatDto> TopAlbumUsers { get; set; } = new();
    
    // Public stats included
    public long TotalRequests { get; set; }
    public long TotalAuthenticatedRequests { get; set; }
    public long TotalUnauthenticatedRequests { get; set; }
    public long TotalApiKeyRequests { get; set; }
    public long TotalJwtRequests { get; set; }
    public int TotalImages { get; set; }
    public int TotalTags { get; set; }
    public int TotalArtists { get; set; }
}

public class DailyStatDto
{
    public DateOnly Date { get; set; }
    public int Count { get; set; }
}

public class UserStatDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public long RequestCount { get; set; }
    public long ApiKeyRequestCount { get; set; }
    public long JwtRequestCount { get; set; }
}

public class UploaderStatDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public long UploadedImageCount { get; set; }
}

public class AlbumUserStatDto
{
    public long Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public long AlbumImageCount { get; set; }
}